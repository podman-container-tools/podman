// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Options configuration for loading and dumping YAML.
// Provides centralized control for indentation, line width, strictness, and
// more.

package libyaml

import (
	"errors"
	"fmt"
)

// Options holds configuration for both loading and dumping YAML.
type Options struct {
	// Loading options

	// KnownFields enforces strict field checking when loading into structs.
	KnownFields bool
	// SingleDocument controls whether only the first document is loaded from a stream.
	SingleDocument bool
	// UniqueKeys enforces that mapping keys are unique when loading.
	UniqueKeys bool
	// StreamNodes enables returning stream boundary nodes when loading YAML.
	StreamNodes bool
	// AllDocuments enables multi-document mode for [Load] and [Dump] operations.
	AllDocuments bool

	// Dumping options
	// Indent sets the number of spaces to use for indentation when dumping YAML content (2-9).
	Indent int
	// CompactSeqIndent controls whether the sequence indicator '- ' is considered part of the indentation.
	CompactSeqIndent bool
	// LineWidth sets the preferred line width for YAML output (-1 for unlimited).
	LineWidth int
	// Unicode controls whether non-ASCII characters are allowed in YAML output.
	Unicode bool
	// Canonical forces canonical YAML output format (explicit tags for all values).
	Canonical bool
	// LineBreak sets the line ending style for YAML output (LN, CR, CRLF).
	LineBreak LineBreak
	// ExplicitStart controls whether document start markers (---) are always emitted.
	ExplicitStart bool
	// ExplicitEnd controls whether document end markers (...) are always emitted.
	ExplicitEnd bool
	// FlowSimpleCollections controls whether simple collections use flow style.
	FlowSimpleCollections bool
	// QuotePreference sets the preferred quote style for strings that require quoting.
	QuotePreference QuoteStyle

	// Parser supplies complete events in place of native parsing.
	Parser ParserPlugin

	// MergeKey selects the mapping merge algorithm.
	MergeKey MergeKeyPlugin

	// AliasData supplies per-stream anchor policy. Nil selects ordinary
	// document-scoped anchors, rather than disabling aliases.
	AliasData AliasDataPlugin

	// IndentConfig enables tab-aware structural indentation.
	IndentConfig *IndentConfig

	// Error formatting (set by WithPlugin with an error-format plugin).
	FormatLoadError func(*LoadError) string
	FormatDumpError func(*DumpError) string

	// Loader limit checks (set by ApplyOptions or a loader-limits plugin).
	// DepthCheck is called to verify the current nesting depth.
	DepthCheck func(depth int, ctx *DepthContext) error
	// AliasCheck is called to verify the number of aliases relative to constructs.
	AliasCheck func(aliasCount, constructCount int) error

	// allowLegacyTrailingContent is an unexported flag used internally
	// to enable the legacy behavior of allowing trailing content after a document.
	//
	// [yaml.NewDecoder] and [yaml.Unmarshal] set this flag to true to enable legacy behavior for backward compatibility.
	//
	// [NewDecoder]: https://pkg.go.dev/go.yaml.in/yaml/v4#NewDecoder
	// [Unmarshal]: https://pkg.go.dev/go.yaml.in/yaml/v4#Unmarshal
	allowLegacyTrailingContent bool
}

// Option allows configuring YAML loading and dumping operations.
type Option func(*Options) error

// DepthKind represents the type of nesting (flow or block).
type DepthKind string

// [DepthKind] constants for nesting depth checks.
//
// They are the possible values for [DepthContext.Kind] when checking the nesting depth of YAML nodes.
const (
	// DepthKindFlow represents flow-style nesting (inside [] or {}).
	DepthKindFlow DepthKind = "flow"
	// DepthKindBlock represents block-style nesting (inside indentation).
	DepthKindBlock DepthKind = "block"
)

// DepthContext holds context about a nesting depth check.
type DepthContext struct {
	Kind DepthKind
}

// DefaultDepthCheck is the default depth check function.
// It returns an error when depth exceeds 10000.
func DefaultDepthCheck(depth int, ctx *DepthContext) error {
	const maxDepth = 10000
	if depth > maxDepth {
		return fmt.Errorf("exceeded max depth of %d", maxDepth)
	}
	return nil
}

