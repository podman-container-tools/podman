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
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
)

// IsLexicallyInRoot is shorthand for strings.HasPrefix(path+"/", root+"/"),
// but properly handling the case where path or root have a "/" suffix.
//
// NOTE: The return value only make sense if the path is already mostly cleaned
// (i.e., doesn't contain "..", ".", nor unneeded "/"s).
func IsLexicallyInRoot(root, path string) bool {
	root = strings.TrimRight(root, "/")
	path = strings.TrimRight(path, "/")
	return strings.HasPrefix(path+"/", root+"/")
}

// LexicallyCleanPath makes a path safe for use with filepath.Join. This is
// done by not only cleaning the path, but also (if the path is relative)
// adding a leading '/' and cleaning it (then removing the leading '/'). This
// ensures that a path resulting from prepending another path will always
// resolve to lexically be a subdirectory of the prefixed path. This is all
// done lexically, so paths that include symlinks won't be safe as a result of
// using CleanPath.
func LexicallyCleanPath(path string) string {
	// Deal with empty strings nicely.
	if path == "" {
		return ""
	}

	// Ensure that all paths are cleaned (especially problematic ones like
	// "/../../../../../" which can cause lots of issues).

	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	// If the path isn't absolute, we need to do more processing to fix paths
	// such as "../../../../<etc>/some/path". We also shouldn't convert absolute
	// paths to relative ones.
	path = filepath.Clean(string(os.PathSeparator) + path)
	// This can't fail, as (by definition) all paths are relative to root.
	path, _ = filepath.Rel(string(os.PathSeparator), path)

	return path
}

// LexicallyStripRoot returns the passed path, stripping the root path if it
// was (lexicially) inside it. Note that both passed paths will always be
// treated as absolute, and the returned path will also always be absolute. In
// addition, the paths are cleaned before stripping the root.
func LexicallyStripRoot(root, path string) string {
	// Make the paths clean and absolute.
	root, path = LexicallyCleanPath("/"+root), LexicallyCleanPath("/"+path)
	switch {
	case path == root:
		path = "/"
	case root == "/":
		// do nothing
	default:
		path = strings.TrimPrefix(path, root+"/")
	}
	return LexicallyCleanPath("/" + path)
}

// hallucinateUnsafePath creates a new unsafePath which has all symlinks
// (including dangling symlinks) fully resolved and any non-existent components
// treated as though they are real. This is effectively just a wrapper around
// [securejoin.SecureJoin] that strips the root. This path *IS NOT* safe to use
// as-is, you *MUST* operate on the returned path with pathrs-lite.
//
// The reason for this methods is that in previous runc versions, we would
// tolerate nonsense paths with dangling symlinks as path components.
// pathrs-lite does not support this, so instead we have to emulate this
// behaviour by doing SecureJoin *purely to get a semi-reasonable path to use*
// and then we use pathrs-lite to operate on the path safely.
//
// It would be quite difficult to emulate this in a race-free way in
// pathrs-lite, so instead we use [securejoin.SecureJoin] to simply produce a
// new candidate path for operations like [MkdirAllInRoot] so they can then
// operate on the new unsafePath as if it was what the user requested.
//
// If unsafePath is already lexically inside root, it is stripped before
// re-resolving it (this is done to ensure compatibility with legacy callers
// within runc that call SecureJoin before calling into pathrs).
func hallucinateUnsafePath(root, unsafePath string) (string, error) {
	unsafePath = LexicallyStripRoot(root, unsafePath)
	weirdPath, err := securejoin.SecureJoin(root, unsafePath)
	if err != nil {
		return "", err
	}
	unsafePath = LexicallyStripRoot(root, weirdPath)
	return unsafePath, nil
}
