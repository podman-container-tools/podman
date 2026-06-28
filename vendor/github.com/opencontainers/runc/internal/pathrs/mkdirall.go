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
	"path/filepath"
)

func splitPath(path string) (dirPath, filename string, err error) {
	dirPath, filename = filepath.Split(path)
	if filepath.Join("/", filename) == "/" {
		return "", "", fmt.Errorf("root subpath %q has bad trailing component %q", path, filename)
	}
	return dirPath, filename, nil
}

// MkdirAllParentInRoot is like [MkdirAllInRoot] except that it only creates
// the parent directory of the target path, returning the trailing component so
// the caller has more flexibility around constructing the final inode.
//
// Callers need to be very careful operating on the trailing path, as trivial
// mistakes like following symlinks can cause security bugs. Most people
// should probably just use [MkdirAllInRoot] or [CreateInRoot].
func MkdirAllParentInRoot(root *os.File, unsafePath string, mode os.FileMode) (*os.File, string, error) {
	// MkdirAllInRoot also does hallucinateUnsafePath, but we need to do it
	// here first because when we split unsafePath into (dir, file) components
	// we want to be doing so with the hallucinated path (so that trailing
	// dangling symlinks are treated correctly).
	unsafePath, err := hallucinateUnsafePath(root.Name(), unsafePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to construct hallucinated target path: %w", err)
	}

	dirPath, filename, err := splitPath(unsafePath)
	if err != nil {
		return nil, "", fmt.Errorf("split path %q for mkdir parent: %w", unsafePath, err)
	}

	dirFd, err := MkdirAllInRoot(root, dirPath, mode)
	return dirFd, filename, err
}