// DefaultAliasCheck is the default alias check function.
// It uses a ratio-based heuristic to prevent DoS attacks via excessive aliasing.
func DefaultAliasCheck(aliasCount, constructCount int) error {
	const (
		aliasRatioRangeLow  = 400000
		aliasRatioRangeHigh = 4000000
		aliasRatioRange     = float64(aliasRatioRangeHigh - aliasRatioRangeLow)
	)
	if aliasCount <= 100 || constructCount <= 1000 {
		return nil
	}
	var allowed float64
	switch {
	case constructCount <= aliasRatioRangeLow:
		allowed = 0.99
	case constructCount >= aliasRatioRangeHigh:
		allowed = 0.10
	default:
		allowed = 0.99 - 0.89*(float64(constructCount-aliasRatioRangeLow)/aliasRatioRange)
	}
	if float64(aliasCount)/float64(constructCount) > allowed {
		return errors.New("document contains excessive aliasing")
	}
	return nil
}

// WithIndent sets the number of spaces to use for indentation when
// dumping YAML content.
//
// Valid values are 2-9. Common choices: 2 (compact), 4 (readable).
func WithIndent(indent int) Option {
	return func(o *Options) error {
		if indent < 2 || indent > 9 {
			return errors.New("yaml: indent must be between 2 and 9 spaces")
		}
		o.Indent = indent
		return nil
	}
}

// WithCompactSeqIndent configures whether the sequence indicator '- ' is
// considered part of the indentation when dumping YAML content.
//
// If compact is true, '- ' is treated as part of the indentation.
// If compact is false, '- ' is not treated as part of the indentation.
// When called without arguments, defaults to true.
func WithCompactSeqIndent(compact ...bool) Option {
	if len(compact) > 1 {
		return func(o *Options) error {
			return errors.New("yaml: WithCompactSeqIndent accepts at most one argument")
		}
	}
	val := len(compact) == 0 || compact[0]
	return func(o *Options) error {
		o.CompactSeqIndent = val
		return nil
	}
}

// WithKnownFields enables or disables strict field checking during YAML loading.
//
// When enabled, loading will return an error if the YAML input contains fields
// that do not correspond to any fields in the target struct.
// Calling WithKnownFields() enables checking; it is disabled by default.
func WithKnownFields(knownFields ...bool) Option {
	if len(knownFields) > 1 {
		return func(o *Options) error {
			return errors.New("yaml: WithKnownFields accepts at most one argument")
		}
	}
	val := len(knownFields) == 0 || knownFields[0]
	return func(o *Options) error {
		o.KnownFields = val
		return nil
	}
}

// WithSingleDocument configures the Loader to only process the first document
// in a YAML stream. After the first document is loaded, subsequent calls to
// Load will return [io.EOF].
//
// When called without arguments, defaults to true.
//
// This ignores later documents, including their syntax errors.
// Omit this option with Load to require exactly one document.
func WithSingleDocument(singleDocument ...bool) Option {
	if len(singleDocument) > 1 {
		return func(o *Options) error {
			return errors.New("yaml: WithSingleDocument accepts at most one argument")
		}
	}
	val := len(singleDocument) == 0 || singleDocument[0]
	return func(o *Options) error {
		o.SingleDocument = val
		return nil
	}
}

// WithStreamNodes enables returning stream boundary nodes when loading YAML.
//
// When enabled, Loader.Load returns an interleaved sequence of StreamNode and
// DocumentNode values:
//
//	[StreamNode, DocNode, StreamNode, DocNode, ..., StreamNode]
//
// StreamNodes contain metadata about the stream including:
//   - Encoding (UTF-8, UTF-16LE, UTF-16BE)
//   - YAML version directive (%YAML)
//   - Tag directives (%TAG)
//   - Position information (Line, Column)
//
// An empty YAML stream returns a single StreamNode.
// When called without arguments, defaults to true.
//
// The default is false.
func WithStreamNodes(enable ...bool) Option {
	if len(enable) > 1 {
		return func(o *Options) error {
			return errors.New("yaml: WithStreamNodes accepts at most one argument")
		}
	}
	val := len(enable) == 0 || enable[0]
	return func(o *Options) error {
		o.StreamNodes = val
		return nil
	}
}

