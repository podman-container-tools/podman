// Copyright 2011-2019 Canonical Ltd
// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Constructor stage: Converts YAML nodes to Go values.
// Handles type resolution, custom unmarshalers, and struct field mapping.

package libyaml

import (
	"encoding"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"reflect"
	"time"
)

// --------------------------------------------------------------------------
// Types and Interfaces

// legacyConstructor is the old-style unmarshaler interface.
// It's kept for backwards compatibility.
type legacyConstructor interface {
	UnmarshalYAML(construct func(any) error) error
}

// constructorAdapter is an interface that wraps the root package's Unmarshaler
// interface.
// This allows the constructor to call constructors that expect *yaml.Node
// instead of *libyaml.Node.
type constructorAdapter interface {
	CallRootConstructor(n *Node) error
}

// ScalarConstructFunc is the signature for tag-specific scalar constructor
// functions.
// Each function handles construction of a specific YAML tag to various Go
// types.
type ScalarConstructFunc func(c *Constructor, n *Node, resolved any, out reflect.Value) bool

// Constructor state
type Constructor struct {
	doc        *Node
	aliases    map[*Node]bool
	TypeErrors []*LoadError

	stringMapType  reflect.Type
	generalMapType reflect.Type

	KnownFields    bool
	UniqueKeys     bool
	constructCount int
	aliasCount     int
	aliasDepth     int
	aliasCheck     func(aliasCount, constructCount int) error
	formatError    func(*LoadError) string

	mergePlugin  MergeKeyPlugin
	mergeConfig  MergeKeyConfig
	mergeAliases map[*Node]*Node
}

// NewConstructor creates a new Constructor initialized with the provided
// options.
func NewConstructor(opts *Options) *Constructor {
	plugin := opts.MergeKey
	if plugin == nil {
		plugin = DefaultMergeKey{}
	}
	return &Constructor{
		stringMapType:  stringMapType,
		generalMapType: generalMapType,
		KnownFields:    opts.KnownFields,
		UniqueKeys:     opts.UniqueKeys,
		aliases:        make(map[*Node]bool),
		aliasCheck:     opts.AliasCheck,
		formatError:    opts.FormatLoadError,
		mergePlugin:    plugin,
		mergeConfig:    plugin.MergeKeyConfig(),
	}
}

// --------------------------------------------------------------------------
// Main Entry Point

// Construct converts a YAML node into the Go value represented by out.
// It dispatches to the appropriate handler based on the node kind and
// handles alias expansion, custom unmarshalers, and type resolution.
// Returns true if the construction was successful.
func (c *Constructor) Construct(n *Node, out reflect.Value) (good bool) {
	if c.mergeAliases[n] != nil {
		c.aliasDepth++
		defer func() { c.aliasDepth-- }()
	}
	c.checkConstruction(n)
	if out.Type() == nodeType {
		out.Set(reflect.ValueOf(n).Elem())
		return true
	}

	switch n.Kind {
	case DocumentNode:
		return c.document(n, out)
	case AliasNode:
		return c.alias(n, out)
	}

	out, constructed, good := c.prepare(n, out)
	if constructed {
		return good
	}

	// When out type implements [encoding.TextUnmarshaler], ensure the node
	// is a scalar. Otherwise, for example, constructing a YAML mapping
	// into a struct having no exported fields, but implementing
	// TextUnmarshaler would silently succeed, but do nothing.
	//
	// Note that this matches the behavior of both encoding/json and
	// encoding/json/v2.
	if n.Kind != ScalarNode && isTextUnmarshaler(out) {
		err := fmt.Errorf("cannot construct %s into %s (TextUnmarshaler)", shortTag(n.Tag), out.Type())
		c.TypeErrors = append(c.TypeErrors, c.formatConstructorError(err, Mark{Line: n.Line, Column: n.Column}))
		return false
	}

	switch n.Kind {
	case ScalarNode:
		good = c.scalar(n, out)
	case MappingNode:
		good = c.mapping(n, out)
	case SequenceNode:
		good = c.sequence(n, out)
	case 0:
		if n.IsZero() {
			return c.null(out)
		}
		fallthrough
	default:
		Fail(c.formatConstructorError(
			fmt.Errorf("cannot construct node with unknown kind: '%d'", n.Kind),
			Mark{Line: n.Line, Column: n.Column},
		))
	}
	return good
}

