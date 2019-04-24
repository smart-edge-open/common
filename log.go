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
	"fmt"
	"io"
	"log/syslog"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultLevel is the initial logging verbosity
	DefaultLevel = syslog.LOG_INFO
	// DefaultFacility is the default facility portion of the syslog priority.
	DefaultFacility = syslog.LOG_LOCAL0
)

const (
	severityMask = 0x07
	facilityMask = 0xf8
)

var (
	// DefaultLogger is the package-level logger that is used for all package
	// funcs.
	DefaultLogger = &Logger{}
)

var (
	svcName string
)

func init() {
	svcExe, _ := os.Executable()
	svcName = filepath.Base(svcExe)
}

// SetOutput changes the writer of local logs written by each logging func in
// addition to any remote syslog connection. If w is nil then os.Stderr will be
// used. If no non-remote logging is desired, set output to ioutil.Discard.
func SetOutput(w io.Writer) { DefaultLogger.SetOutput(w) }

// SetFacility alters the syslog facility used for logs. If the priority
// includes a verbosity level it will be ignored.
func SetFacility(p syslog.Priority) { DefaultLogger.SetFacility(p) }

// GetFacility returns the facility portion of the current syslog priority.
func GetFacility() syslog.Priority { return DefaultLogger.GetFacility() }

// ParseLevel is a convienience function that returns syslog.Priority
// Allowed input values: emerg, emergency, alert, crit, critical, err, error,
// warn, warning, notice, info, information, debug
func ParseLevel(prio string) (syslog.Priority, error) {
	var lvl syslog.Priority
	err := fmt.Errorf("Invalid prio: %q", prio)

	switch strings.ToLower(prio) {
	case "emerg", "emergency":
		lvl, err = syslog.LOG_EMERG, nil
	case "alert":
		lvl, err = syslog.LOG_ALERT, nil
	case "crit", "critical":
		lvl, err = syslog.LOG_CRIT, nil
	case "err", "error":
		lvl, err = syslog.LOG_ERR, nil
	case "warning", "warn":
		lvl, err = syslog.LOG_WARNING, nil
	case "notice":
		lvl, err = syslog.LOG_NOTICE, nil
	case "info", "information":
		lvl, err = syslog.LOG_INFO, nil
	case "debug":
		lvl, err = syslog.LOG_DEBUG, nil
	}
	return lvl, err
}

// SetLevel alters the verbosity level that log will print at and below. It
// takes values syslog.LOG_EMERG...syslog.LOG_DEBUG. If the priority includes a
// facility it will be ignored.
func SetLevel(p syslog.Priority) { DefaultLogger.SetLevel(p) }

// GetLevel returns the verbosity level that log will print at and below. It
// can be compared to syslog.LOG_EMERG...syslog.LOG_DEBUG.
func GetLevel() syslog.Priority { return DefaultLogger.GetLevel() }

// ConnectSyslog connects to a remote syslog. If addr is an empty string, it
// will connect to the local syslog service.
func ConnectSyslog(addr string) error { return DefaultLogger.ConnectSyslog(addr) }

// DisconnectSyslog closes the connection to syslog.
func DisconnectSyslog() error { return DefaultLogger.DisconnectSyslog() }

// Print writes message with severity and set facility to output and syslog if connected.
func Print(p syslog.Priority, a ...interface{}) { DefaultLogger.Print(p, a...) }

// Println writes message with severity and set facility to output and syslog if connected.
func Println(p syslog.Priority, a ...interface{}) { DefaultLogger.Println(p, a...) }

// Printf writes message with severity and set facility to output and syslog if connected.
func Printf(p syslog.Priority, frmt string, a ...interface{}) { DefaultLogger.Printf(p, frmt, a...) }

// Debug writes DEBUG message to output and syslog if connected.
func Debug(a ...interface{}) { DefaultLogger.Debug(a...) }

// Debugln writes DEBUG message to output and syslog if connected.
func Debugln(a ...interface{}) { DefaultLogger.Debugln(a...) }

// Debugf writes formatted DEBUG message to output and syslog if connected.
func Debugf(frmt string, a ...interface{}) { DefaultLogger.Debugf(frmt, a...) }

// Info writes INFO message to output and syslog if connected.
func Info(a ...interface{}) { DefaultLogger.Info(a...) }

// Infoln writes INFO message to output and syslog if connected.
func Infoln(a ...interface{}) { DefaultLogger.Info(a...) }

// Infof writes formatted INFO message to output and syslog if connected.
func Infof(frmt string, a ...interface{}) { DefaultLogger.Infof(frmt, a...) }

// Notice writes NOTICE message to output and syslog if connected.
func Notice(a ...interface{}) { DefaultLogger.Notice(a...) }

// Noticeln writes NOTICE message to output and syslog if connected.
func Noticeln(a ...interface{}) { DefaultLogger.Notice(a...) }

// Noticef writes formatted NOTICE message to output and syslog if connected.
func Noticef(frmt string, a ...interface{}) { DefaultLogger.Noticef(frmt, a...) }

// Warning writes WARNING message to output and syslog if connected.
func Warning(a ...interface{}) { DefaultLogger.Warning(a...) }

// Warningln writes WARNING message to output and syslog if connected.
func Warningln(a ...interface{}) { DefaultLogger.Warning(a...) }

// Warningf writes formatted WARNING message to output and syslog if connected.
func Warningf(frmt string, a ...interface{}) { DefaultLogger.Warningf(frmt, a...) }

// Err writes ERROR message to output and syslog if connected.
func Err(a ...interface{}) { DefaultLogger.Err(a...) }

// Errln writes ERROR message to output and syslog if connected.
func Errln(a ...interface{}) { DefaultLogger.Err(a...) }

// Errf writes formatted ERROR message to output and syslog if connected.
func Errf(frmt string, a ...interface{}) { DefaultLogger.Errf(frmt, a...) }

// Crit writes CRITICAL message to output and syslog if connected.
func Crit(a ...interface{}) { DefaultLogger.Crit(a...) }

// Critln writes CRITICAL message to output and syslog if connected.
func Critln(a ...interface{}) { DefaultLogger.Crit(a...) }

// Critf writes formatted CRITICAL message to output and syslog if connected.
func Critf(frmt string, a ...interface{}) { DefaultLogger.Critf(frmt, a...) }

// Alert writes ALERT message to output and syslog if connected.
func Alert(a ...interface{}) { DefaultLogger.Alert(a...) }

// Alertln writes ALERT message to output and syslog if connected.
func Alertln(a ...interface{}) { DefaultLogger.Alert(a...) }

// Alertf writes formatted ALERT message to output and syslog if connected.
func Alertf(frmt string, a ...interface{}) { DefaultLogger.Alertf(frmt, a...) }

// Emerg writes EMERGENCY message to output and syslog if connected.
func Emerg(a ...interface{}) { DefaultLogger.Emerg(a...) }

// Emergln writes EMERGENCY message to output and syslog if connected.
func Emergln(a ...interface{}) { DefaultLogger.Emerg(a...) }

// Emergf writes formatted EMERGENCY message to output and syslog if connected.
func Emergf(frmt string, a ...interface{}) { DefaultLogger.Emergf(frmt, a...) }