// WithAllDocuments enables multi-document mode for Load and Dump operations.
//
// When used with Load, the target must be a pointer to a slice.
// All documents in the YAML stream will be decoded into the slice.
// Zero documents results in an empty slice (no error).
//
// When used with Dump, the input must be a slice.
// Each element will be encoded as a separate YAML document
// with "---" separators.
//
// When called without arguments, defaults to true.
//
// The default is false (single-document mode).
func WithAllDocuments(all ...bool) Option {
	if len(all) > 1 {
		return func(o *Options) error {
			return errors.New("yaml: WithAllDocuments accepts at most one argument")
		}
	}
	val := len(all) == 0 || all[0]
	return func(o *Options) error {
		o.AllDocuments = val
		return nil
	}
}

// WithLineWidth sets the preferred line width for YAML output.
//
// The emitter attempts to wrap long scalars without changing their value.
// Long words may exceed this width.
// Set to -1 or 0 for unlimited width.
//
// The default is 80 characters.
func WithLineWidth(width int) Option {
	return func(o *Options) error {
		if width < 0 {
			width = -1
		}
		o.LineWidth = width
		return nil
	}
}

// WithUnicode controls whether non-ASCII characters are allowed in YAML output.
//
// When true, non-ASCII characters appear as-is (e.g., "café").
// When false, non-ASCII characters are escaped (e.g., "caf\xE9").
// When called without arguments, defaults to true.
//
// The default is true.
func WithUnicode(unicode ...bool) Option {
	if len(unicode) > 1 {
		return func(o *Options) error {
			return errors.New("yaml: WithUnicode accepts at most one argument")
		}
	}
	val := len(unicode) == 0 || unicode[0]
	return func(o *Options) error {
		o.Unicode = val
		return nil
	}
}

// WithUniqueKeys enables or disables duplicate key detection during YAML loading.
//
// When enabled, loading will return an error if the YAML input contains
// duplicate keys in any mapping. This is a security feature that prevents
// key override attacks.
// When called without arguments, defaults to true.
//
// The default is true.
func WithUniqueKeys(uniqueKeys ...bool) Option {
	if len(uniqueKeys) > 1 {
		return func(o *Options) error {
			return errors.New("yaml: WithUniqueKeys accepts at most one argument")
		}
	}
	val := len(uniqueKeys) == 0 || uniqueKeys[0]
	return func(o *Options) error {
		o.UniqueKeys = val
		return nil
	}
}

// WithCanonical selects the emitter's canonical YAML layout.
//
// It emits explicit document starts, flow collections, and quoted scalars.
// Tags still depend on the node graph and desolver; this option alone does not
// force an explicit tag on every value.
// Scalar types may change when reloading canonical output.
// When called without arguments, defaults to true.
//
// The default is false.
func WithCanonical(canonical ...bool) Option {
	if len(canonical) > 1 {
		return func(o *Options) error {
			return errors.New("yaml: WithCanonical accepts at most one argument")
		}
	}
	val := len(canonical) == 0 || canonical[0]
	return func(o *Options) error {
		o.Canonical = val
		return nil
	}
}

// WithLineBreak sets the line ending style for YAML output.
//
// Available options:
//   - LineBreakLN: Unix-style \n (default)
//   - LineBreakCR: Old Mac-style \r
//   - LineBreakCRLN: Windows-style \r\n
//
// The default is LineBreakLN.
func WithLineBreak(lineBreak LineBreak) Option {
	return func(o *Options) error {
		o.LineBreak = lineBreak
		return nil
	}
}

// WithExplicitStart controls whether document start markers (---) are always emitted.
//
// When true, every document begins with an explicit "---" marker.
// When false (default), the marker is omitted for the first document.
// When called without arguments, defaults to true.
func WithExplicitStart(explicit ...bool) Option {
	if len(explicit) > 1 {
		return func(o *Options) error {
			return errors.New("yaml: WithExplicitStart accepts at most one argument")
		}
	}
	val := len(explicit) == 0 || explicit[0]
	return func(o *Options) error {
		o.ExplicitStart = val
		return nil
	}
}

