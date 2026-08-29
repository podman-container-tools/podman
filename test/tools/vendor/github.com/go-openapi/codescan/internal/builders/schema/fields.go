// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/go-openapi/codescan/internal/builders/handlers"
	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// fieldCarrier holds everything the unified field-emission pipeline needs.
//
// Per-source extractors (struct field, interface method) build it; applyFieldCarrier consumes it to
// emit one Swagger property.
type fieldCarrier struct {
	name      string     // JSON property name (post-tag, post-mangler)
	goName    string     // Go identifier (for x-go-name override)
	propType  types.Type // type to build the property schema from
	afld      *ast.Field // AST field, used for block-comment lookup
	fd        fieldDoc   // pre-scanned doc-comment signals
	isString  bool       // JSON tag's ",string" option (struct only)
	omitEmpty bool       // affects the nullable rule (struct only)
}

// propOwner records the writer of a property: the Go field name and the embed-recursion depth at
// write time.
//
// Used by:
//   - structFieldCarrier — reverse-lookup for `json:"-"` eviction
//     (find the JSON name a given Go name was last bound to).
//   - applyFieldCarrier — ambiguity detection (a later embed-side
//     write at the same or shallower depth than the prior one and
//     with a different Go name is the same-depth-ambiguity case Go
//     itself would refuse to promote).
type propOwner struct {
	goName string
	depth  int
}

// nameByJSON maps a JSON property name to its most recent writer.
//
// The original implementation was map[string]string and drove a post-pass GC over
// target.Properties; that pass was dead under every traced path (applyFieldCarrier is the only
// writer to target.Properties and it always writes the matching nameByJSON entry, so
// `Properties.keys() ⊆ nameByJSON.keys()` is an invariant) and has been removed.

// applyFieldCarrier emits one property of target from c. Common pipeline shared by
// processStructField, processInterfaceMethod and processAnonInterfaceMethod.
//
// `nameByJSON` is optional: nil for anonymous interfaces, which don't track duplicates.
//
// # Details
//
// See [§user-overrides](./README.md#user-overrides) — the last-write-wins ordering across
// `isString` / `StrfmtName` / `TypeOverride` / `applyBlockToField`, plus the `x-go-name` and
// pointer-nullable rules at the tail.
func (s *Builder) applyFieldCarrier(c fieldCarrier, target *oaispec.Schema, nameByJSON map[string]propOwner) error {
	if target.Properties == nil {
		target.Properties = make(map[string]oaispec.Schema)
	}

	// Cross-ref linkage: advance the base path to this property's pointer for the duration of its
	// value build, so anchors emitted within (enum values, nested-object properties, slice/map element
	// fields) carry the correct pointer. parentPath is restored on exit; the property's own anchor is
	// recorded at fieldPath below.
	parentPath := s.path
	fieldPath := ""
	if parentPath != "" {
		fieldPath = parentPath + scanner.JSONPointer("properties", c.name)
	}
	s.path = fieldPath
	defer func() { s.path = parentPath }()

	ps := target.Properties[c.name]
	if err := s.buildFromType(c.propType, NewTypable(&ps, 0, s.skipExtensions)); err != nil {
		return err
	}
	if c.isString {
		ps.Typed("string", ps.Format)
		ps.Ref = oaispec.Ref{}
		ps.Items = nil
	}
	// swagger:type + swagger:strfmt precedence (F3). swagger:type wins on the type axis and always
	// inlines; swagger:strfmt then applies as a supplementary format only when compatible with the
	// resolved type. strfmt ALONE keeps forcing the string-encoded representation (go-swagger#1512).
	// See [§user-overrides](./README.md#user-overrides).
	pos := s.Ctx.PosOf(c.afld.Pos())
	switch {
	case c.fd.TypeOverride != "":
		ps = oaispec.Schema{}
		override := NewTypable(&ps, 0, s.skipExtensions)
		if s.resolveTypeOverride(c.fd.TypeOverride, override, c.propType, pos) {
			if c.fd.StrfmtName != "" {
				s.applyStrfmtFormat(&ps, c.fd.StrfmtName, pos)
			}
		} else {
			// Unresolved (file / unknown): fall through to the field's Go type, inlined as before.
			ps = oaispec.Schema{}
			if err := s.buildFromType(c.propType.Underlying(), NewTypable(&ps, 0, s.skipExtensions)); err != nil {
				return err
			}
		}
	case c.fd.StrfmtName != "":
		ps.Typed("string", c.fd.StrfmtName)
		ps.Ref = oaispec.Ref{}
		ps.Items = nil
	}

	s.applyBlockToField(c.afld, target, &ps, c.name)

	// required: inherited from an embedding field (go-swagger#2701), unless the promoted property set
	// its own required: explicitly.
	// The flag lives on the enclosing object's Required list, so it is written to target.
	if s.embedInherited.RequiredSet && s.embedInherited.Required {
		if _, ownRequired := s.ParseBlock(c.afld.Doc).GetBool(grammar.KwRequired); !ownRequired {
			handlers.SetRequired(target, c.name, true)
		}
	}

	if ps.Ref.String() == "" && c.name != c.goName {
		resolvers.AddExtension(&ps.VendorExtensible, "x-go-name", c.goName, s.skipExtensions)
	}
	if _, isPointer := c.propType.(*types.Pointer); isPointer && !c.omitEmpty {
		s.applyNullable(&ps)
	}

	if nameByJSON != nil {
		s.diagnoseAmbiguousEmbed(c, nameByJSON)
		nameByJSON[c.name] = propOwner{goName: c.goName, depth: s.embedDepth}
	}
	target.Properties[c.name] = ps

	// Cross-ref linkage: anchor the property to its struct field.
	// Only when a base path was initiated (WithPath) for the parent node and a sink is wired.
	// fieldPath is this property's pointer (s.path was advanced to it above and is restored to
	// parentPath on exit).
	if fieldPath != "" && c.afld != nil && s.Ctx.OriginEnabled() {
		s.Ctx.RecordOrigin(fieldPath, s.Ctx.PosOf(c.afld.Pos()))
	}

	return nil
}

