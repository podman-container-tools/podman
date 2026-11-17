// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Error types for YAML parsing and emitting.
// Provides structured error reporting with line/column information.

package libyaml

import (
	"errors"
	"fmt"
	"strings"
)

// Stage identifies the processing stage where an error occurred during YAML
// loading or dumping.
type Stage string

const (
	// Load stages
	ReaderStage      Stage = "reader"      // Input reading and encoding
	ScannerStage     Stage = "scanner"     // Tokenization
	ParserStage      Stage = "parser"      // Event stream parsing
	ComposerStage    Stage = "composer"    // Node tree construction
	ResolverStage    Stage = "resolver"    // Tag resolution
	ConstructorStage Stage = "constructor" // Go value construction

	// Dump stages
	RepresenterStage Stage = "representer" // Go value to Node tree
	SerializerStage  Stage = "serializer"  // Node tree to events
	EmitterStage     Stage = "emitter"     // Events to YAML bytes
	WriterStage      Stage = "writer"      // Output writing
)

// LoadError represents an error that occurred while loading a YAML document.
//
// It provides detailed location information and identifies the processing
// stage where the error occurred.
type LoadError struct {
	// Stage identifies the processing stage where the error occurred.
	Stage Stage
	// Message provides a description of the error.
	Message string

	// Position information

	// Mark is the primary position in the input where the error occurred.
	Mark Mark
	// ContextMark is an optional context position (e.g., start of construct)
	ContextMark Mark
	// ContextMsg is an optional message describing the context of the error.
	ContextMsg string

	// Error chaining
	err error // Underlying error (for Unwrap support)

	// formatter is an optional function that controls the error string
	// representation. Set by the error-format plugin via Options.FormatLoadError.
	formatter func(*LoadError) string
}

// Error returns the error message with stage and position information.
// If a formatter is set (via the error-format plugin), it is called instead.
// Default format: "go-yaml load error in <stage> at L:C: <message>"
// Or with context: "go-yaml load error in <stage> (<ctx>) at L:C-L:C: <message>"
func (e *LoadError) Error() string {
	if e.formatter != nil {
		return e.formatter(e)
	}
	if len(e.ContextMsg) > 0 {
		return fmt.Sprintf("go-yaml load error in %s (%s) at %s: %s",
			e.Stage, e.ContextMsg, e.ContextMark.rangeString(e.Mark), e.Message)
	}
	return fmt.Sprintf("go-yaml load error in %s at %s: %s",
		e.Stage, e.Mark.shortString(), e.Message)
}

// simpleError returns a compact error message for use within LoadErrors
// collections.
// Default format: "line L: <message>" (backwards compatible - no column info)
// Note: simpleError does not invoke the formatter; that is only used by Error().
func (e *LoadError) simpleError() string {
	var builder strings.Builder
	if len(e.ContextMsg) > 0 {
		fmt.Fprintf(&builder, "%s at %s: ", e.ContextMsg, e.ContextMark)
	}
	if len(e.ContextMsg) == 0 || e.ContextMark != e.Mark {
		if e.Mark.Line > 0 {
			fmt.Fprintf(&builder, "line %d: ", e.Mark.Line)
		} else {
			builder.WriteString("<unknown position>: ")
		}
	}
	builder.WriteString(e.Message)
	return builder.String()
}

// Unwrap returns the underlying error.
func (e *LoadError) Unwrap() error {
	return e.err
}

// NewLoadError creates a LoadError with an underlying cause.
// The cause is accessible via Unwrap for use with [errors.Is] and [errors.As].
func NewLoadError(stage Stage, message string, mark Mark, cause error) *LoadError {
	return &LoadError{
		Stage:   stage,
		Message: message,
		Mark:    mark,
		err:     cause,
	}
}

func formatLoadError(err *LoadError, formatter func(*LoadError) string) *LoadError {
	if formatter == nil || err.formatter != nil {
		return err
	}
	dup := *err
	dup.formatter = formatter
	return &dup
}

// DumpError represents an error that occurred while dumping a YAML document.
//
// It identifies the processing stage where the error occurred and provides
// an optional underlying cause via Unwrap.
type DumpError struct {
	// Stage identifies the processing stage where the error occurred.
	Stage Stage
	// Message provides a description of the error.
	Message string

	// Error chaining
	err error // Underlying error (for Unwrap support)

	// formatter is an optional function that controls the error string
	// representation. Set by the error-format plugin via Options.FormatDumpError.
	formatter func(*DumpError) string
}

// Error returns the error message with stage information.
// Format: "go-yaml dump error in <stage>: <message>"
func (e *DumpError) Error() string {
	if e.formatter != nil {
		return e.formatter(e)
	}
	return fmt.Sprintf("go-yaml dump error in %s: %s", e.Stage, e.Message)
}

