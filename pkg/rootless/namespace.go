package rootless

import (
	"fmt"
	"os"
	"strings"
)

// RootlessNamespaceInfo holds diagnostic information about the rootless namespace context.
type RootlessNamespaceInfo struct {
	IsRootless      bool   // Whether the process is executing in a rootless context
	HasJoinedNS     bool   // Whether a joined user/mount namespace was active
	JoinSource      string // Join source: "pause.pid", "ns_handles", or other
	MountNS         string // Current mount namespace (e.g. "mnt:[4026531840]")
	UserNS          string // Current user namespace (e.g. "user:[4026531837]")
	PausePID        int    // PID of pause process if configured/found
	PausePIDRunning bool   // Whether the pause PID process is currently running
	PauseMountNS    string // Mount namespace of the pause PID
	PausePIDPath    string // Path to pause.pid file
	NSHandlesPath   string // Path to ns_handles file
}

// GetRootlessNamespaceInfo collects diagnostic information regarding active rootless user
// and mount namespaces.
func GetRootlessNamespaceInfo(stateDir string) *RootlessNamespaceInfo {
	info := &RootlessNamespaceInfo{
		IsRootless: IsRootless() || os.Getenv("_CONTAINERS_ROOTLESS_UID") != "" || os.Getenv("_CONTAINERS_USERNS_CONFIGURED") != "",
	}
	if !info.IsRootless {
		return info
	}

	getPlatformNamespaceInfo(stateDir, info)
	return info
}

// ContextString returns a formatted diagnostic string summarizing the rootless namespace context.
func (info *RootlessNamespaceInfo) ContextString(isFilesystemErr bool) string {
	if info == nil || !info.IsRootless {
		return ""
	}

	var parts []string
	if info.MountNS != "" {
		parts = append(parts, fmt.Sprintf("mount ns: %s", info.MountNS))
	}
	if info.UserNS != "" {
		parts = append(parts, fmt.Sprintf("user ns: %s", info.UserNS))
	}
	if info.PausePID > 0 {
		status := "not running"
		if info.PausePIDRunning {
			status = "running"
		}
		parts = append(parts, fmt.Sprintf("pause PID: %d (%s)", info.PausePID, status))
	}
	if info.JoinSource != "" {
		parts = append(parts, fmt.Sprintf("join source: %s", info.JoinSource))
	}
	if info.PauseMountNS != "" && info.MountNS != "" && info.PauseMountNS == info.MountNS {
		parts = append(parts, "matches pause process mount ns")
	}

	var prefix string
	if info.HasJoinedNS || info.PausePID > 0 || info.JoinSource != "" {
		prefix = "joined rootless mount namespace active"
	} else {
		prefix = "rootless mode active (joined namespace: inactive)"
	}

	var details string
	if len(parts) > 0 {
		details = fmt.Sprintf("%s: %s", prefix, strings.Join(parts, ", "))
	} else {
		details = prefix
	}

	if isFilesystemErr || info.HasJoinedNS {
		details += "; if this filesystem error is caused by a stale mount view or device re-enumeration, try 'podman system migrate' or stopping the pause process"
	}

	return details
}
