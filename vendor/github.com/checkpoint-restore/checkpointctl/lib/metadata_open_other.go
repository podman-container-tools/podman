// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package metadata

import "os"

func openFile(path string) (*os.File, error) {
	// Preserve the platform's ordinary open behavior. openRegularFile validates
	// the resulting descriptor before any JSON is read.
	return os.Open(path)
}
