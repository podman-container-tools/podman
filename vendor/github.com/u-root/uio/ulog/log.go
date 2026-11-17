// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ulog exposes logging via a Go interface.
//
// ulog has three implementations of the Logger interface: a Go standard
// library "log" package Logger and a test Logger that logs via a test's
// testing.TB.Logf. To use the test logger import "ulog/ulogtest".
package ulog

import "log"

// Logger is a log receptacle.
//
// It puts your information somewhere for safekeeping.
type Logger interface {
	Printf(format string, v ...interface{})
}

// Log is a Logger that prints to the log package's default logger.
var Log Logger = log.Default()

type emptyLogger struct{}

// Printf implements Logger.Printf.
func (emptyLogger) Printf(format string, v ...interface{}) {}

// Null is a logger that prints nothing.
var Null Logger = emptyLogger{}
