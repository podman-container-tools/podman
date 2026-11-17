// Copyright 2026 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

package libyaml

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
)

// AliasDataPlugin creates an independent anchor policy for each input stream.
// Implementations must support concurrent calls to NewAliasDataContext.
type AliasDataPlugin interface {
	NewAliasDataContext(AliasDataServices) (AliasDataContext, error)
}

// AliasDataContext owns anchor definitions and alias lookup for one stream.
// DefineAnchor receives a live node, before its children have been composed.
// Policies may retain that pointer but must not change the node.
// ResolveAlias must return a data node when found is true. External targets
// must have resolved tags; locally defined targets are resolved by the host.
// YAML null is represented by a scalar null node, never a nil pointer.
// EndDocument is called only after successful composition.
// Contexts must not retain open resources; there is no teardown callback.
type AliasDataContext interface {
	BeginDocument() error
	DefineAnchor(name string, node *Node) error
	ResolveAlias(name string) (node *Node, found bool, err error)
	EndDocument() error
}

// AliasDataServices supplies operation-local conversions and source snapshots.
// NewNode copies node graphs and converts Go values through the representer.
// LoadFile loads exactly one mapping document with the selected parser and
// ordinary document-scoped aliases, without consulting external alias data.
// Its copied nodes retain tags, styles, and scalar text; root merges are applied.
type AliasDataServices struct {
	NewNode     func(any) (*Node, error)
	LoadFile    func(string) (*Node, error)
	Environment func() map[string]string
}

// AliasDataSources holds resolved, operation-local source nodes.
// This is an internal implementation detail shared with plugin/aliasdata.
type AliasDataSources struct {
	Data, File, Env       map[string]*Node
	Stream, Path, Disable bool
}

type anchorPolicy struct {
	sources AliasDataSources
	current map[string]*Node
	prior   map[string]*Node
}

// NewAliasDataPolicy constructs an independent node-based anchor policy.
func NewAliasDataPolicy(sources AliasDataSources) AliasDataContext {
	return &anchorPolicy{
		sources: sources,
		current: map[string]*Node{},
		prior:   map[string]*Node{},
	}
}

func (p *anchorPolicy) BeginDocument() error {
	p.current = map[string]*Node{}
	return nil
}

func (p *anchorPolicy) DefineAnchor(name string, node *Node) error {
	p.current[name] = node
	return nil
}

func (p *anchorPolicy) ResolveAlias(name string) (*Node, bool, error) {
	if p.sources.Disable {
		return nil, false, errors.New("alias-data: aliases are disabled")
	}
	parts := []string{name}
	if p.sources.Path && strings.Contains(name, ".") {
		parts = strings.Split(name, ".")
		if err := validateAliasPath(name, parts); err != nil {
			return nil, false, err
		}
	}
	base := parts[0]
	for _, source := range []map[string]*Node{
		p.current, p.sources.Data, p.sources.File, p.sources.Env, p.prior,
	} {
		if node, found := source[base]; found {
			result, err := walkAliasPath(name, node, parts[1:])
			return result, true, err
		}
	}
	if len(parts) > 1 {
		return nil, false, aliasPathError(name, base, "unknown anchor")
	}
	return nil, false, nil
}

func (p *anchorPolicy) EndDocument() error {
	if p.sources.Stream && !p.sources.Disable {
		for name, node := range p.current {
			p.prior[name] = node
		}
	}
	return nil
}

func aliasDataServices(opts *Options) AliasDataServices {
	return AliasDataServices{
		NewNode: func(value any) (node *Node, err error) {
			defer handleErr(&err)
			if err := checkAliasDataValue(reflect.ValueOf(value), opts,
				map[aliasDataVisit]bool{}, 0); err != nil {
				return nil, err
			}
			node = NewRepresenter(opts).Represent("", reflect.ValueOf(value))
			return copyAliasDataNode(node, opts)
		},
		LoadFile: func(path string) (root *Node, err error) {
			defer handleErr(&err)
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			var document Node
			err = Load(data, &document, func(o *Options) error {
				*o = *opts
				o.AliasData = nil
				o.AllDocuments = false
				o.SingleDocument = false
				o.StreamNodes = false
				o.allowLegacyTrailingContent = false
				return nil
			})
			if err != nil {
				return nil, err
			}
			if document.Kind != DocumentNode || len(document.Content) != 1 ||
				document.Content[0].Kind != MappingNode {
				return nil, errors.New("alias-data: file root must be a mapping")
			}
			// Expand only the root's merge directives. Keep values as resolved
			// nodes so destination decoding sees their tags, styles, and text.
			constructor := NewConstructor(opts)
			root = constructor.effectiveMapping(document.Content[0], ifaceType)
			if len(constructor.TypeErrors) > 0 {
				return nil, &LoadErrors{Errors: constructor.TypeErrors}
			}
			return copyAliasDataNode(root, opts)
		},
		Environment: func() map[string]string {
			values := map[string]string{}
			for _, entry := range os.Environ() {
				name, value, _ := strings.Cut(entry, "=")
				values[name] = value
			}
			return values
		},
	}
}

