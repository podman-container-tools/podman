// Copyright 2011-2019 Canonical Ltd
// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Representer stage: Converts Go values to YAML nodes.
// Handles representing from Go types to the intermediate node representation.

package libyaml

import (
	"encoding"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// keyList is a sortable slice of reflect.Values used for sorting map keys
// in a natural order (numeric, then lexicographic).
type keyList []reflect.Value

// Representer converts Go values to YAML node trees with configurable
// formatting options.
type Representer struct {
	flow                  bool
	Indent                int
	lineWidth             int
	explicitStart         bool
	explicitEnd           bool
	flowSimpleCollections bool
	quotePreference       QuoteStyle
	formatDumpError       func(*DumpError) string
}

// NewRepresenter creates a new YAML representer with the given options.
func NewRepresenter(opts *Options) *Representer {
	return &Representer{
		Indent:                opts.Indent,
		lineWidth:             opts.LineWidth,
		explicitStart:         opts.ExplicitStart,
		explicitEnd:           opts.ExplicitEnd,
		flowSimpleCollections: opts.FlowSimpleCollections,
		quotePreference:       opts.QuotePreference,
		formatDumpError:       opts.FormatDumpError,
	}
}

// Represent converts a Go value to a YAML node tree.
// This is the primary method for the Representer stage in the dump pipeline.
func (r *Representer) Represent(tag string, in reflect.Value) *Node {
	var node *Node
	if in.IsValid() {
		node, _ = in.Interface().(*Node)
	}
	if node != nil && node.Kind == DocumentNode {
		// Already a document node, return as-is
		return node
	} else {
		// Wrap the represented value in a document node
		contentNode := r.represent(tag, in)
		return &Node{
			Kind:    DocumentNode,
			Content: []*Node{contentNode},
		}
	}
}

// From http://yaml.org/type/float.html, except the regular expression there
// is bogus. In practice parsers do not enforce the "\.[0-9_]*" suffix.
var base60float = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+(?:\.[0-9_]*)?$`)

// represent is the core conversion method that handles the actual
// type-specific conversion from Go values to YAML nodes.
func (r *Representer) represent(tag string, in reflect.Value) *Node {
	tag = shortTag(tag)
	if !in.IsValid() || in.Kind() == reflect.Pointer && in.IsNil() {
		return r.nilv()
	}
	iface := in.Interface()
	switch value := iface.(type) {
	case *Node:
		return r.nodev(in)
	case Node:
		if !in.CanAddr() {
			n := reflect.New(in.Type()).Elem()
			n.Set(in)
			in = n
		}
		return r.nodev(in.Addr())
	case time.Time:
		return r.timev(tag, in)
	case *time.Time:
		return r.timev(tag, in.Elem())
	case time.Duration:
		return r.stringv(tag, reflect.ValueOf(value.String()))
	case Marshaler:
		v, err := value.MarshalYAML()
		if err != nil {
			failDump(RepresenterStage, err, r.formatDumpError)
		}
		if v == nil {
			return r.nilv()
		}
		return r.represent(tag, reflect.ValueOf(v))
	case encoding.TextMarshaler:
		text, err := value.MarshalText()
		if err != nil {
			failDump(RepresenterStage, err, r.formatDumpError)
		}
		in = reflect.ValueOf(string(text))
	case nil:
		return r.nilv()
	}
	switch in.Kind() {
	case reflect.Interface:
		return r.represent(tag, in.Elem())
	case reflect.Map:
		return r.mapv(tag, in)
	case reflect.Pointer:
		return r.represent(tag, in.Elem())
	case reflect.Struct:
		return r.structv(tag, in)
	case reflect.Slice, reflect.Array:
		return r.slicev(tag, in)
	case reflect.String:
		return r.stringv(tag, in)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return r.intv(tag, in)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return r.uintv(tag, in)
	case reflect.Float32, reflect.Float64:
		return r.floatv(tag, in)
	case reflect.Bool:
		return r.boolv(tag, in)
	default:
		failDumpf(
			RepresenterStage,
			r.formatDumpError,
			"cannot represent type: %s",
			in.Type().String(),
		)
		return nil // unreachable; failDumpf always panics
	}
}

// mapv converts a Go map to a YAML mapping node with sorted keys.
func (r *Representer) mapv(tag string, in reflect.Value) *Node {
	if tag == "" {
		tag = mapTag
	}
	var style Style
	if r.flow {
		r.flow = false
		style = FlowStyle
	}

	keys := keyList(in.MapKeys())
	sort.Sort(keys)
	content := make([]*Node, 0, len(keys)*2)
	for _, k := range keys {
		content = append(content, r.represent("", k))
		content = append(content, r.represent("", in.MapIndex(k)))
	}

	return &Node{
		Kind:    MappingNode,
		Tag:     tag,
		Content: content,
		Style:   style,
	}
}

// structv converts a Go struct to a YAML mapping node, handling field tags,
// omitempty, inline fields, and inline maps.
func (r *Representer) structv(tag string, in reflect.Value) *Node {
	sinfo, err := getStructInfo(in.Type())
	if err != nil {
		failDump(RepresenterStage, err, r.formatDumpError)
	}

	if tag == "" {
		tag = mapTag
	}
	var style Style
	if r.flow {
		r.flow = false
		style = FlowStyle
	}

	content := make([]*Node, 0)
	for _, info := range sinfo.FieldsList {
		var value reflect.Value
		if info.Inline == nil {
			value = in.Field(info.Num)
		} else {
			value = r.fieldByIndex(in, info.Inline)
			if !value.IsValid() {
				continue
			}
		}
		if info.OmitEmpty && isZero(value) {
			continue
		}
		content = append(content, r.represent("", reflect.ValueOf(info.Key)))
		r.flow = info.Flow
		content = append(content, r.represent("", value))
	}
	if sinfo.InlineMap >= 0 {
		m := in.Field(sinfo.InlineMap)
		if m.Len() > 0 {
			r.flow = false
			keys := keyList(m.MapKeys())
			sort.Sort(keys)
			for _, k := range keys {
				if _, found := sinfo.FieldsMap[k.String()]; found {
					failDumpf(
						RepresenterStage,
						r.formatDumpError,
						"cannot have key %q in inlined map: conflicts with struct field",
						k.String(),
					)
				}
				content = append(content, r.represent("", k))
				r.flow = false
				content = append(content, r.represent("", m.MapIndex(k)))
			}
		}
	}

	return &Node{
		Kind:    MappingNode,
		Tag:     tag,
		Content: content,
		Style:   style,
	}
}

// slicev converts a Go slice or array to a YAML sequence node.
func (r *Representer) slicev(tag string, in reflect.Value) *Node {
	if tag == "" {
		tag = seqTag
	}
	var style Style
	if r.flow {
		r.flow = false
		style = FlowStyle
	}

	n := in.Len()
	content := make([]*Node, n)
	for i := 0; i < n; i++ {
		content[i] = r.represent("", in.Index(i))
	}

	return &Node{
		Kind:    SequenceNode,
		Tag:     tag,
		Content: content,
		Style:   style,
	}
}

// stringv converts a Go string to a YAML scalar node, handling quoting,
// binary data (base64 encoding), and special string values.
func (r *Representer) stringv(tag string, in reflect.Value) *Node {
	var style Style
	s := in.String()
	needsQuoting := false

	switch {
	case !utf8.ValidString(s):
		if tag == binaryTag {
			failDumpf(
				RepresenterStage,
				r.formatDumpError,
				"explicitly tagged !!binary data must be base64-encoded",
			)
		}
		if tag != "" {
			failDumpf(
				RepresenterStage,
				r.formatDumpError,
				"cannot represent invalid UTF-8 data as %s",
				shortTag(tag),
			)
		}
		// It can't be represented directly as YAML so use a binary tag
		// and represent it as base64.
		tag = binaryTag
		s = encodeBase64(s)
	case tag == "":
		tag = strTag
		// Check if this string needs quoting for compatibility
		// even though it would resolve as !!str
		needsQuoting = isBase60Float(s) || isOldBool(s) || looksLikeMerge(s)
	}

	// Set the style based on content
	switch {
	case strings.Contains(s, "\n"):
		if r.flow || !shouldUseLiteralStyle(s) {
			style = DoubleQuotedStyle
		} else {
			style = LiteralStyle
		}
	case needsQuoting:
		// Force quoting for YAML 1.1 compatibility values
		style = SingleQuotedStyle
		// and honor the configured quote preference.
		if r.quotePreference.ScalarStyle() == DOUBLE_QUOTED_SCALAR_STYLE {
			style = DoubleQuotedStyle
		}
	default:
		// Plain style by default - Desolver will add quotes if type mismatch
		style = 0
	}

	return &Node{
		Kind:  ScalarNode,
		Tag:   tag,
		Value: s,
		Style: style,
	}
}

// boolv converts a Go bool to a YAML scalar node.
func (r *Representer) boolv(tag string, in reflect.Value) *Node {
	var s string
	if in.Bool() {
		s = "true"
	} else {
		s = "false"
	}
	if tag == "" {
		tag = boolTag
	}
	return &Node{
		Kind:  ScalarNode,
		Tag:   tag,
		Value: s,
	}
}

// intv converts a Go signed integer to a YAML scalar node.
func (r *Representer) intv(tag string, in reflect.Value) *Node {
	s := strconv.FormatInt(in.Int(), 10)
	if tag == "" {
		tag = intTag
	}
	return &Node{
		Kind:  ScalarNode,
		Tag:   tag,
		Value: s,
	}
}

// uintv converts a Go unsigned integer to a YAML scalar node.
func (r *Representer) uintv(tag string, in reflect.Value) *Node {
	s := strconv.FormatUint(in.Uint(), 10)
	if tag == "" {
		tag = intTag
	}
	return &Node{
		Kind:  ScalarNode,
		Tag:   tag,
		Value: s,
	}
}

// timev converts a Go [time.Time] to a YAML scalar node in RFC3339Nano format.
func (r *Representer) timev(tag string, in reflect.Value) *Node {
	t := in.Interface().(time.Time)
	s := t.Format(time.RFC3339Nano)
	if tag == "" {
		tag = timestampTag
	}
	return &Node{
		Kind:  ScalarNode,
		Tag:   tag,
		Value: s,
	}
}

// floatv converts a Go float to a YAML scalar node, handling special values
// like infinity and NaN.
func (r *Representer) floatv(tag string, in reflect.Value) *Node {
	// Issue #352: When formatting, use the precision of the underlying value
	precision := 64
	if in.Kind() == reflect.Float32 {
		precision = 32
	}

	s := strconv.FormatFloat(in.Float(), 'g', -1, precision)
	switch s {
	case "+Inf":
		s = ".inf"
	case "-Inf":
		s = "-.inf"
	case "NaN":
		s = ".nan"
	}
	if tag == "" {
		tag = floatTag
	}
	return &Node{
		Kind:  ScalarNode,
		Tag:   tag,
		Value: s,
	}
}

// nilv creates a YAML null node.
func (r *Representer) nilv() *Node {
	return &Node{
		Kind:  ScalarNode,
		Tag:   nullTag,
		Value: "null",
	}
}

// nodev returns a node value as-is without conversion.
func (r *Representer) nodev(in reflect.Value) *Node {
	// Return the node as-is - no conversion needed
	return in.Interface().(*Node)
}

// Len returns the number of keys in the list.
func (l keyList) Len() int { return len(l) }

// Swap exchanges the positions of two keys in the list.
func (l keyList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

// Less implements a natural sort order for map keys: numeric values sort
// numerically, strings sort with natural number ordering, and mixed types
// sort by kind.
func (l keyList) Less(i, j int) bool {
	a := l[i]
	b := l[j]
	ak := a.Kind()
	bk := b.Kind()
	for (ak == reflect.Interface || ak == reflect.Pointer) && !a.IsNil() {
		a = a.Elem()
		ak = a.Kind()
	}
	for (bk == reflect.Interface || bk == reflect.Pointer) && !b.IsNil() {
		b = b.Elem()
		bk = b.Kind()
	}
	af, aok := keyFloat(a)
	bf, bok := keyFloat(b)
	if aok && bok {
		if af != bf {
			return af < bf
		}
		if ak != bk {
			return ak < bk
		}
		return numLess(a, b)
	}
	if ak != reflect.String || bk != reflect.String {
		return ak < bk
	}
	ar, br := []rune(a.String()), []rune(b.String())
	digits := false
	for i := 0; i < len(ar) && i < len(br); i++ {
		if ar[i] == br[i] {
			digits = unicode.IsDigit(ar[i])
			continue
		}
		al := unicode.IsLetter(ar[i])
		bl := unicode.IsLetter(br[i])
		if al && bl {
			return ar[i] < br[i]
		}
		if al || bl {
			if digits {
				return al
			} else {
				return bl
			}
		}
		var ai, bi int
		var an, bn int64
		if ar[i] == '0' || br[i] == '0' {
			for j := i - 1; j >= 0 && unicode.IsDigit(ar[j]); j-- {
				if ar[j] != '0' {
					an = 1
					bn = 1
					break
				}
			}
		}
		for ai = i; ai < len(ar) && unicode.IsDigit(ar[ai]); ai++ {
			an = an*10 + int64(ar[ai]-'0')
		}
		for bi = i; bi < len(br) && unicode.IsDigit(br[bi]); bi++ {
			bn = bn*10 + int64(br[bi]-'0')
		}
		if an != bn {
			return an < bn
		}
		if ai != bi {
			return ai < bi
		}
		return ar[i] < br[i]
	}
	return len(ar) < len(br)
}

// keyFloat returns a float value for v if it is a number/bool
// and whether it is a number/bool or not.
func keyFloat(v reflect.Value) (f float64, ok bool) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(v.Uint()), true
	case reflect.Bool:
		if v.Bool() {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// numLess returns whether a < b.
// a and b must necessarily have the same kind.
func numLess(a, b reflect.Value) bool {
	switch a.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() < b.Int()
	case reflect.Float32, reflect.Float64:
		return a.Float() < b.Float()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return a.Uint() < b.Uint()
	case reflect.Bool:
		return !a.Bool() && b.Bool()
	}
	panic("not a number")
}

// fieldByIndex navigates through struct fields using the given index path,
// dereferencing pointers as needed.
func (r *Representer) fieldByIndex(v reflect.Value, index []int) (field reflect.Value) {
	for _, num := range index {
		for {
			if v.Kind() == reflect.Pointer {
				if v.IsNil() {
					return reflect.Value{}
				}
				v = v.Elem()
				continue
			}
			break
		}
		v = v.Field(num)
	}
	return v
}

// isBase60 returns whether s is in base 60 notation as defined in YAML 1.1.
//
// The base 60 float notation in YAML 1.1 is a terrible idea and is unsupported
// in YAML 1.2 and by this package, but these should be represented quoted for
// the time being for compatibility with other parsers.
func isBase60Float(s string) (result bool) {
	// Fast path.
	if s == "" {
		return false
	}
	c := s[0]
	if !(c == '+' || c == '-' || c >= '0' && c <= '9') || strings.IndexByte(s, ':') < 0 {
		return false
	}
	// Do the full match.
	return base60float.MatchString(s)
}

// isOldBool returns whether s is bool notation as defined in YAML 1.1.
//
// We continue to force strings that YAML 1.1 would interpret as booleans to be
// rendered as quotes strings so that the represented output valid for YAML 1.1
// parsing.
func isOldBool(s string) (result bool) {
	switch s {
	case "y", "Y", "yes", "Yes", "YES", "on", "On", "ON",
		"n", "N", "no", "No", "NO", "off", "Off", "OFF":
		return true
	default:
		return false
	}
}

// looksLikeMerge returns true if the given string is the merge indicator "<<".
//
// When encoding a scalar with this exact value, it must be quoted to prevent it
// from being interpreted as a merge indicator during decoding.
func looksLikeMerge(s string) (result bool) {
	return s == "<<"
}
