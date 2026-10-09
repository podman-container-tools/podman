// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/types"

	"github.com/go-openapi/codescan/internal/builders/common"
	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/scanner"
)

// inBodyLocation is the `in:` value naming a full-Schema parameter or response field.
const inBodyLocation = "body"

// Delegate runs a schema sub-build for b and hands back whatever it discovered.
//
// The parameters and responses builders resolve most field shapes by deferring to this package, and
// each had written the three-step dance out per shape: construct a sub-builder on the caller's
// context and declaration, build, then drain the post-declaration queue into the caller. Forgetting
// the drain loses a discovered model silently, which is the kind of thing five copies eventually get
// wrong in one of them.
//
// The Options stay with the caller, because that is where the builders genuinely differ — the
// responses builder threads a cross-ref path, and a map value is targeted explicitly rather than
// through OptionFor.
func Delegate(b *common.Builder, opts ...Option) error {
	return delegateWith(b, NewBuilder(b.Ctx, b.Decl), opts...)
}

// DelegateAs is Delegate bound to a RESOLVED declaration rather than the caller's own.
//
// A field whose type resolves to another declaration is built in that declaration's context, so the
// sub-builder takes it and infers names from it. The InferNames call is easy to omit and its absence
// is not obvious in the output, which is reason enough for the two callers to share one spelling.
func DelegateAs(b *common.Builder, decl *scanner.EntityDecl, opts ...Option) error {
	sb := NewBuilder(b.Ctx, decl)
	sb.InferNames()

	return delegateWith(b, sb, opts...)
}

func delegateWith(b *common.Builder, sb *Builder, opts ...Option) error {
	if err := sb.Build(opts...); err != nil {
		return err
	}
	// Propagate the sub-build's discoveries: a model reached only through this field — no
	// swagger:model annotation, no other reference site — arrives in the spec only via this queue.
	for _, d := range sb.PostDeclarations() {
		b.AppendPostDecl(d)
	}

	return nil
}

// BuildFieldAlias resolves an alias reached as a FIELD of a `swagger:parameters` or
// `swagger:response` struct.
//
// Both builders used to carry their own copy of this, and the copies drifted: the declaration lookup
// sat below the TransparentAliases return in one and not the other, `swagger:type` reached the
// non-body branch and not the body one, and the not-found error fired at different points. Every
// classifier the alias declaration may carry lives in this package, so the resolution belongs here
// and the callers keep only what is genuinely theirs — which types they refuse, and which error
// they wrap a missing source in.
//
// notFound produces the caller's error for an alias whose declaration is not in the scanned set. It
// is consulted only on the path that needs the declaration to emit a `$ref`; a dissolve does not.
//
// extra carries per-caller Options — the responses builder threads a cross-ref path; the parameters
// builder passes none.
func BuildFieldAlias(b *common.Builder, tpe *types.Alias, typable ifaces.SwaggerTypable,
	notFound func() error, extra ...Option,
) error {
	resolvers.MustHaveRightHandSide(tpe)

	dissolve := func(t types.Type) error {
		return Delegate(b, append([]Option{OptionFor(t, typable)}, extra...)...)
	}

	// Everything dissolves through the schema builder, which owns the classifier cascade and the
	// TransparentAliases rule alike — handing it the ALIAS rather than the right-hand side is what
	// lets it read the declaration first.
	//
	// The single exception is a body field naming a swagger:model alias, which keeps its `$ref`
	// identity here. TransparentAliases overrides that, dissolving at every use site by definition.
	if typable.In() == inBodyLocation && !b.Ctx.TransparentAliases() {
		o := tpe.Obj()
		decl, found := b.Ctx.GetModel(o.Pkg().Path(), o.Name())
		if !found {
			return notFound()
		}
		if decl.HasModelAnnotation() {
			return b.MakeRef(decl, typable)
		}
	}

	return dissolve(tpe)
}