type aliasDataVisit struct {
	typeOf reflect.Type
	ptr    uintptr
	length int
}

// Check Go containers before representing them, since the ordinary representer
// does not detect cyclic Go values. Node alias graphs are copied separately.
func checkAliasDataValue(value reflect.Value, opts *Options,
	active map[aliasDataVisit]bool, depth int,
) error {
	if !value.IsValid() {
		return nil
	}
	if value.CanInterface() {
		switch value.Interface().(type) {
		case Node, *Node, Marshaler, encoding.TextMarshaler:
			return nil
		}
	}
	if opts.DepthCheck != nil {
		if err := opts.DepthCheck(depth, &DepthContext{Kind: DepthKindFlow}); err != nil {
			return err
		}
	}
	switch value.Kind() {
	case reflect.Interface:
		return checkAliasDataValue(value.Elem(), opts, active, depth)
	case reflect.Map, reflect.Slice, reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		visit := aliasDataVisit{
			typeOf: value.Type(), ptr: uintptr(value.UnsafePointer()),
		}
		if value.Kind() == reflect.Slice {
			visit.length = value.Len()
		}
		if active[visit] {
			return errors.New("alias-data: cyclic Go value")
		}
		active[visit] = true
		defer delete(active, visit)
	}
	switch value.Kind() {
	case reflect.Pointer:
		return checkAliasDataValue(value.Elem(), opts, active, depth)
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			if err := checkAliasDataValue(iter.Key(), opts, active, depth+1); err != nil {
				return err
			}
			if err := checkAliasDataValue(iter.Value(), opts, active, depth+1); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := checkAliasDataValue(value.Index(i), opts, active, depth+1); err != nil {
				return err
			}
		}
	case reflect.Struct:
		info, err := getStructInfo(value.Type())
		if err != nil {
			return err
		}
		for _, field := range info.FieldsList {
			var item reflect.Value
			if field.Inline == nil {
				item = value.Field(field.Num)
			} else {
				item = NewRepresenter(opts).fieldByIndex(value, field.Inline)
			}
			if !item.IsValid() {
				continue
			}
			if field.OmitEmpty && isZero(item) {
				continue
			}
			if err := checkAliasDataValue(item, opts, active, depth+1); err != nil {
				return err
			}
		}
		if info.InlineMap >= 0 {
			return checkAliasDataValue(value.Field(info.InlineMap), opts, active, depth+1)
		}
	}
	return nil
}

// copyAliasDataNode copies the complete graph, preserving alias edges while
// rejecting cycles through Content. Resolve each node without following aliases.
func copyAliasDataNode(root *Node, opts *Options) (*Node, error) {
	if root != nil && root.Kind == DocumentNode && len(root.Content) == 1 {
		root = root.Content[0]
	}
	copies := map[*Node]*Node{}
	active := map[*Node]bool{}
	var copyNode func(*Node, int, bool) (*Node, error)
	copyNode = func(node *Node, depth int, alias bool) (*Node, error) {
		if node == nil {
			return nil, errors.New("alias-data: nil data node")
		}
		if active[node] && !alias {
			return nil, errors.New("alias-data: cyclic node content")
		}
		if copied, found := copies[node]; found {
			return copied, nil
		}
		switch node.Kind {
		case ScalarNode, SequenceNode, MappingNode:
		case AliasNode:
			if node.Alias == nil {
				return nil, errors.New("alias-data: unresolved source alias")
			}
		default:
			return nil, fmt.Errorf("alias-data: invalid data node kind %d", node.Kind)
		}
		if node.Kind == MappingNode && len(node.Content)%2 != 0 {
			return nil, errors.New("alias-data: mapping needs key/value node pairs")
		}
		if opts.DepthCheck != nil {
			if err := opts.DepthCheck(depth, &DepthContext{Kind: DepthKindFlow}); err != nil {
				return nil, err
			}
		}
		copied := *node
		copied.Content = nil
		copied.Alias = nil
		copies[node] = &copied
		active[node] = true
		for _, child := range node.Content {
			value, err := copyNode(child, depth+1, false)
			if err != nil {
				return nil, err
			}
			copied.Content = append(copied.Content, value)
		}
		if node.Alias != nil {
			value, err := copyNode(node.Alias, depth+1, true)
			if err != nil {
				return nil, err
			}
			copied.Alias = value
		}
		delete(active, node)
		// Child nodes have already been resolved. A shallow copy avoids
		// repeatedly walking shared subtrees and recursive alias graphs.
		shallow := copied
		shallow.Content = nil
		NewResolver(opts).Resolve(&shallow)
		copied.Tag = shallow.Tag
		return &copied, nil
	}
	return copyNode(root, 0, false)
}