// Unwrap returns the underlying error.
func (e *DumpError) Unwrap() error {
	return e.err
}

// NewDumpError creates a DumpError with an underlying cause.
// The cause is accessible via Unwrap for use with [errors.Is] and [errors.As].
func NewDumpError(stage Stage, message string, cause error) *DumpError {
	return &DumpError{Stage: stage, Message: message, err: cause}
}

func newDumpError(
	stage Stage,
	message string,
	cause error,
	formatter func(*DumpError) string,
) *DumpError {
	return &DumpError{
		Stage:     stage,
		Message:   message,
		err:       cause,
		formatter: formatter,
	}
}

func formatDumpError(err *DumpError, formatter func(*DumpError) string) *DumpError {
	if formatter == nil || err.formatter != nil {
		return err
	}
	dup := *err
	dup.formatter = formatter
	return &dup
}

// failDump panics with a YAMLError wrapping a DumpError for the given stage.
// If err is exactly a *DumpError it is passed through unchanged to avoid
// double-wrapping (e.g. a user MarshalYAML that returns yaml.NewDumpError).
// Errors that merely wrap a *DumpError are treated as ordinary errors so that
// the outer wrapper's message and context are preserved.
func failDump(stage Stage, err error, formatter func(*DumpError) string) {
	if de, ok := err.(*DumpError); ok {
		panic(&YAMLError{formatDumpError(de, formatter)})
	}
	panic(&YAMLError{newDumpError(stage, err.Error(), err, formatter)})
}

// failDumpf panics with a YAMLError wrapping a formatted DumpError.
func failDumpf(
	stage Stage,
	formatter func(*DumpError) string,
	format string,
	args ...any,
) {
	panic(&YAMLError{newDumpError(stage, fmt.Sprintf(format, args...), nil, formatter)})
}

// EmitterError represents an error that occurred during emitting.
type EmitterError struct {
	Message string
}

// Error returns the error message.
func (e EmitterError) Error() string {
	return "yaml: " + e.Message
}

// WriterError represents an error that occurred while writing output.
type WriterError struct {
	Err error
}

// Error returns the error message.
func (e WriterError) Error() string {
	return fmt.Sprintf("yaml: %s", e.Err)
}

// Unwrap returns the underlying error.
func (e WriterError) Unwrap() error {
	return e.Err
}

// LoadErrors is returned when one or more fields cannot be properly decoded.
type LoadErrors struct {
	Errors []*LoadError
}

// Error returns a formatted error message listing all construct errors.
func (e *LoadErrors) Error() string {
	var b strings.Builder
	b.WriteString("yaml: construct errors: ")
	for i, err := range e.Errors {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(err.simpleError())
	}
	return b.String()
}

// As implements [errors.As] for Go versions prior to 1.20 that don't support
// the Unwrap() []error interface. It allows [LoadErrors] to match against
// *LoadError or *TypeError targets.
func (e *LoadErrors) As(target any) bool {
	switch t := target.(type) {
	case **LoadError:
		if len(e.Errors) == 0 {
			return false
		}
		*t = e.Errors[0]
		return true
	case **TypeError:
		var msgs []string
		for _, err := range e.Errors {
			msgs = append(msgs, err.simpleError())
		}
		*t = &TypeError{Errors: msgs}
		return true
	}
	return false
}

// Is implements [errors.Is] for Go versions prior to 1.20 that don't support
// the Unwrap() []error interface. It checks if any wrapped error matches
// the target error.
func (e *LoadErrors) Is(target error) bool {
	for _, err := range e.Errors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// TypeError is a legacy error type retained for compatibility.
//
// [Load] and [Loader.Load] return *LoadErrors when one or more fields cannot
// be decoded into the requested types.
// The destination value may be partially populated.
// For compatibility, [LoadErrors.As] synthesizes a *TypeError when callers
// use [errors.As].
//
// Deprecated: Use [LoadErrors] instead.
type TypeError struct {
	Errors []string
}

// Error returns a formatted error message listing all unmarshal errors.
func (e *TypeError) Error() string {
	return "yaml: unmarshal errors: " + strings.Join(e.Errors, "; ")
}

// YAMLError is an internal error wrapper type.
type YAMLError struct {
	Err error
}

// Error returns the error message.
func (e *YAMLError) Error() string {
	return e.Err.Error()
}

// handleErr recovers from panics caused by yaml errors.
// It's used in defer statements to convert YAMLError panics into regular errors.
func handleErr(err *error) {
	if v := recover(); v != nil {
		if e, ok := v.(*YAMLError); ok {
			*err = e.Err
		} else {
			panic(v)
		}
	}
}
