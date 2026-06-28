// SPDX-License-Identifier: Apache-2.0
/*
 * Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
 * Copyright (C) 2024-2025 SUSE LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package pathrs

import (
	"fmt"
	"os"

	"github.com/cyphar/filepath-securejoin/pathrs-lite"
	"github.com/sirupsen/logrus"
)

// MkdirAllInRoot attempts to make
//
//	path, _ := securejoin.SecureJoin(root.Name(), unsafePath)
//	os.MkdirAll(path, mode)
//	os.Open(path)
//
// safer against attacks where components in the path are changed between
// SecureJoin returning and MkdirAll (or Open) being called. In particular, we
// try to detect any symlink components in the path while we are doing the
// MkdirAll.
//
// NOTE: If unsafePath is a subpath of root, we assume that you have already
// called SecureJoin and so we use the provided path verbatim without resolving
// any symlinks (this is done in a way that avoids symlink-exchange races).
// This means that the path also must not contain ".." elements, otherwise an
// error will occur.
//
// This uses (pathrs-lite).MkdirAllHandle under the hood, but it has special
// handling if unsafePath has already been scoped within the rootfs (this is
// needed for a lot of runc callers and fixing this would require reworking a
// lot of path logic).
func MkdirAllInRoot(root *os.File, unsafePath string, mode os.FileMode) (*os.File, error) {
	unsafePath, err := hallucinateUnsafePath(root.Name(), unsafePath)
	if err != nil {
		return nil, fmt.Errorf("failed to construct hallucinated target path: %w", err)
	}

	// Check for any silly mode bits.
	if mode&^0o7777 != 0 {
		return nil, fmt.Errorf("tried to include non-mode bits in MkdirAll mode: 0o%.3o", mode)
	}
	// Linux (and thus os.MkdirAll) silently ignores the suid and sgid bits if
	// passed. While it would make sense to return an error in that case (since
	// the user has asked for a mode that won't be applied), for compatibility
	// reasons we have to ignore these bits.
	if ignoredBits := mode &^ 0o1777; ignoredBits != 0 {
		logrus.Warnf("MkdirAll called with no-op mode bits that are ignored by Linux: 0o%.3o", ignoredBits)
		mode &= 0o1777
	}

	return retryEAGAIN(func() (*os.File, error) {
		return pathrs.MkdirAllHandle(root, unsafePath, mode)
	})
}
