// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2019 Intel Corporation

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
	"time"
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
	cc.Stderr = ctrlSuccess
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
	ap.Stderr = applSuccess
	ap.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := ap.Start(); err != nil {
		log.Fatalf("error starting edge appliance: %v", err)
	}
	go func() {
		<-exiting
		fmt.Println("Main process: Caught the exit signal")
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

	// Allow children to finish writes to stdout
	time.Sleep(time.Second)
	// Enumerate errors from process shudowns
	close(errC)
	exitCode := 0
	for err := range errC {
		exitCode = 1
		log.Printf("Error channel: %v\n", err)
	}

	// Check that process stderrs end with successful RPC
	if !ctrlSuccess.Success() {
		exitCode = (exitCode | 2)
		log.Printf("controller did not succeed on final RPC to appliance")
	}
	if !applSuccess.Success() {
		exitCode = (exitCode | 4)
		log.Printf("appliance did not succeed on final RPC to controller")
	}

	fmt.Printf("Exiting with code: %v\n", exitCode)
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
}

func (r *rpcSuccessChecker) Success() bool { return r.lastLineWasSuccess }

var reSuccessRPC = regexp.MustCompile(`got response from`)

func (r *rpcSuccessChecker) Write(p []byte) (int, error) {
	r.lastLineWasSuccess = reSuccessRPC.Match(p)
	if r.lastLineWasSuccess {
		fmt.Printf("<OK> %v", string(p))
	} else {
		fmt.Printf("<NA> %v", string(p))
	}

	return len(p), nil
}
