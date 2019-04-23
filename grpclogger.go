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
	"log/syslog"
	"os"
	"sync"
)

// GrpcLogger implements grpclog's Logger and LoggerV2 interfaces.
type GrpcLogger struct {
	// Logger is the nderlying Logger to write to. If none is specified, then
	// the default logger (package var) is used.
	Logger *Logger

	// PrintLevel specifies the level that all print messages will be written
	// at. If none is specified, the default of INFO will be used.
	PrintLevel syslog.Priority

	once sync.Once
}

func (l *GrpcLogger) init() {
	if l.Logger == nil {
		l.Logger = DefaultLogger
	}
	if l.PrintLevel == 0 {
		l.PrintLevel = syslog.LOG_INFO
	}
}

// Print logs to the level set at init. Arguments are handled in the manner
// of fmt.Print.
func (l *GrpcLogger) Print(args ...interface{}) {
	l.once.Do(l.init)
	l.Logger.Print(l.PrintLevel, args...)
}

// Println logs to the level set at init. Arguments are handled in the
// manner of fmt.Println.
func (l *GrpcLogger) Println(args ...interface{}) {
	l.Print(args...)
}

// Printf logs to the level set at init. Arguments are handled in the
// manner of fmt.Printf.
func (l *GrpcLogger) Printf(format string, args ...interface{}) {
	l.once.Do(l.init)
	l.Logger.Printf(l.PrintLevel, format, args...)
}

// Info logs to INFO log. Arguments are handled in the manner of fmt.Print.
func (l *GrpcLogger) Info(args ...interface{}) {
	l.once.Do(l.init)
	l.Logger.Info(args...)
}

// Infoln logs to INFO log. Arguments are handled in the manner of fmt.Println.
func (l *GrpcLogger) Infoln(args ...interface{}) { l.Info(args...) }

// Infof logs to INFO log. Arguments are handled in the manner of fmt.Printf.
func (l *GrpcLogger) Infof(format string, args ...interface{}) {
	l.once.Do(l.init)
	l.Logger.Infof(format, args...)
}

// Warning logs to WARNING log. Arguments are handled in the manner of fmt.Print.
func (l *GrpcLogger) Warning(args ...interface{}) {
	l.once.Do(l.init)
	l.Logger.Warning(args...)
}

// Warningln logs to WARNING log. Arguments are handled in the manner of fmt.Println.
func (l *GrpcLogger) Warningln(args ...interface{}) { l.Warning(args...) }

// Warningf logs to WARNING log. Arguments are handled in the manner of fmt.Printf.
func (l *GrpcLogger) Warningf(format string, args ...interface{}) {
	l.once.Do(l.init)
	l.Logger.Warningf(format, args...)
}

// Error logs to ERROR log. Arguments are handled in the manner of fmt.Print.
func (l *GrpcLogger) Error(args ...interface{}) {
	l.once.Do(l.init)
	l.Logger.Err(args...)
}

// Errorln logs to ERROR log. Arguments are handled in the manner of fmt.Println.
func (l *GrpcLogger) Errorln(args ...interface{}) { l.Error(args...) }

// Errorf logs to ERROR log. Arguments are handled in the manner of fmt.Printf.
func (l *GrpcLogger) Errorf(format string, args ...interface{}) {
	l.once.Do(l.init)
	l.Logger.Errf(format, args...)
}

// Fatal logs to ERROR log. Arguments are handled in the manner of fmt.Print.
// This function should call os.Exit() with a non-zero exit code.
func (l *GrpcLogger) Fatal(args ...interface{}) {
	l.once.Do(l.init)
	l.Logger.Alert(args...)
	os.Exit(1)
}

// Fatalln logs to ERROR log. Arguments are handled in the manner of fmt.Println.
// This function should call os.Exit() with a non-zero exit code.
func (l *GrpcLogger) Fatalln(args ...interface{}) { l.Fatal(args...) }

// Fatalf logs to ERROR log. Arguments are handled in the manner of fmt.Printf.
// This function should call os.Exit() with a non-zero exit code.
func (l *GrpcLogger) Fatalf(format string, args ...interface{}) {
	l.once.Do(l.init)
	l.Logger.Alertf(format, args...)
	os.Exit(1)
}

// V reports whether verbosity level l is at least the requested verbose level.
func (l *GrpcLogger) V(level int) bool {
	l.once.Do(l.init)
	return syslog.Priority(level) >= l.Logger.GetLevel()
}
