// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package packages

import "golang.org/x/tools/go/packages"

// The public vocabulary is aliased, not redefined, so that a caller can move between this loader and packages.Load
// without touching anything but the call itself.
type (
	// Config mirrors packages.Config.
	//
	// See the package doc for which fields are honoured.
	Config = packages.Config

	// Package mirrors packages.Package.
	//
	// Fields codescan does not consume are left unset.
	Package = packages.Package

	// Error mirrors packages.Error.
	Error = packages.Error

	// LoadMode mirrors packages.LoadMode.
	//
	// It is accepted for signature compatibility; this loader always produces syntax and type information, since that is
	// the only mode codescan asks for.
	LoadMode = packages.LoadMode
)

// Error kinds, re-exported for callers that classify Errors.
const (
	ListError  = packages.ListError
	ParseError = packages.ParseError
	TypeError  = packages.TypeError
)
