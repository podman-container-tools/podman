// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// This file contains the Loader API for reading YAML documents.
//
// Primary functions:
// - Load: Decode YAML document(s) into a value (use WithAllDocuments for multi-doc)
// - NewLoader: Create a streaming loader from io.Reader

package libyaml

import (
	"bytes"
	"errors"
	"io"
	"reflect"
)

// A Loader reads and loads YAML values from an input stream with configurable
// options.
type Loader struct {
	composer    *Composer
	resolver    *Resolver
	constructor *Constructor
	options     *Options
	docCount    int
}

// NewLoader returns a new Loader that reads from r with the given options.
//
// The Loader introduces its own buffering and may read data from r beyond the
// YAML values requested.
func NewLoader(r io.Reader, opts ...Option) (*Loader, error) {
	o, err := ApplyOptions(opts...)
	if err != nil {
		return nil, err
	}
	c := NewComposerFromReader(r, o)
	c.SetStreamNodes(o.StreamNodes)
	return &Loader{
		composer:    c,
		resolver:    NewResolver(o),
		constructor: NewConstructor(o),
		options:     o,
	}, nil
}

// Load loads YAML document(s) with the given options.
//
// By default, Load requires exactly one document in the input.
// If zero documents are found, it returns an error.
// If multiple documents are found, it returns an error.
//
// Use [WithAllDocuments] to load all documents into a slice:
//
//	var configs []Config
//	yaml.Load(multiDocYAML, &configs, yaml.WithAllDocuments())
//
// When [WithAllDocuments] is used, out must be a pointer to a slice.
// Each document is loaded into the slice element type.
// Zero documents results in an empty slice (no error).
//
// Maps and pointers (to a struct, string, int, etc) are accepted as out
// values. If an internal pointer within a struct is not initialized,
// the yaml package will initialize it if necessary. The out parameter
// must not be nil.
//
// The type of the loaded values should be compatible with the respective
// values in out. If one or more values cannot be loaded due to type
// mismatches, decoding continues partially until the end of the YAML
// content, and a *yaml.LoadErrors is returned with details for all
// missed values.
//
// Struct fields are only loaded if they are exported (have an upper case
// first letter), and are loaded using the field name lowercased as the
// default key. Custom keys may be defined via the "yaml" name in the field
// tag: the content preceding the first comma is used as the key, and the
// following comma-separated options control the loading and dumping behavior.
//
// For example:
//
//	type T struct {
//	    F int `yaml:"a,omitempty"`
//	    B int
//	}
//	var t T
//	yaml.Load([]byte("a: 1\nb: 2"), &t)
//
// See the documentation of Dump for the format of tags and a list of
// supported tag options.
func Load(in []byte, out any, opts ...Option) error {
	o, err := ApplyOptions(opts...)
	if err != nil {
		return err
	}

	if o.AllDocuments {
		// Multi-document mode: out must be pointer to slice
		return loadAll(in, out, o)
	}

	// Single-document mode: exactly one document required
	return loadSingle(in, out, o)
}

// Load reads the next YAML-encoded document from its input and stores it
// in the value pointed to by v.
//
// Returns [io.EOF] when there are no more documents to read.
// If the [WithSingleDocument] option was set and a document was already read,
// subsequent calls return [io.EOF].
//
// Maps and pointers (to a struct, string, int, etc) are accepted as v
// values. If an internal pointer within a struct is not initialized,
// the yaml package will initialize it if necessary. The v parameter
// must not be nil.
//
// Struct fields are only loaded if they are exported (have an upper case
// first letter), and are loaded using the field name lowercased as the
// default key. Custom keys may be defined via the "yaml" name in the field
// tag: the content preceding the first comma is used as the key, and the
// following comma-separated options control the loading and dumping behavior.
//
// See the documentation of the package-level [Load] function for more details
// about YAML to Go conversion and tag options.
func (l *Loader) Load(v any) (err error) {
	defer handleErr(&err)
	if l.options.SingleDocument && l.docCount > 0 {
		return io.EOF
	}

	// Stage 1: Compose - parse events into node tree (unresolved tags)
	node := l.composer.Compose() // *Node
	if node == nil {
		return io.EOF
	}
	l.docCount++

	// Stage 2: Resolve - determine implicit types for untagged scalars
	l.resolver.Resolve(node)

	// Stage 3: Construct - convert node tree to Go values
	out := reflect.ValueOf(v)
	if out.Kind() == reflect.Pointer && !out.IsNil() {
		out = out.Elem()
	}
	l.constructor.Construct(node, out)
	if len(l.constructor.TypeErrors) > 0 {
		typeErrors := l.constructor.TypeErrors
		l.constructor.TypeErrors = nil
		return &LoadErrors{Errors: typeErrors}
	}
	return nil
}

