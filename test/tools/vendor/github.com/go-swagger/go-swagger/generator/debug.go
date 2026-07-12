// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	// Debug when the env var DEBUG or SWAGGER_DEBUG is not empty
	// the generators will be very noisy about what they are doing.
	Debug = os.Getenv("DEBUG") != "" || os.Getenv("SWAGGER_DEBUG") != ""
	// generatorLogger is a debug logger for this package.
	generatorLogger *log.Logger
)

func debugOptions() {
	generatorLogger = log.New(os.Stdout, "generator:", log.LstdFlags)
}

// debugLog wraps log.Printf with a debug-specific logger.
func debugLogf(format string, args ...any) {
	if !Debug {
		return
	}

	_, file, pos, _ := runtime.Caller(1)
	safeArgs := sanitizeDebugLogArgs(args...)
	generatorLogger.Printf("%s:%d: %s", filepath.Base(file), pos,
		fmt.Sprintf(format, safeArgs...))
}

// debugLogAsJSON unmarshals its last arg as pretty JSON.
func debugLogAsJSONf(format string, args ...any) {
	if !Debug {
		return
	}

	var dfrmt string
	const extraNumArgs = 2
	_, file, pos, _ := runtime.Caller(1)
	dargs := make([]any, 0, len(args)+extraNumArgs)
	dargs = append(dargs, filepath.Base(file), pos)

	if len(args) > 0 {
		dfrmt = "%s:%d: " + format + "\n%s"
		bbb, _ := json.MarshalIndent(args[len(args)-1], "", " ") //nolint:errchkjson // OK: it's okay for debug
		dargs = append(dargs, args[0:len(args)-1]...)
		dargs = append(dargs, string(bbb))
	} else {
		dfrmt = "%s:%d: " + format
	}

	generatorLogger.Printf(dfrmt, dargs...)
}

// sanitizeDebugLogArgs traverses arguments to debugLog and redacts fields
// that may contain sensitive information, such as API keys or credentials.
func sanitizeDebugLogArgs(args ...any) []any {
	safeArgs := make([]any, len(args))
	for i, arg := range args {
		safeArgs[i] = sanitizeValue(arg)
	}

	return safeArgs
}

// sanitizeValue redacts sensitive information from known data structures.
// It can be expanded for more types over time as needed.
func sanitizeValue(val any) any {
	switch v := val.(type) {
	case map[string]any:
		// recursively sanitize map values
		res := make(map[string]any, len(v))
		for k, subv := range v {
			if k == "IsAPIKeyAuth" || k == "TokenURL" { // false positive: this is a bool indicator, not a sensitive value
				continue
			}

			lower := strings.ToLower(k)
			if lower == "apikey" || lower == "token" ||
				lower == "secret" ||
				strings.Contains(lower, "password") ||
				strings.Contains(lower, "apikey") ||
				strings.Contains(lower, "token") {
				res[k] = "***REDACTED***"

				continue
			}

			res[k] = sanitizeValue(subv)
		}
		return res

	case []any:
		res := make([]any, len(v))
		for i, subv := range v {
			res[i] = sanitizeValue(subv)
		}
		return res

	case string:
		// heuristic: redact if looks like a key/secret
		lower := strings.ToLower(v)
		if strings.Contains(lower, "apikey") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") ||
			strings.Contains(lower, "password") {
			return "***REDACTED***"
		}
		return v

	default:
		// Optionally, process struct types for known sensitive fields
		return v
	}
}

// fatal wraps [log.Fatal] with extra context provided in debug mode.
func fatal(v ...any) {
	fatalln(v...)
}

// fatalln wraps [log.Fatalln] with extra context provided in debug mode.
func fatalln(v ...any) {
	if Debug {
		b := fmt.Appendln([]byte{}, v...)
		traceFatalf("%s", b)
	}

	log.Fatalln(v...)
}

// traceFatalf allows to capture more context about the caller of a fatalX function.
//
// This output is not disabled when muting the [log.Logger] (e.g. when running tests).
func traceFatalf(format string, v ...any) {
	const callstackOffset = 3

	_, file, pos, _ := runtime.Caller(callstackOffset)
	safeArgs := sanitizeDebugLogArgs(v...)
	fmt.Fprintf(os.Stderr, "fatal error: %s:%d: %s\n", filepath.Base(file), pos, fmt.Sprintf(format, safeArgs...))
}
