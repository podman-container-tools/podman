package machine

import (
	"fmt"
	"strings"

	"go.podman.io/podman/v6/pkg/machine/ignition"
	"go.podman.io/podman/v6/pkg/systemd/parser"
)

// fcosDirSymlinks maps FCOS root-level symlinks to their real paths.
// FCOS symlinks several top-level dirs into /var for the read-only rootfs.
// systemd rejects a mount unit whose Where= traverses a symlink, so we
// resolve these before writing the ignition unit.
var fcosDirSymlinks = map[string]string{
	"/home": "/var/home",
	"/mnt":  "/var/mnt",
	"/opt":  "/var/opt",
	"/root": "/var/roothome",
	"/srv":  "/var/srv",
}

// canonicalizeFCOSMountTarget returns the canonical path for target by
// substituting any known FCOS root-level symlink prefix.
func canonicalizeFCOSMountTarget(target string) string {
	for link, real := range fcosDirSymlinks {
		if target == link || strings.HasPrefix(target, link+"/") {
			return real + target[len(link):]
		}
	}
	return target
}

// GenerateSystemDFilesForVirtiofsMounts generates the systemd unit files needed
// to mount virtiofs volumes inside a FCOS guest VM. It is shared between the
// AppleHV, LibKrun, and QEMU providers.
//
// Mounting in FCOS with virtiofs is a bit of a dance.  we need a unit file for
// the mount, a unit file for automatic mounting on boot, and a "preparatory"
// service file that disables FCOS security, performs the mkdir of the mount
// point, and then re-enables security. This must be done for each mount.
func GenerateSystemDFilesForVirtiofsMounts(mounts []VirtIoFs) ([]ignition.Unit, error) {
	unitFiles := make([]ignition.Unit, 0, len(mounts)+2)
	for _, mnt := range mounts {
		// Create mount unit for each mount
		mountUnit := parser.NewUnitFile()
		mountUnit.Add("Mount", "What", "%s")
		mountUnit.Add("Mount", "Where", "%s")
		mountUnit.Add("Mount", "Type", "virtiofs")
		mountUnit.Add("Mount", "Options", fmt.Sprintf("context=\"%s\"", NFSSELinuxContext))
		mountUnit.Add("Install", "WantedBy", "local-fs.target")
		mountUnitFile, err := mountUnit.ToString()
		if err != nil {
			return nil, err
		}

		// Use the canonical path so systemd accepts the Where= value.
		// On FCOS /home is a symlink to var/home; systemd rejects non-canonical paths.
		canonicalTarget := canonicalizeFCOSMountTarget(mnt.Target)
		virtiofsMount := ignition.Unit{
			Enabled:  ignition.BoolToPtr(true),
			Name:     fmt.Sprintf("%s.mount", parser.PathEscape(canonicalTarget)),
			Contents: ignition.StrToPtr(fmt.Sprintf(mountUnitFile, mnt.Tag, canonicalTarget)),
		}

		unitFiles = append(unitFiles, virtiofsMount)
	}

	// This is a way to workaround the FCOS limitation of creating directories
	// at the rootfs / and then mounting to them.
	immutableRootOff := parser.NewUnitFile()
	immutableRootOff.Add("Unit", "Description", "Allow systemd to create mount points on /")
	immutableRootOff.Add("Unit", "DefaultDependencies", "no")

	immutableRootOff.Add("Service", "Type", "oneshot")
	immutableRootOff.Add("Service", "ExecStart", "chattr -i /")

	immutableRootOff.Add("Install", "WantedBy", "local-fs-pre.target")
	immutableRootOffFile, err := immutableRootOff.ToString()
	if err != nil {
		return nil, err
	}

	immutableRootOffUnit := ignition.Unit{
		Contents: ignition.StrToPtr(immutableRootOffFile),
		Name:     "immutable-root-off.service",
		Enabled:  ignition.BoolToPtr(true),
	}
	unitFiles = append(unitFiles, immutableRootOffUnit)

	immutableRootOn := parser.NewUnitFile()
	immutableRootOn.Add("Unit", "Description", "Set / back to immutable after mounts are done")
	immutableRootOn.Add("Unit", "DefaultDependencies", "no")
	immutableRootOn.Add("Unit", "After", "local-fs.target")

	immutableRootOn.Add("Service", "Type", "oneshot")
	immutableRootOn.Add("Service", "ExecStart", "chattr +i /")

	immutableRootOn.Add("Install", "WantedBy", "local-fs.target")
	immutableRootOnFile, err := immutableRootOn.ToString()
	if err != nil {
		return nil, err
	}

	immutableRootOnUnit := ignition.Unit{
		Contents: ignition.StrToPtr(immutableRootOnFile),
		Name:     "immutable-root-on.service",
		Enabled:  ignition.BoolToPtr(true),
	}
	unitFiles = append(unitFiles, immutableRootOnUnit)

	return unitFiles, nil
}
