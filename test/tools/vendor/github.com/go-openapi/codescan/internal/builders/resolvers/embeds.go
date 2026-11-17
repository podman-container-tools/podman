// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package resolvers

import (
	"go/ast"
	"go/types"
)

// Embed pairs an anonymous entry of a declaration's member list with the type go/types gave it.
//
// The AST half carries the annotation — an embed's `swagger:allOf` / `swagger:ignore` lives in its
// doc comment and nowhere else — and the type half gives what the embed denotes.
type Embed struct {
	Field *ast.Field
	Type  types.Type
}

// Embeds pairs every anonymous member of an AST member list with its type, read from the declared
// type's underlying rather than from types.Info.
//
// members is a struct's field list or an interface's method list, and under the same declaration's
// underlying type. The pairing is positional, which is exact in both shapes:
//
//   - a struct's fields are built in source order, an entry with N names contributing N fields and
//     an anonymous one contributing exactly one embedded field;
//   - an interface's embedded types are recorded in source order, so the k-th anonymous member is
//     the k-th embedded type.
//
// Entries the two halves do not agree on are dropped rather than guessed at. Anything that is
// neither a struct nor an interface has no embeds and yields nothing.
//
// Reading the underlying type instead of types.Info.Types resolves an embed for a
// package whose types were not checked from its source — export data carries the struct and its
// field types, and carries no expression records at all.
func Embeds(members []*ast.Field, under types.Type) []Embed {
	switch utpe := under.(type) {
	case *types.Struct:
		return structEmbeds(members, utpe)
	case *types.Interface:
		return interfaceEmbeds(members, utpe)
	default:
		return nil
	}
}

func structEmbeds(members []*ast.Field, utpe *types.Struct) []Embed {
	var out []Embed

	idx := 0
	for _, afld := range members {
		if len(afld.Names) > 0 { // a named field, or several sharing one type
			idx += len(afld.Names)

			continue
		}

		if idx < utpe.NumFields() && utpe.Field(idx).Embedded() {
			out = append(out, Embed{Field: afld, Type: utpe.Field(idx).Type()})
		}
		idx++
	}

	return out
}

func interfaceEmbeds(members []*ast.Field, utpe *types.Interface) []Embed {
	var out []Embed

	idx := 0
	for _, afld := range members {
		if len(afld.Names) > 0 { // an explicit method
			continue
		}

		if idx < utpe.NumEmbeddeds() {
			out = append(out, Embed{Field: afld, Type: utpe.EmbeddedType(idx)})
		}
		idx++
	}

	return out
}
