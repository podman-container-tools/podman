//go:build !remote && (linux || freebsd)

package abi

import (
	"fmt"
	"os"
	"strings"

	"go.podman.io/podman/v6/libpod/define"
	"golang.org/x/sys/unix"
)

// getSdNotifyMode returns the `sdNotifyAnnotation/$name` for the specified
// name. If name is empty, it'll only look for `sdNotifyAnnotation`.
func getSdNotifyMode(annotations map[string]string, name string) (string, error) {
	var mode string
	switch len(name) {
	case 0:
		mode = annotations[sdNotifyAnnotation]
	default:
		mode = annotations[sdNotifyAnnotation+"/"+name]
	}
	return mode, define.ValidateSdNotifyMode(mode)
}

// openPathSafely opens the given name under the trusted root path, the unsafeName
// must be a single path component and not contain "/".
// The resulting path will be opened or created if it does not exists.
// Following of symlink is done within staying under root, escapes outsides
// of root are not allowed and prevent.
//
// This custom function is needed because securejoin.SecureJoin() is not race safe
// and the volume might be mounted in another container that could swap in a symlink
// after the function ahs run. securejoin.OpenInRoot() doesn't work either because
// it cannot create files and doesn't work on freebsd.
func openPathSafely(root, unsafeName string) (*os.File, error) {
	if strings.Contains(unsafeName, "/") {
		return nil, fmt.Errorf("name %q must not contain path separator", unsafeName)
	}
	fdDir, err := os.OpenFile(root, unix.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer fdDir.Close()
	flags := unix.O_CREAT | unix.O_WRONLY | unix.O_TRUNC | unix.O_CLOEXEC
	fd, err := unix.Openat(int(fdDir.Fd()), unsafeName, flags|unix.O_NOFOLLOW, 0o644)
	if err == nil {
		return os.NewFile(uintptr(fd), unsafeName), nil
	}
	if err == unix.ELOOP {
		return openSymlinkPath(fdDir, unsafeName, flags)
	}
	return nil, &os.PathError{Op: "openat", Path: unsafeName, Err: err}
}