// loadAll loads all documents from the input into a slice.
// The out parameter must be a non-nil pointer to a slice.
// Each document is appended to the slice as an element.
func loadAll(in []byte, out any, opts *Options) error {
	outVal := reflect.ValueOf(out)
	if outVal.Kind() != reflect.Pointer || outVal.IsNil() {
		msg := "yaml: WithAllDocuments requires a non-nil pointer to a slice"
		return &LoadErrors{Errors: []*LoadError{{
			Stage:   ConstructorStage,
			Message: msg,
			err:     errors.New(msg),
		}}}
	}

	sliceVal := outVal.Elem()
	if sliceVal.Kind() != reflect.Slice {
		msg := "yaml: WithAllDocuments requires a pointer to a slice"
		return &LoadErrors{Errors: []*LoadError{{
			Stage:   ConstructorStage,
			Message: msg,
			err:     errors.New(msg),
		}}}
	}

	// Create a new slice (clear existing content)
	sliceVal.Set(reflect.MakeSlice(sliceVal.Type(), 0, 0))

	l, err := NewLoader(bytes.NewReader(in), func(o *Options) error {
		*o = *opts // Copy options
		return nil
	})
	if err != nil {
		return err
	}

	elemType := sliceVal.Type().Elem()
	for {
		// Create new element of slice's element type
		elemPtr := reflect.New(elemType)
		err := l.Load(elemPtr.Interface())
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Append loaded element to slice
		sliceVal.Set(reflect.Append(sliceVal, elemPtr.Elem()))
	}

	return nil
}

// loadSingle loads exactly one document from the input.
// Returns an error if the input contains zero or multiple documents
// (unless [Options.allowLegacyTrailingContent] option is set for backward compatibility).
func loadSingle(in []byte, out any, opts *Options) error {
	l, err := NewLoader(bytes.NewReader(in), func(o *Options) error {
		*o = *opts // Copy options
		return nil
	})
	if err != nil {
		return err
	}

	// Load first document
	err = l.Load(out)
	if err == io.EOF {
		if opts.allowLegacyTrailingContent {
			return nil
		}
		msg := "yaml: no documents in stream"
		return &LoadErrors{Errors: []*LoadError{{
			Stage:   ConstructorStage,
			Message: msg,
			err:     errors.New(msg),
		}}}
	}
	if err != nil {
		return err
	}

	// Skip trailing document check for legacy Unmarshal() compatibility
	if opts.allowLegacyTrailingContent {
		return nil
	}

	// Check for additional documents
	var dummy any
	err = l.Load(&dummy)
	if err != io.EOF {
		if err != nil {
			// Some other error occurred
			return err
		}
		// Successfully loaded a second document - this is an error in strict mode
		msg := "yaml: expected single document, found multiple"
		return &LoadErrors{Errors: []*LoadError{{
			Stage:   ConstructorStage,
			Message: msg,
			err:     errors.New(msg),
		}}}
	}

	return nil
}

// SetLegacyLoaderKnownFields allows changing the known fields setting from the legacy Decoder API.
//
// This method is only provided for backward compatibility with the legacy Decoder API.
// Do not use in libyaml, use [WithKnownFields] instead.
func (l *Loader) SetLegacyLoaderKnownFields(enable bool) {
	l.constructor.KnownFields = enable
}

// LoadAny parses YAML data into generic Go structures (map[string]any, []any).
//
// Useful for test data loading where the structure is unknown at compile time.
// This is a convenience wrapper around Load with an any target.
func LoadAny(data []byte) (any, error) {
	var result any
	if err := Load(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
