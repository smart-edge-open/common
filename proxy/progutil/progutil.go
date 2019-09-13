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
	"errors"
	"fmt"
	logger "github.com/otcshare/common/log"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var errNoDeadlineSupport = errors.New("listener does not support accept deadline setting")
var log = logger.DefaultLogger.WithField("proxy", nil)

// PrefaceListener accepts connections and separates them based on whether they
// begin with a client HTTP/2 preface or our custom ID. It is assumed that
//
// 1. No connections are made without a preface being sent within a reasonable
// amount of time.
// 2. Accept will be called in a loop for as long as Dial will be used.
type PrefaceListener struct {
	// Listener is the underlying connection acceptor. It cannot be nil.
	net.Listener

	chEva chan net.Conn
	chEla chan net.Conn
}

func (l *PrefaceListener) Addr() net.Addr { return l.Listener.Addr() }

// Sends the connection into the channel
func storeConn(conn net.Conn, ch chan net.Conn) {
	select {
	// empty channel of previous conn, if any
	case discard := <-ch:
		discard.Close()
	default:
	}

	select {
	// store conn unless an equally recent conn already got stored
	case ch <- conn:
	default:
		conn.Close()
	}
}

func NewPrefaceListener(l net.Listener) *PrefaceListener {
	pl := new(PrefaceListener)

	pl.Listener = l

	// Init our channels
	pl.chEva = make(chan net.Conn, 1)
	pl.chEla = make(chan net.Conn, 1)

	return pl
}

func (l *PrefaceListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	// read preface
	packet := make([]byte, 3)
	log.Debugf("Connection from %v, awaiting 1st packet", conn.RemoteAddr())
	n, err := conn.Read(packet)
	if err != nil {
		log.Debugf("Failed to read 1st packet: %s", err)
	}
	log.Debugf("First packet received, %d/%d bytes: %q", n, len(packet), string(packet[:n]))
	packet = packet[:n]

	// If the conn is from a server, store it for later use
	if bytes.Equal(packet, []byte("EVA")) {
		log.Debugf("we have EVA proxy callback")
		storeConn(conn, l.chEva)
	} else if bytes.Equal(packet, []byte("ELA")) {
		log.Debugf("we have ELA proxy callback")
		storeConn(conn, l.chEla)
	} else {
		// If the conn is from a client, return immediately
		log.Debugf("we have a client connection")
		// reconstruct data
		conn = readerConn{conn, io.MultiReader(io.MultiReader(
			bytes.NewBuffer(packet), io.Reader(conn)), conn)}

		return conn, nil
	}

	return l.Accept() // nothing we can return, loop
}
func (l *PrefaceListener) Close() error {
	close(l.chEva)
	close(l.chEla)

	return l.Listener.Close()
}
func (l *PrefaceListener) SetDeadline(deadline time.Time) error {
	lis, ok := l.Listener.(interface{ SetDeadline(time.Time) error })
	if !ok {
		return errNoDeadlineSupport
	}
	return lis.SetDeadline(deadline)
}

func dialCommon(ch chan net.Conn, dur time.Duration) (net.Conn, error) {
	select {
	case conn := <-ch:
		if conn == nil {
			return nil, &net.OpError{Op: "dial", Err: fmt.Errorf("dialer closing")}
		}
		return conn, nil
	case <-time.After(dur):
		return nil, context.DeadlineExceeded
	}
}
func (l *PrefaceListener) DialEva(_ string, dur time.Duration) (net.Conn, error) {
	return dialCommon(l.chEva, dur)
}
func (l *PrefaceListener) DialEla(_ string, dur time.Duration) (net.Conn, error) {
	return dialCommon(l.chEla, dur)
}

type readerConn struct {
	net.Conn
	io.Reader
}

func (conn readerConn) Read(p []byte) (int, error) { return conn.Reader.Read(p) }

type DialListener struct {
	RemoteAddr net.Addr
	Name       string

	// Connection management variables
	established int32 // awaiting connections with no data yet
	active      int32 // connections that had some bytes flowing
}

// Accept waits for and returns the next connection to the listener.
func (lis *DialListener) Accept() (net.Conn, error) {
	if lis.established > lis.active {
		// We have at least 1 free connection, nothing to do
		time.Sleep(time.Second)
		return nil, tempErr{}
	}
	if lis.established > 0 {
		// Do not log when controller is completely down.
		// So we log when all (non-zero) connections we have are in use
		log.Debugf("%v DialListener: ConnPool: %v/%v",
			lis.Name, lis.active, lis.established)
		log.Debugf("%v DialListener: dialling %v", lis.Name, lis.RemoteAddr)
	}

	// Last connection in use (or no connections), make a new one
	conn, err := net.Dial(
		lis.RemoteAddr.Network(), lis.RemoteAddr.String())
	if err != nil {
		// Controller down, keep trying to dial every 1 second
		time.Sleep(time.Second)
		return nil, tempErr{}
	}

	// Send our ID
	conn.Write([]byte(lis.Name))
	atomic.AddInt32(&lis.established, 1)

	log.Debugf("%v DialListener connection established: ConnPool: %v/%v",
		lis.Name, lis.active, lis.established)

	return &notifyOnNetErr{Conn: conn, L: lis}, nil
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (lis *DialListener) Close() (err error) {

	return
}

// Addr returns the listener's network address.
func (lis *DialListener) Addr() net.Addr {
	return nil // TODO
}

type notifyOnNetErr struct {
	net.Conn
	L      *DialListener
	active bool

	onceErr, onceRead sync.Once
}

func (conn *notifyOnNetErr) Read(b []byte) (int, error) {
	n, err := conn.Conn.Read(b)

	if e, ok := err.(net.Error); (err != nil && !ok) || (ok && !e.Temporary()) {
		conn.onceErr.Do(func() {
			log.Debugf("Read error happened with one of the connections: %v", err)
			atomic.AddInt32(&conn.L.established, -1)
			if conn.active {
				atomic.AddInt32(&conn.L.active, -1)
			}
		})
		return n, err
	}

	if n == 0 {
		return n, err
	}
	conn.onceRead.Do(func() {
		conn.active = true
		atomic.AddInt32(&conn.L.active, 1)
	})

	return n, err
}

func (conn *notifyOnNetErr) Write(p []byte) (int, error) {
	n, err := conn.Conn.Write(p)
	if e, ok := err.(net.Error); (err != nil && !ok) || (ok && !e.Temporary()) {
		conn.onceErr.Do(func() {
			log.Debugf("Write error happened with one of the connections: %v", err)
			atomic.AddInt32(&conn.L.established, -1)
			if conn.active {
				atomic.AddInt32(&conn.L.active, -1)
			}
		})
	}
	return n, err
}

func (conn *notifyOnNetErr) Close() error {
	conn.onceErr.Do(func() {
		atomic.AddInt32(&conn.L.established, -1)
		if conn.active {
			atomic.AddInt32(&conn.L.active, -1)
		}
	})

	return conn.Conn.Close()
}

type tempErr struct {
	error
}

func (tempErr) Temporary() bool { return true }
