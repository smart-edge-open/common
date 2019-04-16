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
	"bytes"
	"log/syslog"
	"strings"
	"testing"

	"github.com/smartedgemec/log"
)

func TestDefaultLoggerSetOutput(t *testing.T) {
	defer func() { log.DefaultLogger = new(log.Logger) }()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.Info("hello")
	if !strings.HasSuffix(buf.String(), "hello\n") {
		t.Errorf("expected %q to end with 'hello\\n'", buf.String())
	}
}

func TestDefaultLoggerPriority(t *testing.T) {
	defer func() { log.DefaultLogger = new(log.Logger) }()

	var (
		defaultLevel    = log.DefaultLevel
		defaultFacility = log.DefaultFacility
	)

	// Test default priority
	if lvl := log.GetLevel(); lvl != defaultLevel {
		t.Errorf("expected default syslog level %d, got %d", defaultLevel, lvl)
	}
	if fac := log.GetFacility(); fac != defaultFacility {
		t.Errorf("expected default syslog facility %d, got %d", defaultLevel, fac)
	}

	// Test setting severity
	log.SetLevel(syslog.LOG_DEBUG)
	if lvl := log.GetLevel(); lvl != syslog.LOG_DEBUG {
		t.Errorf("expected syslog level %d, got %d", syslog.LOG_DEBUG, lvl)
	}
	log.SetLevel(syslog.LOG_CRIT)
	if lvl := log.GetLevel(); lvl != syslog.LOG_CRIT {
		t.Errorf("expected syslog level %d, got %d", syslog.LOG_CRIT, lvl)
	}

	// Test setting facility
	log.SetFacility(syslog.LOG_MAIL)
	if fac := log.GetFacility(); fac != syslog.LOG_MAIL {
		t.Errorf("expected syslog facility %d, got %d", syslog.LOG_MAIL, fac)
	}
	log.SetFacility(syslog.LOG_LOCAL5)
	if fac := log.GetFacility(); fac != syslog.LOG_LOCAL5 {
		t.Errorf("expected syslog facility %d, got %d", syslog.LOG_LOCAL5, fac)
	}
}
