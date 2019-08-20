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

//go:generate protoc -I ./pb --go_out=plugins=grpc:pb ./pb/test.proto

package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sync"
	"syscall"
)

func main() {
	var (
		wg          sync.WaitGroup
		exiting     = make(chan struct{})
		errC        = make(chan error, 2)
		ctrlSuccess = new(rpcSuccessChecker)
		applSuccess = new(rpcSuccessChecker)
	)
	wg.Add(2)

	// Capture interrupt to kill subprocesses
	intC := make(chan os.Signal, 1)
	signal.Notify(intC, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-intC
		close(exiting)
	}()

	// Channel to receive cloud listener's ephemeral port after it starts
	var portC = make(chan string, 1)

	// Run cloud component listening on an ephemeral port
	log.Printf("Starting Controller")
	cc := exec.Command("go", "run", "./controller.go")
	cc.Stdout = addrParser{w: os.Stdout, c: portC}
	cc.Stderr = io.MultiWriter(os.Stderr, ctrlSuccess)
	cc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cc.Start(); err != nil {
		log.Fatalf("error starting cloud controller: %v", err)
	}
	go func() {
		<-exiting
		// Kill the process group
		syscall.Kill(-cc.Process.Pid, syscall.SIGINT)
	}()
	go func() {
		defer wg.Done()

		// Wait for component to shutdown
		if err := cc.Wait(); err != nil {
			select {
			case <-exiting:
			default:
				errC <- fmt.Errorf("cloud controller errored: %v", err)

				// Close intC to trigger shutdown of other components
				signal.Stop(intC)
				close(intC)
			}
		}
	}()

	// Run edge component with cloud component's port as flag
	port := <-portC
	log.Printf("Starting Appliance")
	ap := exec.Command("go", "run", "./appliance.go", "-port", port)
	ap.Stdout = os.Stdout
	ap.Stderr = io.MultiWriter(os.Stderr, applSuccess)
	ap.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := ap.Start(); err != nil {
		log.Fatalf("error starting edge appliance: %v", err)
	}
	go func() {
		<-exiting
		// Kill the process group
		syscall.Kill(-ap.Process.Pid, syscall.SIGINT)
	}()
	go func() {
		defer wg.Done()

		// Wait for component to shutdown
		if err := ap.Wait(); err != nil {
			select {
			case <-exiting:
			default:
				errC <- fmt.Errorf("edge appliance errored: %v", err)

				// Close intC to trigger shutdown of other components
				signal.Stop(intC)
				close(intC)
			}
		}
	}()

	// Initiate shutdown and wait for processes to finish
	wg.Wait()

	// Enumerate errors from process shudowns
	close(errC)
	exitCode := 0
	for err := range errC {
		exitCode = 1
		log.Println(err)
	}

	// Check that process stderrs end with successful RPC
	if !ctrlSuccess.Success() {
		exitCode = 1
		log.Printf("controller did not succeed on final RPC to appliance")
	}
	if !applSuccess.Success() {
		exitCode = 1
		log.Printf("appliance did not succeed on final RPC to controller")
	}

	os.Exit(exitCode)
}

// helper type to capture the server address from its logs
type addrParser struct {
	w io.Writer
	c chan<- string

	buf   []byte
	found bool
}

var reListener = regexp.MustCompile(`Listening on \S+ (\S+)\n`)

func (w addrParser) Write(p []byte) (n int, err error) {
	if !w.found {
		w.buf = append(w.buf, p...)
		if reListener.Match(w.buf) {
			w.found = true
			w.c <- string(reListener.FindSubmatch(w.buf)[1])
		} else {
			w.buf = w.buf[bytes.LastIndexByte(w.buf, '\n')+1:]
		}
	}
	return w.w.Write(p)
}

// helper type to check that RPC successes were logged
type rpcSuccessChecker struct {
	lastLineWasSuccess bool

	buf []byte
}

func (r *rpcSuccessChecker) Success() bool { return r.lastLineWasSuccess }

var reSuccessRPC = regexp.MustCompile(`got response from`)

func (r *rpcSuccessChecker) Write(p []byte) (int, error) {
	r.buf = append(r.buf, bytes.TrimSpace(p)...)
	r.buf = r.buf[bytes.LastIndexByte(r.buf, '\n')+1:]
	r.lastLineWasSuccess = reSuccessRPC.Match(r.buf)
	return len(p), nil
}
