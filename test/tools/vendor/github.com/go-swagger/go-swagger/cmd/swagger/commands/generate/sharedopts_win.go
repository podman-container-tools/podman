// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package generate

import "github.com/go-swagger/go-swagger/generator"

// pluginOptions exposes no option on windows: the template plugin relies on go plugins,
// which windows does not support.
type pluginOptions struct{}

func (p pluginOptions) apply(_ *generator.GenOpts) {}
