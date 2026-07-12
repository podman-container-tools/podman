// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"github.com/go-swagger/go-swagger/generator"
)

// contribOptionsOverride gives contributed templates the ability to override the options if they need.
func contribOptionsOverride(opts *generator.GenOpts) {
	if opts.Template == "stratoscale" {
		// Stratoscale template needs to regenerate the configureapi on every run.
		opts.RegenerateConfigureAPI = true
		// It also does not use the main.go
		opts.IncludeMain = false
	}
}
