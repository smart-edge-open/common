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
	"regexp"
	"testing"

	"github.com/smartedgemec/log"
)

func TestPrinterPrint(t *testing.T) {
	tests := map[string]struct {
		printerLvl syslog.Priority
		inputLvl   syslog.Priority
		inputMsg   string
		expect     *regexp.Regexp
	}{
		"debug message at debug level": {
			printerLvl: syslog.LOG_DEBUG,
			inputLvl:   syslog.LOG_DEBUG,
			inputMsg:   "hello",
			expect:     regexp.MustCompile(`: hello\n$`),
		},
		"debug message at info level": {
			printerLvl: syslog.LOG_INFO,
			inputLvl:   syslog.LOG_DEBUG,
			inputMsg:   "hello",
			expect:     regexp.MustCompile(`^$`),
		},
	}

	for desc, test := range tests {
		var buf bytes.Buffer

		log := new(log.Logger)
		log.SetLevel(test.printerLvl)
		log.SetOutput(&buf)

		log.Print(test.inputLvl, test.inputMsg)
		if actual := buf.String(); !test.expect.MatchString(actual) {
			t.Errorf("[%s] expected to match regexp %q, got %q", desc, test.expect, actual)
		}
	}
}
