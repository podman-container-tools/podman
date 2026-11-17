// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package vfs is the loader's single point of filesystem contact.
//
// Every read goes through it: build-constraint matching inside go/build, directory walking during pattern resolution,
// and source reading before parsing.
// One place to point at either the real filesystem or an fs.FS allows scanning a tree that was never written to
// disk possible at all.
//
// It is its own package because both halves of the loader need it and neither owns it — resolution walks directories,
// and the loader reads the files resolution found.
package vfs
