// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"errors"
	"fmt"
	"go/types"
)

// ErrSchema is the sentinel error for all errors originating from the schema builder package.
var ErrSchema = errors.New("codescan:builders:schema")

func missingSource(tpe types.Type) error {
	return fmt.Errorf("can't find source file for type: %v: %w", tpe, ErrSchema)
}