// repath swaps the cross-ref base path (s.path) for the duration of a child build and returns a
// restore func, mirroring enterEmbed's idiom.
//
// Use `defer s.repath("")()` to clear the path when descending into a subtree whose node pointer
// isn't tracked (an allOf member, an aliased compound), so no wrong anchor is emitted there; finer
// nodes resolve to the nearest anchored ancestor instead.
func (s *Builder) repath(base string) func() {
	saved := s.path
	s.path = base
	return func() { s.path = saved }
}

// descend path-joins segments onto the cross-ref base path for the duration of a child build
// (items, additionalProperties, …), keeping s.path aligned with the schema node currently being
// filled so anchors emitted within — enum values, nested-object properties — carry the correct
// pointer.
//
// No-op (and no restore cost) when provenance is off (s.path == "").
func (s *Builder) descend(segments ...string) func() {
	if s.path == "" {
		return func() {}
	}
	return s.repath(s.path + scanner.JSONPointer(segments...))
}

// descendItems path-joins `depth` "items" segments onto the cross-ref base path for the duration of
// an array-element build, so anchors emitted inside the innermost element (an inlined struct's
// properties, enum values, validations) carry the …/items[/items…] pointer that matches where
// the element renders.
//
// Used by the `[]T` array-layer arms of swagger:type and swagger:additionalProperties, which build
// their layers via Items() rather than the structural slice/array path.
// No-op (and no restore cost) when provenance is off (s.path == "") or depth == 0.
func (s *Builder) descendItems(depth int) func() {
	if s.path == "" || depth == 0 {
		return func() {}
	}
	segments := make([]string, depth)
	for i := range segments {
		segments[i] = "items"
	}
	return s.descend(segments...)
}

// diagnoseAmbiguousEmbed fires a SeverityWarning Diagnostic on embed-side writes that would
// overwrite a prior entry whose write was at the same or shallower depth and bound to a different
// Go name.
//
// Last-write-wins is preserved; only the signal is added.
//
// # Details
//
// See [§embed-depth](./README.md#embed-depth) — depth-rule disambiguation and the three
// classification cases.
func (s *Builder) diagnoseAmbiguousEmbed(c fieldCarrier, nameByJSON map[string]propOwner) {
	if s.embedDepth == 0 {
		return
	}
	prior, found := nameByJSON[c.name]
	if !found || prior.goName == c.goName {
		return
	}
	if prior.depth > s.embedDepth {
		// Legitimate depth-rule shadowing: current writer is closer to the parent struct than the prior,
		// Go would prefer it.
		return
	}
	var pos token.Position
	if c.afld != nil {
		pos = s.Ctx.PosOf(c.afld.Pos())
	}
	s.RecordDiagnostic(grammar.Diagnostic{
		Pos:      pos,
		Severity: grammar.SeverityWarning,
		Code:     grammar.CodeAmbiguousEmbed,
		Message: fmt.Sprintf(
			"JSON property %q is promoted by Go field %q (depth %d) and Go field %q (depth %d); "+
				"Go would treat this as ambiguous and not promote it. The schema emits the later writer's shape.",
			c.name, prior.goName, prior.depth, c.goName, s.embedDepth,
		),
	})
}

