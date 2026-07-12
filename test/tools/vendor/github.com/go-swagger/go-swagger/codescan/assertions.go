// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codescan

import (
	"fmt"
	"go/types"
)

type Error string

func (e Error) Error() string {
	return string(e)
}

const (
	ErrInternal Error = "internal error due to a bug or a mishandling of go types AST. This usually indicates a bug in the scanner"
)

// code assertions to be explicit about the various expectations when entering a function

func mustNotBeABuiltinType(o *types.TypeName) {
	if o.Pkg() != nil {
		return
	}

	panic(fmt.Errorf("type %q expected not to be a builtin: %w", o.Name(), ErrInternal))
}

func mustHaveRightHandSide(a *types.Alias) {
	if a.Rhs() != nil {
		return
	}

	panic(fmt.Errorf("type alias %q expected to declare a right-hand-side: %w", a.Obj().Name(), ErrInternal))
}
