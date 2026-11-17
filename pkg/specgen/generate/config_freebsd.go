//go:build !remote

package generate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"golang.org/x/sys/unix"
	"tags.cncf.io/container-device-interface/pkg/cdi"
)

// DevicesFromPath computes a list of devices
func DevicesFromPath(g *generate.Generator, devicePath string, config *config.Config) error {
	if isCDIDevice(devicePath) {
		registry, err := cdi.NewCache(
			cdi.WithSpecDirs(config.Engine.CdiSpecDirs.Get()...),
			cdi.WithAutoRefresh(false),
		)
		if err != nil {
			return fmt.Errorf("creating CDI registry: %w", err)
		}
		if err := registry.Refresh(); err != nil {
			logrus.Debugf("The following error was triggered when refreshing the CDI registry: %v", err)
		}
		if _, err = registry.InjectDevices(g.Config, devicePath); err != nil {
			return fmt.Errorf("setting up CDI devices: %w", err)
		}
		return nil
	}
	devs := strings.Split(devicePath, ":")
	resolvedDevicePath := devs[0]
	// check if it is a symbolic link
	if src, err := os.Lstat(resolvedDevicePath); err == nil && src.Mode()&os.ModeSymlink == os.ModeSymlink {
		if linkedPathOnHost, err := filepath.EvalSymlinks(resolvedDevicePath); err == nil {
			resolvedDevicePath = linkedPathOnHost
		}
	}
	st, err := os.Stat(resolvedDevicePath)
	if err != nil {
		return err
	}
	if st.IsDir() {
		// For devfs, we need to add the directory as well
		if err := addDevice(g, resolvedDevicePath); err != nil {
			return err
		}

		found := false
		src := resolvedDevicePath
		dest := src
		var devmode string
		if len(devs) > 1 {
			if len(devs[1]) > 0 && devs[1][0] == '/' {
				dest = devs[1]
			} else {
				devmode = devs[1]
			}
		}
		if len(devs) > 2 {
			if devmode != "" {
				return fmt.Errorf("invalid device specification %s: %w", devicePath, unix.EINVAL)
			}
			devmode = devs[2]
		}

		// mount the internal devices recursively
		if err := filepath.WalkDir(resolvedDevicePath, func(dpath string, d fs.DirEntry, _ error) error {
			if d.Type()&os.ModeDevice == os.ModeDevice {
				found = true
				device := fmt.Sprintf("%s:%s", dpath, filepath.Join(dest, strings.TrimPrefix(dpath, src)))
				if devmode != "" {
					device = fmt.Sprintf("%s:%s", device, devmode)
				}
				if err := addDevice(g, device); err != nil {
					return fmt.Errorf("failed to add %s device: %w", dpath, err)
				}
			}
			return nil
		}); err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("no devices found in %s: %w", devicePath, unix.EINVAL)
		}
		return nil
	}
	return addDevice(g, strings.Join(append([]string{resolvedDevicePath}, devs[1:]...), ":"))
}

func addDevice(g *generate.Generator, device string) error {
	src, dst, permissions, err := ParseDevice(device)
	if err != nil {
		return err
	}
	if src != dst {
		return fmt.Errorf("container device must be the same as host device on FreeBSD")
	}
	mode := 0
	if strings.Contains(permissions, "r") {
		mode |= unix.S_IRUSR
	}
	if strings.Contains(permissions, "w") {
		mode |= unix.S_IWUSR
	}
	// Find the devfs mount so that we can add rules to expose the device
	for k, m := range g.Config.Mounts {
		if m.Type == "devfs" {
			if dev, ok := strings.CutPrefix(src, "/dev/"); ok {
				m.Options = append(m.Options,
					fmt.Sprintf("rule=path %s unhide mode %04o", dev, mode))
			} else {
				return fmt.Errorf("expected device to start with \"/dev\": %v", dev)
			}
			g.Config.Mounts[k] = m
			return nil
		}
	}
	return fmt.Errorf("devfs not found in generator")
}
