// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codescan

import (
	"fmt"
	"go/token"

	"github.com/go-openapi/codescan/internal/builders/spec"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// Options for the scanner.
type Options = scanner.Options

// Run the scanner to produce a swagger spec with the options provided.
func Run(opts *Options) (_ *oaispec.Swagger, err error) { // TODO(fred/claude): use option functors pattern
	// Last-resort backstop: the spec builder wraps each per-declaration step under a located panic
	// guard (scan.internal-panic), but a panic outside any decl loop (e.g. name reduction, the package
	// walk) would still escape as a raw stack trace.
	// Convert it into a clean error and a diagnostic instead.
	defer func() {
		if r := recover(); r != nil {
			if opts != nil && opts.OnDiagnostic != nil {
				opts.OnDiagnostic(grammar.Errorf(token.Position{}, grammar.CodeInternalPanic,
					"unrecovered panic during scan: %v", r))
			}
			err = fmt.Errorf("unrecovered panic during scan: %v: %w", r, ErrCodeScan)
		}
	}()

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