func (c *Constructor) checkConstruction(n *Node) {
	c.constructCount++
	if c.aliasDepth > 0 {
		c.aliasCount++
	}
	if c.aliasCheck != nil {
		if err := c.aliasCheck(c.aliasCount, c.constructCount); err != nil {
			Fail(c.formatConstructorError(err, Mark{Line: n.Line, Column: n.Column}))
		}
	}
}

// --------------------------------------------------------------------------
// Package-level Variables and Constants
var (
	nodeType       = reflect.TypeOf(Node{})
	durationType   = reflect.TypeOf(time.Duration(0))
	stringMapType  = reflect.TypeOf(map[string]any{})
	generalMapType = reflect.TypeOf(map[any]any{})
	ifaceType      = generalMapType.Elem()
)

// scalarConstructors maps YAML scalar tags to their constructor functions.
var scalarConstructors = map[string]ScalarConstructFunc{
	strTag:       (*Constructor).constructStr,
	intTag:       (*Constructor).constructInt,
	boolTag:      (*Constructor).constructBool,
	floatTag:     (*Constructor).constructFloat,
	nullTag:      (*Constructor).constructNull,
	timestampTag: (*Constructor).constructTimestamp,
	binaryTag:    (*Constructor).constructBinary,
	mergeTag:     (*Constructor).constructMerge,
}

// --------------------------------------------------------------------------
// Scalar tag constructors

// constructStr constructs a !!str tagged value into various Go types.
func (c *Constructor) constructStr(n *Node, resolved any, out reflect.Value) bool {
	switch out.Kind() {
	case reflect.String:
		out.SetString(n.Value)
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Handle time.Duration parsing from strings like "3s", "1m",
		// etc.
		if out.Type() == durationType {
			d, err := time.ParseDuration(n.Value)
			if err == nil {
				out.SetInt(int64(d))
				return true
			}
		}
	case reflect.Bool:
		// YAML 1.1 compatibility: allow string values like "y", "on",
		// "Off" as bools
		switch n.Value {
		case "y", "Y", "yes", "Yes", "YES", "on", "On", "ON":
			out.SetBool(true)
			return true
		case "n", "N", "no", "No", "NO", "off", "Off", "OFF":
			out.SetBool(false)
			return true
		}
	case reflect.Interface:
		out.Set(reflect.ValueOf(resolved))
		return true
	}
	c.tagError(n, strTag, out)
	return false
}

