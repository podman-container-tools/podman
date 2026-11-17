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

	"github.com/cyphar/filepath-securejoin/pathrs-lite"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/internal/linux"
)

// OpenInRoot opens the given path inside the root with the provided flags. It
// is effectively shorthand for [securejoin.OpenatInRoot] followed by
// [securejoin.Reopen].
func OpenInRoot(root *os.File, subpath string, flags int) (*os.File, error) {
	handle, err := retryEAGAIN(func() (*os.File, error) {
		return pathrs.OpenatInRoot(root, subpath)
	})
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	return Reopen(handle, flags)
}

// CreateInRoot creates a new file inside a root (as well as any missing parent
// directories) and returns a handle to said file. This effectively has
// open(O_CREAT|O_NOFOLLOW) semantics. If you want the creation to use O_EXCL,
// include it in the passed flags. The fileMode argument uses unix.* mode bits,
// *not* os.FileMode.
func CreateInRoot(root *os.File, subpath string, flags int, fileMode uint32) (*os.File, error) {
	dirFd, filename, err := MkdirAllParentInRoot(root, subpath, 0o755)
	if err != nil {
		return nil, err
	}
	defer dirFd.Close()

	// We know that the filename does not have any "/" components, and that
	// dirFd is inside the root. O_NOFOLLOW will stop us from following
	// trailing symlinks, so this is safe to do. libpathrs's Root::create_file
	// works the same way.
	flags |= unix.O_CREAT | unix.O_NOFOLLOW
	fd, err := linux.Openat(int(dirFd.Fd()), filename, flags, fileMode)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), root.Name()+"/"+subpath), nil
}

// UnlinkInRoot deletes the inode specified at the given subpath. If you pass
// [unix.AT_REMOVEDIR] it will remove directories, otherwise it will remove
// non-directory inodes.
func UnlinkInRoot(root *os.File, subpath string, flags int) error {
	dirPath, filename, err := splitPath(subpath)
	if err != nil {
		return fmt.Errorf("split path %q for unlink: %w", subpath, err)
	}

	dirFd := root
	if filepath.Join("/", dirPath) != "/" {
		newDirFd, err := OpenInRoot(root, dirPath, unix.O_DIRECTORY|unix.O_PATH)
		if err != nil {
			return fmt.Errorf("failed to open parent directory %q for unlink: %w", dirPath, err)
		}
		dirFd = newDirFd
		defer dirFd.Close()
	}

	err = unix.Unlinkat(int(dirFd.Fd()), filename, flags)
	if err != nil {
		err = &os.PathError{Op: "unlinkat", Path: dirFd.Name() + "/" + filename, Err: err}
	}
	return err
}

// SymlinkInRoot creates a symlink inside a root with the given target (as well
// as creating any missing parent directories). If the subpath already exists,
// an error is returned.
func SymlinkInRoot(linktarget string, root *os.File, subpath string) error {
	dirFd, filename, err := MkdirAllParentInRoot(root, subpath, 0o755)
	if err != nil {
		return err
	}
	defer dirFd.Close()

	err = unix.Symlinkat(linktarget, int(dirFd.Fd()), filename)
	if err != nil {
		err = &os.PathError{Op: "symlinkat", Path: dirFd.Name() + "/" + filename, Err: err}
	}
	return err
}