// structFieldCarrier produces the carrier for a struct field, or returns ok=false when the field
// must be skipped silently (embedded, unexported, no AST, doc-ignored, JSON-tag-ignored).
//
// The JSON-tag-ignored case carries a side effect: it removes from target.Properties any entry
// whose owner Go name matches the current field's Go name — an embed-side property re-declared
// with `json:"-"` wins over the inherited one.
func (s *Builder) structFieldCarrier(fld *types.Var, decl *scanner.EntityDecl, target *oaispec.Schema, nameByJSON map[string]propOwner) (fieldCarrier, bool, error) {
	if fld.Embedded() || !fld.Exported() {
		return fieldCarrier{}, false, nil
	}

	afld := resolvers.FindASTField(decl.File, fld.Pos())
	if afld == nil && fld.Pkg() != nil {
		// The field is not in the embedding decl's file.
		// This happens when an embedded named type promotes fields whose source lives elsewhere — e.g.
		// embedding a cross-package defined type (`type AnotherPackageAlias color.Color`), where `decl`
		// is the alias in package `a` but the fields belong to `color.Color` in package `color`.
		//
		// Resolve the field's AST against its own source file so its json tag and doc are read correctly.
		// See go-swagger#2417.
		if file, ok := s.Ctx.FileForPos(fld.Pkg().Path(), fld.Pos()); ok {
			afld = resolvers.FindASTField(file, fld.Pos())
		}
	}
	if afld == nil {
		return fieldCarrier{}, false, nil
	}

	fd := s.scanFieldDoc(afld)
	if fd.Ignored {
		return fieldCarrier{}, false, nil
	}

	name, ignore, isString, omitEmpty, err := resolvers.ParseFieldTag(afld, fld.Name(), s.Ctx.NameFromTags())
	if err != nil {
		return fieldCarrier{}, false, err
	}
	if ignore {
		for jsonName, prior := range nameByJSON {
			if prior.goName == fld.Name() {
				delete(target.Properties, jsonName)
				break
			}
		}
		return fieldCarrier{}, false, nil
	}

	// swagger:name overrides the json-tag / field-name derivation on a struct field — matching its
	// documented "field OR method" scope and the behaviour already honoured on interface methods (F5,
	// doc-site-quirks.md). applyFieldCarrier still records the Go name as x-go-name when it differs
	// from the emitted JSON name.
	if fd.JSONName != "" {
		name = fd.JSONName
	}

	return fieldCarrier{
		name:      name,
		goName:    fld.Name(),
		propType:  fld.Type(),
		afld:      afld,
		fd:        fd,
		isString:  isString,
		omitEmpty: omitEmpty,
	}, true, nil
}

// methodCarrier produces the carrier for an interface method, or returns ok=false when the method
// must be skipped silently (unexported, not a parameterless single-result signature, no AST,
// doc-ignored).
func (s *Builder) methodCarrier(fld *types.Func, decl *scanner.EntityDecl) (fieldCarrier, bool) {
	if !fld.Exported() {
		return fieldCarrier{}, false
	}

	sig, isSignature := fld.Type().(*types.Signature)
	if !isSignature {
		return fieldCarrier{}, false
	}
	if sig.Params().Len() > 0 {
		return fieldCarrier{}, false
	}
	if sig.Results() == nil || sig.Results().Len() != 1 {
		return fieldCarrier{}, false
	}

	afld := resolvers.FindASTField(decl.File, fld.Pos())
	if afld == nil {
		return fieldCarrier{}, false
	}

	fd := s.scanFieldDoc(afld)
	if fd.Ignored {
		return fieldCarrier{}, false
	}

	name := fd.JSONName
	if name == "" {
		// No swagger:name override: auto-jsonify the Go method name unless the author opted out, in which
		// case the Go name is emitted verbatim.
		if s.Ctx.SkipJSONifyInterfaceMethods() {
			name = fld.Name()
		} else {
			name = s.interfaceJSONName(fld.Name())
		}
	}

	return fieldCarrier{
		name:     name,
		goName:   fld.Name(),
		propType: sig.Results().At(0).Type(),
		afld:     afld,
		fd:       fd,
	}, true
}

func (s *Builder) applyNullable(target *oaispec.Schema) {
	if !s.Ctx.SetXNullableForPointers() {
		return
	}

	if target.Extensions == nil || (target.Extensions["x-nullable"] == nil && target.Extensions["x-isnullable"] == nil) {
		target.AddExtension("x-nullable", true)
	}
}

// interfaceJSONName maps a Go interface-method name to its JSON property name via the Builder's
// mangler.
func (s *Builder) interfaceJSONName(goName string) string {
	return s.methodMangler.ToJSONName(goName)
}
