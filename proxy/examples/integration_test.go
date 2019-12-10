// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2019 Intel Corporation

package examples_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestZeroProxyExample(t *testing.T) {
	defer doBuild(t, "zero_proxy")()

	// Run Zero Proxy example
	cmd := exec.Command("./zero_proxy")
	cmd.Dir = "./zero_proxy"
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	done := make(chan struct{})
	defer close(done)
	if err := cmd.Start(); err != nil {
		t.Fatalf("error starting process: %v", err)
	}

	// Kill after N seconds
	go func(sec, pid int) {
		select {
		case <-done:
		case <-time.After(time.Duration(sec) * time.Second):
			fmt.Printf("Time is up - killing the main process\n")
			syscall.Kill(pid, syscall.SIGINT)
		}
	}(9, cmd.Process.Pid)

	// Check exit code
	cmd.Wait()
	if !cmd.ProcessState.Success() {
		t.Fatalf("process exited: %#v", cmd.ProcessState)
	}
}

func doBuild(t *testing.T, s string) (cleanup func()) {
	cmd := exec.Command("go", "build", ".")
	cmd.Dir = filepath.Join(".", s)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("error building example: %v", err)
	}
	return func() { os.Remove(filepath.Join(".", s, s)) }
}
