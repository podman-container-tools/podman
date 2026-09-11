package golang

import (
	"maps"
	"text/template"
)

// Merge [template.FuncMap] s into the target [template.FuncMap].
func Merge(target template.FuncMap, merged ...template.FuncMap) template.FuncMap {
	return MergeMaps(target, merged...)
}

// Coalesce [template.FuncMap] s into the target [template.FuncMap].
//
// Built-in functions are implied and are therefore always protected from an override.
func Coalesce(target template.FuncMap, coalesced ...template.FuncMap) template.FuncMap {
	m := CoalesceMaps(target, coalesced...)
	for _, builtin := range builtinFuncMap {
		delete(m, builtin)
	}

	return m
}

// builtins holds the name of all built-in functions.
//
// These are provided by the text/templates package.
//
// See https://pkg.go.dev/text/template@go1.26.5#hdr-Functions
var builtinFuncMap = []string{
	"and", "not", "or",
	"eq", "ge", "gt", "le", "lt", "ne",
	"call",
	"index", "slice", "len",
	"print", "printf", "println",
	"html", "js", "urlquery",
}

// MergeMaps merges maps into the target.
//
// If the target is nil, a new merged map is created.
//
// Merge semantics are: overwrite, last win.
func MergeMaps[M ~map[K]V, K comparable, V any](target M, merged ...M) M {
	if target == nil {
		var c int
		for _, m := range merged {
			c += len(m)
		}
		target = make(map[K]V, c)
	}

	for _, m := range merged {
		maps.Copy(target, m)
	}

	return target
}

// CoalesceMaps merges maps into the target.
//
// If the target is nil, a new merged map is created.
//
// Merge semantics are: coalesce without overwrite, first win.
func CoalesceMaps[M ~map[K]V, K comparable, V any](target M, coalesced ...M) M {
	if target == nil {
		var c int
		for _, m := range coalesced {
			c += len(m)
		}
		target = make(map[K]V, c)
	}

	for _, co := range coalesced {
		for k, v := range co {
			_, found := target[k]
			if found {
				continue
			}
			target[k] = v
		}
	}

	return target
}
