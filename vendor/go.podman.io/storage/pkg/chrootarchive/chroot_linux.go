package chrootarchive

import (
	"fmt"
	"net"
	"os/user"

	"go.podman.io/storage/pkg/unshare"
	"golang.org/x/sys/unix"
)

// chroot to the given path and do some important initialization beforehand
func chroot(path string) (err error) {
	// NOTE: this must happen before the chroot as IsRootless needs access to /proc and
	// because IsRootless caches the result in memory this will make callers later work.
	_ = unshare.IsRootless()

	// initialize nss libraries in Glibc so that the dynamic libraries are loaded in the host
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
