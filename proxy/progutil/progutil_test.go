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

package progutil_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/http2"

	"github.com/otcshare/common/proxy/progutil"
)

func TestPrefaceListenerAddr(t *testing.T) {
	// Start TCP listener
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	pl := &progutil.PrefaceListener{Listener: lis}
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
	pl := &progutil.PrefaceListener{Listener: lis}

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
		b := make([]byte, len(http2.ClientPreface))
		if _, err := conn.Read(b); err != nil {
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
	pl := &progutil.PrefaceListener{Listener: lis}

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

		if err := http2.NewFramer(conn, nil).WriteSettings(); err != nil {
			dialErrC <- fmt.Errorf("error writing server preface: %v", err)
			return
		}
	}()

	// "Dial" to get an incoming conn
	readErrC := make(chan error, 1)
	go func() {
		defer close(readErrC)

		conn, err := pl.Dial("", time.Second)
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

func TestDialListenerTLS(t *testing.T) {
	// Generate self-signed CA
	root, key, err := genRootCA()
	if err != nil {
		t.Fatal(err)
	}
	conf := &tls.Config{Certificates: []tls.Certificate{
		{Certificate: [][]byte{root.Raw}, PrivateKey: key},
	}}

	// Listen on an ephemeral port
	lis, err := tls.Listen("tcp", "localhost:0", conf)
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()

	// Connect to test server
	caPool := x509.NewCertPool()
	caPool.AddCert(root)
	clientConf := &tls.Config{ServerName: root.Subject.CommonName, RootCAs: caPool}
	dlis := &progutil.DialListener{RemoteAddr: lis.Addr(), TLS: clientConf}
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
		TLSClientConfig: clientConf,
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return conn, nil
		},
	}}
	req, err := http.NewRequest(http.MethodGet, "http://anything/", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := cli.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	if http.StatusNotFound != resp.StatusCode {
		t.Fatalf("expected HTTP status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func genRootCA() (*x509.Certificate, crypto.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	todayY, todayM, todayD := time.Now().Date()
	today := time.Time{}.AddDate(todayY, int(todayM), todayD).AddDate(-1, -1, -1)
	tomorrow := today.AddDate(0, 0, 1)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(0),
		Subject:               pkix.Name{CommonName: "Root"},
		IsCA:                  true,
		BasicConstraintsValid: true,
		NotBefore:             today,
		NotAfter:              tomorrow,
	}
	root, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(root)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

func TestDialListenerUnix(t *testing.T) {
	t.Skip("TODO")
}
