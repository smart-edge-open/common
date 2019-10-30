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

package log

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// SignalVerbosityChanges captures SIGUSR1 and SIGUSR2 and decreases and
// increases verbosity on each signal, respectively.
//
// This function spawns a goroutine in order to make it safe to send a USR1 or
// USR2 signal as soon as the function has returned.
func SignalVerbosityChanges(ctx context.Context, l *Logger) {
	decC := make(chan os.Signal, 1)
	incC := make(chan os.Signal, 1)
	signal.Notify(decC, syscall.SIGUSR1)
	signal.Notify(incC, syscall.SIGUSR2)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-decC:
				l.priorityMu.Lock()
				l.setLevel(l.getLevel() - 1)
				l.priorityMu.Unlock()
			case <-incC:
				l.priorityMu.Lock()
				l.setLevel(l.getLevel() + 1)
				l.priorityMu.Unlock()
			}
		}
	}()
}
