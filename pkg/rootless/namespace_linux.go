//go:build linux

package rootless

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func getPlatformNamespaceInfo(stateDir string, info *RootlessNamespaceInfo) {
	// Read current mount and user namespace identifiers
	if mntLink, err := os.Readlink("/proc/self/ns/mnt"); err == nil {
		info.MountNS = mntLink
	}
	if userLink, err := os.Readlink("/proc/self/ns/user"); err == nil {
		info.UserNS = userLink
	}

	// Check if in a non-initial user namespace
	const userNsInitIno uint64 = 0xEFFF_FFFD
	var st unix.Stat_t
	if err := unix.Stat("/proc/self/ns/user", &st); err == nil {
		if st.Ino != userNsInitIno {
			info.HasJoinedNS = true
		}
	}
	if os.Getenv("_CONTAINERS_USERNS_CONFIGURED") != "" {
		info.HasJoinedNS = true
	}

	// Check join source and pause PID from environment if set by C init
	if src := os.Getenv("_PODMAN_JOIN_SOURCE"); src != "" {
		info.JoinSource = src
		info.HasJoinedNS = true
	}
	if pidStr := os.Getenv("_PODMAN_PAUSE_PID"); pidStr != "" {
		if p, err := strconv.Atoi(pidStr); err == nil && p > 0 {
			info.PausePID = p
		}
	}

	// Resolve state directory if not provided
	if stateDir == "" {
		if xdgRuntime := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntime != "" {
			stateDir = filepath.Join(xdgRuntime, "libpod", "tmp")
		}
	}

	// Inspect state directory for pause.pid and ns_handles
	if stateDir != "" {
		pausePidFile := GetPausePidPath(stateDir)
		info.PausePIDPath = pausePidFile
		if info.PausePID == 0 {
			if data, err := os.ReadFile(pausePidFile); err == nil {
				if p, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && p > 0 {
					info.PausePID = p
				}
			}
		}

		nsHandlesFile := GetNamespaceHandlesPath(stateDir)
		if _, err := os.Stat(nsHandlesFile); err == nil {
			info.NSHandlesPath = nsHandlesFile
			if info.JoinSource == "" {
				info.JoinSource = "ns_handles"
			}
		} else if info.JoinSource == "" && info.PausePID > 0 {
			info.JoinSource = "pause.pid"
		}
	}

	// If pause PID was discovered, inspect its state
	if info.PausePID > 0 {
		if unix.Kill(info.PausePID, 0) == nil {
			info.PausePIDRunning = true
		}
		if pauseMnt, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/mnt", info.PausePID)); err == nil {
			info.PauseMountNS = pauseMnt
			if info.MountNS != "" && pauseMnt == info.MountNS {
				info.HasJoinedNS = true
				if info.JoinSource == "" {
					info.JoinSource = "pause.pid"
				}
			}
		}
	}
}