// constructInt constructs a !!int tagged value into various Go types.
func (c *Constructor) constructInt(n *Node, resolved any, out reflect.Value) bool {
	switch out.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		isDuration := out.Type() == durationType

		switch resolved := resolved.(type) {
		case int:
			if !isDuration && !out.OverflowInt(int64(resolved)) {
				out.SetInt(int64(resolved))
				return true
			} else if isDuration && resolved == 0 {
				out.SetInt(0)
				return true
			}
		case int64:
			if !isDuration && !out.OverflowInt(resolved) {
				out.SetInt(resolved)
				return true
			}
		case uint64:
			if !isDuration && resolved <= math.MaxInt64 {
				intVal := int64(resolved)
				if !out.OverflowInt(intVal) {
					out.SetInt(intVal)
					return true
				}
			}
		case float64:
			if !isDuration && resolved >= math.MinInt64 && resolved <= math.MaxInt64 {
				intVal := int64(resolved)
				// Verify conversion is lossless (handles
				// floating-point precision)
				if float64(intVal) == resolved && !out.OverflowInt(intVal) {
					out.SetInt(intVal)
					return true
				}
			}
		case string:
			if out.Type() == durationType {
				d, err := time.ParseDuration(resolved)
				if err == nil {
					out.SetInt(int64(d))
					return true
				}
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		switch resolved := resolved.(type) {
		case int:
			if resolved >= 0 && !out.OverflowUint(uint64(resolved)) {
				out.SetUint(uint64(resolved))
				return true
			}
		case int64:
			if resolved >= 0 && !out.OverflowUint(uint64(resolved)) {
				out.SetUint(uint64(resolved))
				return true
			}
		case uint64:
			if !out.OverflowUint(resolved) {
				out.SetUint(resolved)
				return true
			}
		case float64:
			if resolved >= 0 && resolved <= math.MaxUint64 {
				uintVal := uint64(resolved)
				// Verify conversion is lossless (handles
				// floating-point precision)
				if float64(uintVal) == resolved && !out.OverflowUint(uintVal) {
					out.SetUint(uintVal)
					return true
				}
			}
		}
	case reflect.Float32, reflect.Float64:
		// Allow int to float conversion
		switch resolved := resolved.(type) {
		case int:
			out.SetFloat(float64(resolved))
			return true
		case int64:
			out.SetFloat(float64(resolved))
			return true
		case uint64:
			out.SetFloat(float64(resolved))
			return true
		}
	case reflect.String:
		// Allow int to string conversion
		out.SetString(n.Value)
		return true
	case reflect.Interface:
		out.Set(reflect.ValueOf(resolved))
		return true
	}
	c.tagError(n, intTag, out)
	return false
}

// constructBool constructs a !!bool tagged value into various Go types.
func (c *Constructor) constructBool(n *Node, resolved any, out reflect.Value) bool {
	switch out.Kind() {
	case reflect.Bool:
		switch resolved := resolved.(type) {
		case bool:
			out.SetBool(resolved)
			return true
		case string:
			// This offers some compatibility with the 1.1 spec
			// (https://yaml.org/type/bool.html).
			// It only works if explicitly attempting to construct
			// into a typed bool value.
			switch resolved {
			case "y", "Y", "yes", "Yes", "YES", "on", "On", "ON":
				out.SetBool(true)
				return true
			case "n", "N", "no", "No", "NO", "off", "Off", "OFF":
				out.SetBool(false)
				return true
			}
		}
	case reflect.String:
		// Allow bool to be constructed as string (e.g., true -> "true")
		out.SetString(n.Value)
		return true
	case reflect.Interface:
		out.Set(reflect.ValueOf(resolved))
		return true
	}
	c.tagError(n, boolTag, out)
	return false
}

// constructFloat constructs a !!float tagged value into various Go types.
func (c *Constructor) constructFloat(n *Node, resolved any, out reflect.Value) bool {
	switch out.Kind() {
	case reflect.Float32, reflect.Float64:
		switch resolved := resolved.(type) {
		case int:
			out.SetFloat(float64(resolved))
			return true
		case int64:
			out.SetFloat(float64(resolved))
			return true
		case uint64:
			out.SetFloat(float64(resolved))
			return true
		case float64:
			out.SetFloat(resolved)
			return true
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Allow float to int conversion if lossless
		isDuration := out.Type() == durationType
		if fval, ok := resolved.(float64); ok && !isDuration {
			if fval >= math.MinInt64 && fval <= math.MaxInt64 {
				intVal := int64(fval)
				if float64(intVal) == fval && !out.OverflowInt(intVal) {
					out.SetInt(intVal)
					return true
				}
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		// Allow float to uint conversion if lossless
		if fval, ok := resolved.(float64); ok {
			if fval >= 0 && fval <= math.MaxUint64 {
				uintVal := uint64(fval)
				if float64(uintVal) == fval && !out.OverflowUint(uintVal) {
					out.SetUint(uintVal)
					return true
				}
			}
		}
	case reflect.String:
		out.SetString(n.Value)
		return true
	case reflect.Interface:
		out.Set(reflect.ValueOf(resolved))
		return true
	}
	c.tagError(n, floatTag, out)
	return false
}

// constructTimestamp constructs a !!timestamp tagged value into various Go
// types.
func (c *Constructor) constructTimestamp(n *Node, resolved any, out reflect.Value) bool {
	switch out.Kind() {
	case reflect.Struct:
		if resolvedv := reflect.ValueOf(resolved); out.Type() == resolvedv.Type() {
			out.Set(resolvedv)
			return true
		}
	case reflect.String:
		// Allow timestamp values to be constructed as string.
		out.SetString(n.Value)
		return true
	case reflect.Interface:
		out.Set(reflect.ValueOf(resolved))
		return true
	}
	c.tagError(n, timestampTag, out)
	return false
}

// constructBinary constructs a !!binary tagged value into various Go types.
func (c *Constructor) constructBinary(n *Node, resolved any, out reflect.Value) bool {
	switch out.Kind() {
	case reflect.String:
		out.SetString(resolved.(string))
		return true
	case reflect.Slice:
		// allow decoding !!binary-tagged value into []byte specifically
		if out.Type().Elem().Kind() == reflect.Uint8 {
			out.SetBytes([]byte(resolved.(string)))
			return true
		}
	case reflect.Interface:
		out.Set(reflect.ValueOf(resolved))
		return true
	}
	c.tagError(n, binaryTag, out)
	return false
}

// constructNull constructs a !!null tagged value into various Go types.
func (c *Constructor) constructNull(n *Node, resolved any, out reflect.Value) bool {
	return c.null(out)
}

// constructMerge handles !!merge tagged keys.
// Merge keys are directives, not values, so construction always fails.
// They are handled specially by the mapping() function.
func (c *Constructor) constructMerge(n *Node, resolved any, out reflect.Value) bool {
	return false
}

// --------------------------------------------------------------------------
// Node Kind Handlers

// document constructs a DocumentNode by processing its single content node.
func (c *Constructor) document(n *Node, out reflect.Value) (good bool) {
	c.mergeAliases = nil
	if len(n.Content) == 1 {
		c.doc = n
		c.Construct(n.Content[0], out)
		return true
	}
	return false
}

// alias constructs an AliasNode by following the alias reference and
// tracking alias depth to detect circular references.
func (c *Constructor) alias(n *Node, out reflect.Value) (good bool) {
	identity := c.originalMergeNode(n)
	if c.aliases[identity] {
		// TODO this could actually be allowed in some circumstances.
		Fail(&LoadError{
			Stage:     ComposerStage,
			Mark:      Mark{Line: n.Line, Column: n.Column},
			Message:   fmt.Sprintf("anchor '%s' value contains itself", n.Value),
			formatter: c.formatError,
		})
	}
	c.aliases[identity] = true
	c.aliasDepth++
	good = c.Construct(n.Alias, out)
	c.aliasDepth--
	delete(c.aliases, identity)
	return good
}

// scalar constructs a ScalarNode by resolving its tag and value, then
// dispatching to the appropriate tag-specific constructor or using
// TextUnmarshaler if available.
func (c *Constructor) scalar(n *Node, out reflect.Value) bool {
	if c.mergeConfig.Disable && isMerge(n) {
		if n.Style&TaggedStyle != 0 {
			c.mergeFailure(errors.New("merge-key: merge tag is disabled"), n)
		}
		copyNode := *n
		copyNode.Tag = strTag
		n = &copyNode
	}
	// Resolve the tag and value
	var tag string
	var resolved any
	if n.indicatedString() {
		tag = strTag
		resolved = n.Value
	} else {
		tag, resolved = resolve(n.Tag, n.Value, c.formatError)
		if tag == binaryTag {
			data, err := base64.StdEncoding.DecodeString(resolved.(string))
			if err != nil {
				Fail(c.formatConstructorError(
					errors.New("!!binary value contains invalid base64 data"),
					Mark{Line: n.Line, Column: n.Column},
				))
			}
			resolved = string(data)
		}
	}

	// Handle null
	if resolved == nil {
		return c.null(out)
	}

	// Fast path: exact type match
	if resolvedv := reflect.ValueOf(resolved); out.Type() == resolvedv.Type() {
		out.Set(resolvedv)
		return true
	}

	// Handle TextUnmarshaler interface
	if out.CanAddr() {
		u, ok := out.Addr().Interface().(encoding.TextUnmarshaler)
		if ok {
			var text []byte
			if tag == binaryTag {
				text = []byte(resolved.(string))
			} else {
				text = []byte(n.Value)
			}
			err := u.UnmarshalText(text)
			if err != nil {
				c.TypeErrors = append(c.TypeErrors, c.formatConstructorError(err, Mark{Line: n.Line, Column: n.Column}))
				return false
			}
			return true
		}
	}

	// Dispatch to tag-specific constructor
	if constructor, ok := scalarConstructors[tag]; ok {
		return constructor(c, n, resolved, out)
	}

	// Unknown tag - try some fallback behaviors
	switch out.Kind() {
	case reflect.Interface:
		// For interface{} targets, accept any resolved value
		out.Set(reflect.ValueOf(resolved))
		return true
	case reflect.Struct:
		// For struct targets with matching types, try direct assignment
		if resolvedv := reflect.ValueOf(resolved); out.Type() == resolvedv.Type() {
			out.Set(resolvedv)
			return true
		}
	}

	// No constructor and no fallback worked
	c.tagError(n, tag, out)
	return false
}

// sequence constructs a SequenceNode into a Go slice, array, or interface.
func (c *Constructor) sequence(n *Node, out reflect.Value) (good bool) {
	l := len(n.Content)

	var iface reflect.Value
	switch out.Kind() {
	case reflect.Slice:
		out.Set(reflect.MakeSlice(out.Type(), l, l))
	case reflect.Array:
		if l != out.Len() {
			Fail(c.formatConstructorError(
				fmt.Errorf("invalid array: want %d elements but got %d", out.Len(), l),
				Mark{Line: n.Line, Column: n.Column},
			))
		}
	case reflect.Interface:
		// No type hints. Will have to use a generic sequence.
		iface = out
		out = settableValueOf(make([]any, l))
	default:
		c.tagError(n, seqTag, out)
		return false
	}
	et := out.Type().Elem()

	j := 0
	for i := 0; i < l; i++ {
		e := reflect.New(et).Elem()
		if ok := c.Construct(n.Content[i], e); ok {
			out.Index(j).Set(e)
			j++
		}
	}
	if out.Kind() != reflect.Array {
		out.Set(out.Slice(0, j))
	}
	if iface.IsValid() {
		iface.Set(out)
	}
	return true
}

// mapping constructs a MappingNode into a Go map, struct, or interface.
// It handles key uniqueness checking, merge keys, and type-appropriate
// map construction (string-keyed vs general).
func (c *Constructor) mapping(n *Node, out reflect.Value) (good bool) {
	stringMapping := isStringMap(n)
	keyType := ifaceType
	switch out.Kind() {
	case reflect.Map:
		keyType = out.Type().Key()
	case reflect.Struct:
		keyType = reflect.TypeOf("")
	case reflect.Interface:
		if stringMapping {
			keyType = c.stringMapType.Key()
		}
	}
	original := n
	n = c.effectiveMapping(n, keyType)
	if n == nil {
		return false
	}
	l := len(n.Content)
	switch out.Kind() {
	case reflect.Struct:
		return c.mappingStruct(n, original, out)
	case reflect.Map:
		// okay
	case reflect.Interface:
		iface := out
		if stringMapping {
			out = reflect.MakeMap(c.stringMapType)
		} else {
			out = reflect.MakeMap(c.generalMapType)
		}
		iface.Set(out)
	default:
		c.tagError(n, mapTag, out)
		return false
	}

	outt := out.Type()
	kt := outt.Key()
	et := outt.Elem()

	stringMapType := c.stringMapType
	generalMapType := c.generalMapType
	if outt.Elem() == ifaceType {
		if outt.Key().Kind() == reflect.String {
			c.stringMapType = outt
		} else if outt.Key() == ifaceType {
			c.generalMapType = outt
		}
	}

	mapIsNew := false
	if out.IsNil() {
		out.Set(reflect.MakeMap(outt))
		mapIsNew = true
	}
	for i := 0; i < l; i += 2 {
		k := reflect.New(kt).Elem()
		if c.Construct(n.Content[i], k) {
			kkind := k.Kind()
			if kkind == reflect.Interface {
				kkind = k.Elem().Kind()
			}
			if kkind == reflect.Map || kkind == reflect.Slice {
				c.failUnhashableKey(k.Interface(), n.Content[i])
			}
			e := reflect.New(et).Elem()
			if c.Construct(n.Content[i+1], e) || n.Content[i+1].ShortTag() == nullTag && (mapIsNew || !out.MapIndex(k).IsValid()) {
				out.SetMapIndex(k, e)
			}
		}
	}

	c.stringMapType = stringMapType
	c.generalMapType = generalMapType
	return true
}

// --------------------------------------------------------------------------
// Mapping/Struct Support

// mappingStruct constructs a MappingNode into a struct value.
// It handles field matching by name, inline fields, inline maps, merge keys,
// and enforces known fields and unique keys when configured.
func (c *Constructor) mappingStruct(n, original *Node, out reflect.Value) (good bool) {
	sinfo, err := getStructInfo(out.Type())
	if err != nil {
		panic(err)
	}

	var inlineMap reflect.Value
	var elemType reflect.Type
	if sinfo.InlineMap != -1 {
		inlineMap = out.Field(sinfo.InlineMap)
		elemType = inlineMap.Type().Elem()
	}

	for _, index := range sinfo.InlineConstructors {
		field := c.fieldByIndex(original, out, index)
		c.prepare(original, field)
	}

	var doneFields []bool
	if c.UniqueKeys {
		doneFields = make([]bool, len(sinfo.FieldsList))
	}
	name := settableValueOf("")
	l := len(n.Content)
	for i := 0; i < l; i += 2 {
		ni := n.Content[i]
		if !c.Construct(ni, name) {
			continue
		}
		sname := name.String()
		if info, ok := sinfo.FieldsMap[sname]; ok {
			if c.UniqueKeys {
				if doneFields[info.Id] {
					c.TypeErrors = append(c.TypeErrors, c.formatConstructorError(
						fmt.Errorf("field %s already set in type %s", name.String(), out.Type()),
						Mark{Line: ni.Line, Column: ni.Column},
					))
					continue
				}
				doneFields[info.Id] = true
			}
			var field reflect.Value
			if info.Inline == nil {
				field = out.Field(info.Num)
			} else {
				field = c.fieldByIndex(n, out, info.Inline)
			}
			c.Construct(n.Content[i+1], field)
		} else if sinfo.InlineMap != -1 {
			if inlineMap.IsNil() {
				inlineMap.Set(reflect.MakeMap(inlineMap.Type()))
			}
			value := reflect.New(elemType).Elem()
			c.Construct(n.Content[i+1], value)
			inlineMap.SetMapIndex(name, value)
		} else if c.KnownFields {
			c.TypeErrors = append(c.TypeErrors, c.formatConstructorError(
				fmt.Errorf("field %s not found in type %s", name.String(), out.Type()),
				Mark{Line: ni.Line, Column: ni.Column},
			))
		}
	}

	return true
}

// isStringMap checks if a MappingNode has only string or merge keys.
// This determines whether to use map[string]any or map[any]any when
// constructing into an interface{}.
func isStringMap(n *Node) bool {
	if n.Kind != MappingNode {
		return false
	}
	l := len(n.Content)
	for i := 0; i < l; i += 2 {
		shortTag := n.Content[i].ShortTag()
		if shortTag != strTag && shortTag != mergeTag {
			return false
		}
	}
	return true
}

// isMerge recognizes resolved merge tags and unresolved plain << keys.
func isMerge(n *Node) bool {
	return n.Kind == ScalarNode && (shortTag(n.Tag) == mergeTag ||
		n.Tag == "" && n.Value == "<<" &&
			n.Style&(SingleQuotedStyle|DoubleQuotedStyle|LiteralStyle|FoldedStyle) == 0)
}

// failWantMap panics with a constructor error for invalid merge key values.
func (c *Constructor) failWantMap(n *Node) {
	err := errors.New("map merge requires map or sequence of maps as the value")
	Fail(c.formatConstructorError(err, Mark{Line: n.Line, Column: n.Column}))
}

// failUnhashableKey panics with an error message for keys that cannot be used
// as Go map keys.
func (c *Constructor) failUnhashableKey(key any, n *Node) {
	Fail(c.formatConstructorError(
		fmt.Errorf("cannot use '%#v' as a map key; try decoding into yaml.Node", key),
		Mark{Line: n.Line, Column: n.Column},
	))
}

// --------------------------------------------------------------------------
// Utility Methods

// prepare initializes and dereferences pointers and calls UnmarshalYAML
// if a value is found to implement it.
// It returns the initialized and dereferenced out value, whether
// construction was already done by UnmarshalYAML, and if so whether
// its types constructed appropriately.
//
// If n holds a null value, prepare returns before doing anything.
func (c *Constructor) prepare(n *Node, out reflect.Value) (newout reflect.Value, constructed, good bool) {
	if n.ShortTag() == nullTag {
		return out, false, false
	}
	again := true
	for again {
		again = false
		if out.Kind() == reflect.Pointer {
			if out.IsNil() {
				out.Set(reflect.New(out.Type().Elem()))
			}
			out = out.Elem()
			again = true
		}
		if out.CanAddr() {
			outi := out.Addr().Interface()
			// Check for libyaml.constructor
			if u, ok := outi.(constructor); ok {
				good = c.callConstructor(n, u)
				return out, true, good
			}
			if u, ok := outi.(legacyConstructor); ok {
				good = c.callLegacyConstructor(n, u)
				return out, true, good
			}
			if nodeType := v3NodeHook(out.Addr().Type()); nodeType != nil {
				err := fmt.Errorf(
					"%s implements UnmarshalYAML(*%s.Node); "+
						"v4 cannot invoke v3 Node hooks by default; "+
						"migrate to *go.yaml.in/yaml/v4.Node or use "+
						"UnmarshalYAML(func(any) error) error",
					out.Addr().Type(), nodeType.Elem().PkgPath())
				c.TypeErrors = append(c.TypeErrors, c.formatConstructorError(
					err, Mark{Line: n.Line, Column: n.Column}))
				return out, true, false
			}
		}
	}
	return out, false, false
}

// fieldByIndex returns the struct field at the given index path, initializing
// any nil pointers along the way.
func (c *Constructor) fieldByIndex(n *Node, v reflect.Value, index []int) (field reflect.Value) {
	if n.ShortTag() == nullTag {
		return reflect.Value{}
	}
	for _, num := range index {
		for {
			if v.Kind() == reflect.Pointer {
				if v.IsNil() {
					v.Set(reflect.New(v.Type().Elem()))
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

// callConstructor invokes the UnmarshalYAML method on a value implementing
// the constructor interface, handling errors appropriately.
func (c *Constructor) callConstructor(n *Node, u constructor) (good bool) {
	err := u.UnmarshalYAML(n)
	switch e := err.(type) {
	case nil:
		return true
	case *LoadErrors:
		c.TypeErrors = append(c.TypeErrors, e.Errors...)
		return false
	default:
		c.TypeErrors = append(c.TypeErrors, c.formatConstructorError(
			err,
			Mark{Line: n.Line, Column: n.Column},
		))
		return false
	}
}

// callLegacyConstructor invokes the UnmarshalYAML method on a value
// implementing the old-style legacyConstructor interface.
func (c *Constructor) callLegacyConstructor(n *Node, u legacyConstructor) (good bool) {
	terrlen := len(c.TypeErrors)
	err := u.UnmarshalYAML(func(v any) (err error) {
		defer handleErr(&err)
		c.Construct(n, reflect.ValueOf(v))
		if len(c.TypeErrors) > terrlen {
			issues := append([]*LoadError{}, c.TypeErrors[terrlen:]...)
			c.TypeErrors = c.TypeErrors[:terrlen]
			return &LoadErrors{issues}
		}
		return nil
	})
	switch e := err.(type) {
	case nil:
		return true
	case *LoadErrors:
		c.TypeErrors = append(c.TypeErrors, e.Errors...)
		return false
	default:
		c.TypeErrors = append(c.TypeErrors, c.formatConstructorError(
			err,
			Mark{Line: n.Line, Column: n.Column},
		))
		return false
	}
}

// tagError records a type construction error indicating that a node with a
// given tag cannot be constructed into the target type.
func (c *Constructor) tagError(n *Node, tag string, out reflect.Value) {
	if n.Tag != "" {
		tag = n.Tag
	}
	value := n.Value
	if tag != seqTag && tag != mapTag {
		if len(value) > 10 {
			value = " `" + value[:7] + "...`"
		} else {
			value = " `" + value + "`"
		}
	}
	c.TypeErrors = append(c.TypeErrors, c.formatConstructorError(
		fmt.Errorf("cannot construct %s%s into %s", shortTag(tag), value, out.Type()),
		Mark{Line: n.Line, Column: n.Column},
	))
}

// null constructs a null value by setting the target to its zero value.
// Only works for nillable types (interface, pointer, map, slice).
func (c *Constructor) null(out reflect.Value) bool {
	if out.CanAddr() {
		switch out.Kind() {
		case reflect.Interface, reflect.Pointer, reflect.Map, reflect.Slice:
			out.Set(reflect.Zero(out.Type()))
			return true
		}
	}
	return false
}

// isTextUnmarshaler checks if a value implements [encoding.TextUnmarshaler].
// It dereferences pointers to check the underlying type.
func isTextUnmarshaler(out reflect.Value) bool {
	// Dereference pointers to check the underlying type,
	// similar to how prepare() handles Constructor checks.
	for out.Kind() == reflect.Pointer {
		if out.IsNil() {
			// Create a new instance to check the type
			out = reflect.New(out.Type().Elem()).Elem()
		} else {
			out = out.Elem()
		}
	}
	if out.CanAddr() {
		_, ok := out.Addr().Interface().(encoding.TextUnmarshaler)
		return ok
	}
	return false
}

// settableValueOf returns a settable [reflect.Value] for the given value.
func settableValueOf(i any) reflect.Value {
	v := reflect.ValueOf(i)
	sv := reflect.New(v.Type()).Elem()
	sv.Set(v)
	return sv
}

// formatConstructorError creates a LoadError for constructor-stage errors.
func (c *Constructor) formatConstructorError(err error, mark Mark) *LoadError {
	return &LoadError{
		Stage:     ConstructorStage,
		Mark:      mark,
		Message:   err.Error(),
		err:       err,
		formatter: c.formatError,
	}
}

// formatConstructorErrorContext creates a LoadError with both context and
// problem information for constructor-stage errors.
func (c *Constructor) formatConstructorErrorContext(context string, contextMark Mark, err error, mark Mark) *LoadError {
	return &LoadError{
		Stage:       ConstructorStage,
		ContextMark: contextMark,
		ContextMsg:  context,
		Mark:        mark,
		Message:     err.Error(),
		err:         err,
		formatter:   c.formatError,
	}
}
