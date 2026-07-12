//go:build !windows && !linux && !darwin

package chrootarchive

import (
	"fmt"
	"net"
	"os/user"

	"golang.org/x/sys/unix"
)

// chroot to the given path and do some important initialization beforehand
func chroot(path string) (err error) {
	// initialize nss libraries in libc so that the dynamic libraries are loaded in the host
	// environment not in the chroot from untrusted files.
	_, _ = user.Lookup("storage")
	_, _ = net.LookupHost("localhost")

	if err := unix.Chroot(path); err != nil {
		return fmt.Errorf("chroot %q: %w", path, err)
	}
	if err := unix.Chdir("/"); err != nil {
		return fmt.Errorf("changing to new root after chroot: %w", err)
	}
	return nil
}
