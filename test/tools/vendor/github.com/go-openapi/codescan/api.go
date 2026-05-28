// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codescan

import (
	"fmt"

	"github.com/go-openapi/codescan/internal/builders/spec"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// Options for the scanner.
type Options = scanner.Options

// Run the scanner to produce a swagger spec with the options provided.
func Run(opts *Options) (*oaispec.Swagger, error) { // TODO(fred/claude): use option functors pattern
	ctx, err := scanner.NewScanCtx(opts)
	if err != nil {
		return nil, fmt.Errorf("could not scan source: %w: %w", err, ErrCodeScan)
	}

	builder := spec.NewBuilder(opts.InputSpec, ctx, opts.ScanModels) // TODO(fred/claude): use option functors pattern
	sp, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("could not build spec: %w: %w", err, ErrCodeScan)
	}

	return sp, nil
}
