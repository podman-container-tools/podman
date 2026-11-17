// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package repo

// LoadPlugin is a no-op on Windows.
//
// Go plugins (the "plugin" package) are not supported on Windows, so the
// template-plugin option cannot be honoured on this platform and is ignored.
func (t *Repository) LoadPlugin(_ string) error {
	return nil
}
