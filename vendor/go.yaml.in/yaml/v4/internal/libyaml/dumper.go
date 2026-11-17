// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// This file contains the Dumper API for writing YAML documents.
//
// Primary functions:
// - Dump: Encode value(s) to YAML (use WithAllDocuments for multi-doc)
// - NewDumper: Create a streaming dumper to io.Writer

package libyaml

import (
	"bytes"
	"io"
	"reflect"
)

// A Dumper writes YAML values to an output stream with configurable options.
// It uses a 3-stage pipeline mirroring the Loader:
//  1. Representer: Go values → Tagged Node tree
//  2. Desolver: Remove inferable tags
//  3. Serializer: Node tree → Events → YAML
type Dumper struct {
	representer *Representer
	desolver    *Desolver
	serializer  *Serializer
	options     *Options
}

// NewDumper returns a new Dumper that writes to w with the given options.
//
// The Dumper should be closed after use to flush all data to w.
func NewDumper(w io.Writer, opts ...Option) (*Dumper, error) {
	o, err := ApplyOptions(opts...)
	if err != nil {
		return nil, err
	}
	return &Dumper{
		representer: NewRepresenter(o), // No writer - builds nodes
		desolver:    NewDesolver(o),
		serializer:  NewSerializer(w, o), // Writer here - emits YAML
		options:     o,
	}, nil
}

// Dump encodes a value to YAML with the given options.
//
// By default, Dump encodes a single value as a single YAML document.
//
// Use [WithAllDocuments] to encode multiple values as a multi-document stream:
//
//	docs := []Config{config1, config2, config3}
//	yaml.Dump(docs, yaml.WithAllDocuments())
//
// When [WithAllDocuments] is used, in must be a slice.
// Each element is encoded as a separate YAML document with "---" separators.
//
// See [yaml.Marshal] for details.
//
// [yaml.Marshal]: https://pkg.go.dev/go.yaml.in/yaml/v4#Marshal
func Dump(in any, opts ...Option) (out []byte, err error) {
	defer handleErr(&err)

	o, err := ApplyOptions(opts...)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	d, err := NewDumper(&buf, func(opts *Options) error {
		*opts = *o // Copy options
		return nil
	})
	if err != nil {
		return nil, err
	}

	if o.AllDocuments {
		// Multi-document mode: in must be a slice
		inVal := reflect.ValueOf(in)
		if inVal.Kind() != reflect.Slice {
			return nil, &DumpError{
				Stage:     RepresenterStage,
				Message:   "WithAllDocuments requires a slice input",
				formatter: o.FormatDumpError,
			}
		}

		// Dump each element as a separate document
		for i := 0; i < inVal.Len(); i++ {
			if err := d.Dump(inVal.Index(i).Interface()); err != nil {
				return nil, err
			}
		}
	} else {
		// Single-document mode
		if err := d.Dump(in); err != nil {
			return nil, err
		}
	}

	if err := d.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Dump writes the YAML encoding of v to the stream.
//
// If multiple values are dumped to the stream, the second and subsequent
// documents will be preceded with a "---" document separator.
//
// See the documentation for [yaml.Marshal] for details about the conversion of Go
// values to YAML.
//
// [yaml.Marshal]: https://pkg.go.dev/go.yaml.in/yaml/v4#Marshal
func (d *Dumper) Dump(v any) (err error) {
	defer handleErr(&err)

	// Stage 1: Represent - Go values → Tagged Node tree
	node := d.representer.Represent("", reflect.ValueOf(v))

	// Stage 2: Desolve - Remove inferable tags
	d.desolver.Desolve(node)

	// Stage 3: Serialize - Node tree → Events → YAML
	d.serializer.Serialize(node)

	return nil
}

// Close closes the Dumper by writing any remaining data.
// It does not write a stream terminating string "...".
func (d *Dumper) Close() (err error) {
	defer handleErr(&err)
	d.serializer.Finish()
	return nil
}

// SetLegacyEncoderIndent allows changing the indentation for the legacy Encoder API.
//
// This method is only provided for backward compatibility with the legacy Encoder API.
// Do not use in libyaml, use [WithIndent] instead.
func (d *Dumper) SetLegacyEncoderIndent(spaces int) {
	if spaces < 0 {
		failDumpf(
			SerializerStage,
			d.options.FormatDumpError,
			"cannot indent to a negative number of spaces",
		)
	}
	// Set on serializer's emitter
	d.serializer.Emitter.BestIndent = spaces
}

// SetLegacyEncoderCompactSeqIndent allows changing the compact sequence indentation for the legacy Encoder API.
//
// This method is only provided for backward compatibility with the legacy Encoder API.
// Do not use in libyaml, use [WithCompactSeqIndent] instead.
func (d *Dumper) SetLegacyEncoderCompactSeqIndent(compact bool) {
	d.serializer.Emitter.CompactSequenceIndent = compact
}
