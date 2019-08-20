// Copyright 2019 Smart-Edge.com, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package progutil

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

var errNoDeadlineSupport = errors.New("listener does not support accept deadline setting")

// PrefaceListener accepts connections and separates them based on whether they
// begin with a server or client HTTP/2 preface. It is assumed that
//
// 1. No connections are made without a preface being sent within a reasonable
// amount of time.
// 2. Accept will be called in a loop for as long as Dial will be used.
type PrefaceListener struct {
	// Listener is the underlying connection acceptor. It cannot be nil.
	net.Listener

	// PrefaceReadTimeout is the amount of time to wait on receiving an HTTP/2
	// client or server preface before closing and dropping the connection. If
	// not set a default of 200ms will be used.
	PrefaceReadTimeout time.Duration

	ch chan net.Conn

	init, cleanup sync.Once
}

func (l *PrefaceListener) Addr() net.Addr { return l.Listener.Addr() }
func (l *PrefaceListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	// read preface
	var (
		r        = io.Reader(conn)
		nextByte = make([]byte, 1)
		isClient = true
		consumed []byte
	)
	for i := range http2.ClientPreface {
		_, err := conn.Read(nextByte)
		if err == nil {
			consumed = append(consumed, nextByte[0])
		}
		if err != nil || nextByte[0] != http2.ClientPreface[i] {
			// Must be a server pushing a SETTINGS frame
			isClient = false
			break
		}
	}

	// reconstruct conn
	conn = readerConn{conn, io.MultiReader(io.MultiReader(bytes.NewBuffer(consumed), r), conn)}

	// If the conn is from a client, return immediately
	if isClient {
		return conn, nil
	}

	// If the conn is from a server, store it for later use
	l.init.Do(func() { l.ch = make(chan net.Conn, 1) })
	select {
	// empty channel of previous conn, if any
	case discard := <-l.ch:
		discard.Close()
	default:
	}
	select {
	// store conn unless an equally recent conn already got stored
	case l.ch <- conn:
	default:
		conn.Close()
	}
	return l.Accept()
}
func (l *PrefaceListener) Close() error {
	l.init.Do(func() { l.ch = make(chan net.Conn, 1) })
	err := l.Listener.Close()
	l.cleanup.Do(func() { close(l.ch) })
	return err
}
func (l *PrefaceListener) SetDeadline(deadline time.Time) error {
	lis, ok := l.Listener.(interface{ SetDeadline(time.Time) error })
	if !ok {
		return errNoDeadlineSupport
	}
	return lis.SetDeadline(deadline)
}
func (l *PrefaceListener) Dial(_ string, dur time.Duration) (net.Conn, error) {
	l.init.Do(func() { l.ch = make(chan net.Conn, 1) })
	select {
	case conn := <-l.ch:
		if conn == nil {
			return nil, &net.OpError{Op: "dial", Err: fmt.Errorf("dialer closing")}
		}
		return conn, nil
	case <-time.After(dur):
		return nil, context.DeadlineExceeded
	}
}

type readerConn struct {
	net.Conn
	io.Reader
}

func (conn readerConn) Read(p []byte) (int, error) { return conn.Reader.Read(p) }

type DialListener struct {
	LocalAddr  net.Addr
	RemoteAddr net.Addr
	TLS        *tls.Config

	// TODO: combine a local listener
	// Inner net.Listener

	connMu sync.Mutex
	conn   net.Conn

	closeMu sync.Mutex
	closing chan struct{}
	redialC chan struct{}
}

// Accept waits for and returns the next connection to the listener.
func (lis *DialListener) Accept() (net.Conn, error) {
	// Initialize channels
	lis.closeMu.Lock()
	if lis.closing == nil {
		lis.closing = make(chan struct{})
		lis.redialC = make(chan struct{}, 1)
		lis.redialC <- struct{}{}
	}
	lis.closeMu.Unlock()

	select {
	case <-lis.closing:
		return nil, io.EOF
	default:
		// Nest so that if both channels are loaded close will always have
		// precedence
		select {
		case <-lis.closing:
			return nil, io.EOF
		case <-lis.redialC:
			/*
				log.Printf("Redialing from [%s] to [%s] (TLS: %v)",
					lis.LocalAddr, lis.RemoteAddr, lis.TLS != nil)
			*/
		}
	}

	// Dial to network
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-ctx.Done():
		case <-lis.closing:
			cancel()
		}
	}()
	conn, err := (&net.Dialer{LocalAddr: lis.LocalAddr}).DialContext(
		context.TODO(), // for future use with SetDeadline
		lis.RemoteAddr.Network(), lis.RemoteAddr.String())
	if err != nil {
		lis.redialC <- struct{}{}
		// Ensure error is always temporary
		return nil, tempErr{err}
	}
	if lis.TLS != nil {
		conn = tls.Client(conn, lis.TLS)
	}

	// Store conn in memory
	lis.connMu.Lock()
	defer lis.connMu.Unlock()
	lis.conn = conn
	return &notifyOnNetErr{Conn: conn, C: lis.redialC}, nil
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (lis *DialListener) Close() (err error) {
	lis.closeMu.Lock()
	defer lis.closeMu.Unlock()
	select {
	case <-lis.closing:
		return
	default:
		if lis.closing != nil {
			close(lis.closing)
		}
	}

	lis.connMu.Lock()
	defer lis.connMu.Unlock()
	if lis.conn != nil {
		lis.conn, err = nil, lis.conn.Close()
	}
	return
}

