// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"fmt"
	"go/types"
	"log"
)

// logCallerDepth is the caller depth for log.Output.
const logCallerDepth = 2

func DebugLogf(debug bool, format string, args ...any) {
	if debug {
		_ = log.Output(logCallerDepth, fmt.Sprintf(format, args...))
	}
}

// UnsupportedTypeKind emits a uniform warning when a go/types kind
// cannot be translated to a Swagger 2.0 construct.
//
// The scanner runs on arbitrary user code in uncontrolled environments,
// so encountering an unsupported kind must not panic — we log and let
// the caller skip. `where` is the dispatcher site (typically the
// function name) so a future go/types evolution — e.g. a new kind we
// haven't modeled — surfaces one grep-able diagnostic instead of
// disappearing behind a silent default.
func UnsupportedTypeKind(where string, tpe types.Type) {
	log.Printf("WARNING: %s: unsupported Go type kind %[2]T (%[2]v); skipping", where, tpe)
}
