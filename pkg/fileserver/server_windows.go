package fileserver

import (
	"fmt"

	"github.com/linuxkit/virtsock/pkg/hvsock"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/fileserver/plan9"
)

// Start serving the given shares on Windows HVSocks for use by a Hyper-V VM.
// Mounts is formatted as a map of directory to be shared to vsock GUID.
// The vsocks used must already be defined before StartShares is called; it's
// expected that the vsocks will be created and torn down by the program calling
// gvproxy.
// TODO: The map here probably doesn't make sense.
// In the future, possibly accept a struct instead, so we can accept things
// other than a vsock and support non-Windows OSes.
func StartShares(mounts map[string]string) (defErr error) {
	for path, guid := range mounts {
		service, err := hvsock.GUIDFromString(guid)
		if err != nil {
			return fmt.Errorf("parsing vsock guid %s: %w", guid, err)
		}

		listener, err := hvsock.Listen(hvsock.Addr{
			VMID:      hvsock.GUIDWildcard,
			ServiceID: service,
		})
		if err != nil {
			return fmt.Errorf("retrieving listener for vsock %s: %w", guid, err)
		}

		logrus.Debugf("Going to serve directory %s on vsock %s", path, guid)

		server, err := plan9.New9pServer(listener, path)
		if err != nil {
			return fmt.Errorf("serving directory %s on vsock %s: %w", path, guid, err)
		}
		defer func() {
			if defErr != nil {
				if err := server.Stop(); err != nil {
					logrus.Errorf("Error stopping 9p server: %v", err)
				}
			}
		}()

		serverDir := path

		go func() {
			if err := server.WaitForError(); err != nil {
				logrus.Errorf("Error from 9p server for %s: %v", serverDir, err)
			} else {
				// We do not expect server exits - this should
				// run until the program exits.
				logrus.Warnf("9p server for %s exited without error", serverDir)
			}
		}()
	}

	return nil
}
