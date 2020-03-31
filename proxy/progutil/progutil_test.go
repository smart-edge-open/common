// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2019 Intel Corporation

package progutil_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/http2"

	"github.com/open-ness/common/proxy/progutil"
)

func TestPrefaceListenerAddr(t *testing.T) {
	// Start TCP listener
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	pl := progutil.NewPrefaceListener(lis)
	if pl.Addr().String() != lis.Addr().String() {
		t.Fatal("address should match inner listener")
	}
}

func TestPrefaceListenerAccept(t *testing.T) {
	// Start TCP listener
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()
	pl := progutil.NewPrefaceListener(lis)

	// Start dialing
	dialErrC := make(chan error, 1)
	go func() {
		defer close(dialErrC)

		addr := lis.Addr()
		conn, err := (&net.Dialer{}).Dial(addr.Network(), addr.String())
		if err != nil {
			dialErrC <- err
			return
		}
		defer conn.Close()

		_, err = conn.Write([]byte(http2.ClientPreface))
		if err != nil {
			dialErrC <- fmt.Errorf("error writing client preface: %v", err)
			return
		}
	}()

	// Accept an incoming conn
	readErrC := make(chan error, 1)
	go func() {
		defer close(readErrC)

		conn, err := pl.Accept()
		if err != nil {
			readErrC <- err
			return
		}
		b, err := ioutil.ReadAll(conn)
		if err != nil {
			readErrC <- fmt.Errorf("error reading client preface: %v", err)
			return
		}
		if !bytes.Equal(b, []byte(http2.ClientPreface)) {
			readErrC <- fmt.Errorf("expected %q, got %q",
				http2.ClientPreface, string(b))
			return
		}
	}()

	// Check errors
	if err := <-dialErrC; err != nil {
		t.Fatalf("error dialing and writing to conn: %v", err)
	}
	if err := <-readErrC; err != nil {
		t.Fatalf("error accepting and reading from conn: %v", err)
	}
}

func TestPrefaceListenerDial(t *testing.T) {
	// Start TCP listener
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()
	pl := progutil.NewPrefaceListener(lis)

	pl.RegisterHost("127.0.0.1")

	// Start dialing
	dialErrC := make(chan error, 1)
	go func() {
		defer close(dialErrC)

		addr := lis.Addr()
		conn, err := (&net.Dialer{}).Dial(addr.Network(), addr.String())
		if err != nil {
			dialErrC <- err
			return
		}
		defer conn.Close()

		if _, err := conn.Write([]byte("EVA")); err != nil {
			dialErrC <- fmt.Errorf("error writing EVA preface: %v", err)
		}
		if err := http2.NewFramer(conn, nil).WriteSettings(); err != nil {
			dialErrC <- fmt.Errorf("error writing server preface: %v", err)
			return
		}
	}()

	// "Dial" to get an incoming conn
	readErrC := make(chan error, 1)
	go func() {
		defer close(readErrC)

		conn, err := pl.DialEva("127.0.0.1", time.Second)
		if err != nil {
			readErrC <- err
			return
		}
		hdr, err := http2.ReadFrameHeader(conn)
		if err != nil {
			readErrC <- err
			return
		}
		if hdr.Type != http2.FrameSettings {
			readErrC <- fmt.Errorf("expected to receive SETTINGS frame, got %s", hdr.Type)
			return
		}
	}()

	// Accept should block indefinitely
	go func() {
		conn, err := pl.Accept()
		if err != nil {
			return
		}
		t.Log("WARNING: PrefaceListener unexpectedly accepted a conn")
		conn.Close()

	}()

	// Check errors
	if err := <-dialErrC; err != nil {
		t.Fatalf("error dialing and writing to conn: %v", err)
	}
	if err := <-readErrC; err != nil {
		t.Fatalf("error dialing via PrefaceListener and reading from conn: %v", err)
	}
}

func TestDialListenerTCP(t *testing.T) {
	// Listen on an ephemeral port
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	// Connect to test server
	dlis := &progutil.DialListener{RemoteAddr: lis.Addr()}
	defer dlis.Close()

	// Serve HTTP
	go func() { _ = http.Serve(dlis, http.NotFoundHandler()) }()

	// Accept a conn to use for an HTTP client
	conn, err := lis.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Make an HTTP request
	cli := http.Client{Transport: &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return conn, nil
		},
	}}
	resp, err := cli.Get("http://anything/")
	if err != nil {
		t.Fatal(err)
	}
	if http.StatusNotFound != resp.StatusCode {
		t.Fatalf("expected HTTP status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}
