// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package generator

type GenOpts struct {
	GenOptsCommon
}

func (g *GenOpts) setTemplates() error {
	return g.GenOptsCommon.setTemplates()
}
