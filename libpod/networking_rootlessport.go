//go:build !remote && linux

package libpod

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"go.podman.io/common/libnetwork/slirp4netns" // RootlessKit port mapping only, not the removed slirp4netns backend
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

func (r *Runtime) setupRootlessPortMappingViaRLK(ctr *Container, netnsPath string, netStatus map[string]types.StatusBlock) error {
	// Only create pipes if they don't exist yet
	if ctr.rootlessPortSyncR == nil {
		var err error
		ctr.rootlessPortSyncR, ctr.rootlessPortSyncW, err = os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create rootless port sync pipe: %w", err)
		}
	}
	// Only defer close if not in PostConfigureNetNS mode to avoid double-close
	if !ctr.config.PostConfigureNetNS {
		defer errorhandling.CloseQuiet(ctr.rootlessPortSyncR)
	}
	return slirp4netns.SetupRootlessPortMappingViaRLK(&slirp4netns.SetupOptions{
		Config:                r.config,
		ContainerID:           ctr.ID(),
		Netns:                 netnsPath,
		Ports:                 ctr.convertPortMappings(),
		RootlessPortExitPipeR: ctr.rootlessPortSyncR,
	}, nil, netStatus)
}

// reloadRootlessRLKPortMapping will trigger a reload for the port mappings in the rootlessport process.
// This should only be called by network connect/disconnect and only as rootless.
func (c *Container) reloadRootlessRLKPortMapping() error {
	if len(c.config.PortMappings) == 0 {
		return nil
	}
	childIP := slirp4netns.GetRootlessPortChildIP(nil, c.state.NetworkStatus)
	logrus.Debugf("reloading rootless ports for container %s, childIP is %s", c.config.ID, childIP)

	conn, err := openUnixSocket(filepath.Join(c.runtime.config.Engine.TmpDir, "rp", c.config.ID))
	if err != nil {
		return fmt.Errorf("could not reload rootless port mappings, port forwarding may no longer work correctly: %w", err)
	}
	defer conn.Close()
	enc := json.NewEncoder(conn)
	err = enc.Encode(childIP)
	if err != nil {
		return fmt.Errorf("port reloading failed: %w", err)
	}
	b, err := io.ReadAll(conn)
	if err != nil {
		return fmt.Errorf("port reloading failed: %w", err)
	}
	data := string(b)
	if data != "OK" {
		return fmt.Errorf("port reloading failed: %s", data)
	}
	return nil
}
