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

package log_test

import (
	"context"
	"log/syslog"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/open-ness/common"
)

func TestSignalVerbosityChanges(t *testing.T) {
	defer func() { log.DefaultLogger = new(log.Logger) }()

	var (
		ctx, cancel = context.WithCancel(context.Background())
		pid         = os.Getpid()
		timeout     = time.After(time.Second)
	)
	defer cancel()
	log.SignalVerbosityChanges(ctx, log.DefaultLogger)

	// Decrease dynamically
	log.SetLevel(syslog.LOG_DEBUG)
	if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
		t.Fatalf("got error sending USR1 signal to self: %v", err)
	}
WaitForDecrease:
	for {
		if lvl := log.GetLevel(); lvl == syslog.LOG_INFO {
			break WaitForDecrease
		}
		select {
		case <-timeout:
			t.Fatalf("timed out before signal decreased verbosity")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Increase dynamically
	log.SetLevel(syslog.LOG_EMERG)
	if err := syscall.Kill(pid, syscall.SIGUSR2); err != nil {
		t.Fatalf("got error sending USR2 signal to self: %v", err)
	}
WaitForIncrease:
	for {
		if lvl := log.GetLevel(); lvl == syslog.LOG_ALERT {
			break WaitForIncrease
		}
		select {
		case <-timeout:
			t.Fatalf("timed out before signal increased verbosity")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
