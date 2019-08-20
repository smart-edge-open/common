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

package examples_test

import (
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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
			syscall.Kill(pid, syscall.SIGINT)
		}
	}(5, -cmd.Process.Pid)

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
