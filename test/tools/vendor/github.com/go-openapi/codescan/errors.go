// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codescan

type codescanError string

func (e codescanError) Error() string {
	return string(e)
}

const (
	// ErrCodeScan is the sentinel error for all errors originating from the codescan package.
	ErrCodeScan codescanError = "codescan error"
)
