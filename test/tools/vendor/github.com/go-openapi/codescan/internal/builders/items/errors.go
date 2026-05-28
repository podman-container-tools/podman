// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package items

type itemsError string

func (e itemsError) Error() string {
	return string(e)
}

const (
	// ErrItems is the sentinel error for all errors originating from the items package.
	ErrItems itemsError = "builders:items"
)