// WithExplicitEnd controls whether document end markers (...) are always emitted.
//
// When true, every document ends with an explicit "..." marker.
// When false (default), the marker is omitted.
// When called without arguments, defaults to true.
func WithExplicitEnd(explicit ...bool) Option {
	if len(explicit) > 1 {
		return func(o *Options) error {
			return errors.New("yaml: WithExplicitEnd accepts at most one argument")
		}
	}
	val := len(explicit) == 0 || explicit[0]
	return func(o *Options) error {
		o.ExplicitEnd = val
		return nil
	}
}

// WithFlowSimpleCollections controls whether simple collections use flow style.
//
// When true, sequences and mappings containing only scalar values (no nested
// collections) are rendered in flow style if they fit within the line width.
// Example: {name: test, count: 42} or [a, b, c]
// When called without arguments, defaults to true.
//
// When false (default), all collections use block style.
func WithFlowSimpleCollections(flow ...bool) Option {
	if len(flow) > 1 {
		return func(o *Options) error {
			return errors.New("yaml: WithFlowSimpleCollections accepts at most one argument")
		}
	}
	val := len(flow) == 0 || flow[0]
	return func(o *Options) error {
		o.FlowSimpleCollections = val
		return nil
	}
}

// WithQuotePreference sets the preferred quote style for strings that require
// quoting.
//
// This option only affects strings that require quoting per the YAML spec.
// Plain strings that don't need quoting remain unquoted regardless of this
// setting. Quoting is required for:
//   - Strings that look like other YAML types (true, false, null, 123, etc.)
//   - Strings with leading/trailing whitespace
//   - Strings containing special YAML syntax characters
//   - Empty strings in certain contexts
//
// Quote styles:
//   - QuoteSingle: Use single quotes (v4 default)
//   - QuoteDouble: Use double quotes
//   - QuoteLegacy: Legacy v2/v3 behavior (mixed quoting)
func WithQuotePreference(style QuoteStyle) Option {
	return func(o *Options) error {
		switch style {
		case QuoteSingle, QuoteDouble, QuoteLegacy:
			o.QuotePreference = style
			return nil
		default:
			return fmt.Errorf("invalid QuoteStyle value: %d", style)
		}
	}
}

// CombineOptions combines multiple options into a single Option.
// This is useful for creating option presets or combining version defaults
// with custom options.
func CombineOptions(opts ...Option) Option {
	return func(o *Options) error {
		for _, opt := range opts {
			if err := opt(o); err != nil {
				return err
			}
		}
		return nil
	}
}

// ApplyOptions applies the given options to a new options struct.
// Starts with v4 defaults.
func ApplyOptions(opts ...Option) (*Options, error) {
	o := &Options{
		Canonical: false,
		LineBreak: LN_BREAK,

		// v4 defaults
		Indent:           2,
		CompactSeqIndent: true,
		LineWidth:        80,
		Unicode:          true,
		UniqueKeys:       true,

		// Default safety limits
		DepthCheck: DefaultDepthCheck,
		AliasCheck: DefaultAliasCheck,
	}
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

// DefaultOptions holds the default options for APIs that don't accept options.
var DefaultOptions = &Options{
	Indent:          4,
	LineWidth:       -1,
	Unicode:         true,
	UniqueKeys:      true,
	QuotePreference: QuoteLegacy,

	// Default safety limits
	DepthCheck: DefaultDepthCheck,
	AliasCheck: DefaultAliasCheck,
}

// SetLegacyAllowTrailingContent sets the internal allowLegacyTrailingContent flag to true.
//
// This function is not exported out of the module and is used in [yaml.NewDecoder] and [yaml.Unmarshal]
// to indicate that the legacy behavior of allowing trailing content
// after a document should be enabled for backward compatibility.
//
// [NewDecoder]: https://pkg.go.dev/go.yaml.in/yaml/v4#NewDecoder
// [Unmarshal]: https://pkg.go.dev/go.yaml.in/yaml/v4#Unmarshal
func SetLegacyAllowTrailingContent(o *Options) {
	o.allowLegacyTrailingContent = true
}