// Addr returns the listener's network address.
func (lis *DialListener) Addr() net.Addr {
	lis.connMu.Lock()
	defer lis.connMu.Unlock()

	if lis.conn == nil {
		return nil
	}
	return lis.conn.LocalAddr()
}

type notifyOnNetErr struct {
	net.Conn
	C chan<- struct{}

	once sync.Once
}

func (conn *notifyOnNetErr) Read(b []byte) (int, error) {
	n, err := conn.Conn.Read(b)
	if e, ok := err.(net.Error); (err != nil && !ok) || (ok && !e.Temporary()) {
		conn.once.Do(func() { conn.C <- struct{}{} })
	}
	return n, err
}

func (conn *notifyOnNetErr) Write(p []byte) (int, error) {
	n, err := conn.Conn.Write(p)
	if e, ok := err.(net.Error); (err != nil && !ok) || (ok && !e.Temporary()) {
		conn.once.Do(func() { conn.C <- struct{}{} })
	}
	return n, err
}

// SplitTLSListener creates a listener that can split out certain incoming
// connections from an inner listener to child listeners it creates.
type SplitTLSListener struct {
	net.Listener

	matchersMu sync.Mutex
	matchers   []func(tls.ConnectionState) chan<- net.Conn
}

// DropUnsplit throws out all connections that aren't split to a child
// listener. This func should be run in a goroutine.
func (l *SplitTLSListener) DropUnsplit() {
	conn, err := l.Accept()
	for {
		if err != nil {
			return
		}
		/*
			if tlsConn, ok := conn.(*tls.Conn); ok {
				log.Printf("Dropping conn: %+v", tlsConn.ConnectionState())
			}
		*/
		conn.Close()
		conn, err = l.Accept()
	}
}

// Accept waits for and returns the next connection to the listener. All conns
// that can go to a child listener are silently sent and nothing is returned
// from this Accept func.
//
// WARNING: This is the main accept loop. If it is not called continuously
// until an error condition, then child listeners may not Accept new
// connections. If all expected conns will be sent to a child listener, then
// call `go lis.DropUnsplit()`.
func (l *SplitTLSListener) Accept() (net.Conn, error) {
CatchAllListener:
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		tlsConn, isTLS := conn.(*tls.Conn)
		if !isTLS {
			return conn, nil
		}
		if err := tlsConn.Handshake(); err != nil {
			tlsConn.Close()
			continue CatchAllListener
		}
		state := tlsConn.ConnectionState()

		l.matchersMu.Lock()
		for _, matcher := range l.matchers {
			found := matcher(state)
			if found != nil {
				l.matchersMu.Unlock()
				select {
				case found <- conn:
				default:
				}
				continue CatchAllListener
			}
		}
		l.matchersMu.Unlock()

		return conn, nil
	}
}

// SplitSNI creates a child listener that only includes connections that dialed
// to a particular TLS SNI. This is only useful for configs
func (l *SplitTLSListener) SplitSNI(sni string) net.Listener {
	ch := make(chan net.Conn, 100)
	l.matchersMu.Lock()
	l.matchers = append(l.matchers, func(cs tls.ConnectionState) chan<- net.Conn {
		if cs.ServerName == sni {
			return ch
		}
		return nil
	})
	l.matchersMu.Unlock()
	return chanListener{l.Listener, ch}
}

type chanListener struct {
	net.Listener
	ch <-chan net.Conn
}

// Accept waits for and returns the next connection to the listener.
func (l chanListener) Accept() (net.Conn, error) {
	// TODO: handle listener being closed or timed out
	return <-l.ch, nil
}

type tempErr struct {
	error
}

func (tempErr) Temporary() bool { return true }
