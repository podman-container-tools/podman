package rootless

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootlessNamespaceInfo_ContextString(t *testing.T) {
	tests := []struct {
		name               string
		info               *RootlessNamespaceInfo
		isFSErr            bool
		expectedSubstrings []string
		excludedSubstrings []string
	}{
		{
			name: "non-rootless returns empty",
			info: &RootlessNamespaceInfo{
				IsRootless: false,
			},
			isFSErr:            true,
			expectedSubstrings: nil,
			excludedSubstrings: []string{"rootless"},
		},
		{
			name: "rootless joined with pause PID and mount NS",
			info: &RootlessNamespaceInfo{
				IsRootless:      true,
				HasJoinedNS:     true,
				MountNS:         "mnt:[4026531840]",
				UserNS:          "user:[4026531837]",
				PausePID:        12345,
				PausePIDRunning: true,
				JoinSource:      "pause.pid",
				PauseMountNS:    "mnt:[4026531840]",
			},
			isFSErr: true,
			expectedSubstrings: []string{
				"joined rootless mount namespace active",
				"mount ns: mnt:[4026531840]",
				"user ns: user:[4026531837]",
				"pause PID: 12345 (running)",
				"join source: pause.pid",
				"matches pause process mount ns",
				"podman system migrate",
			},
		},
		{
			name: "rootless joined via ns_handles with stopped pause process",
			info: &RootlessNamespaceInfo{
				IsRootless:      true,
				HasJoinedNS:     true,
				MountNS:         "mnt:[4026532000]",
				PausePID:        54321,
				PausePIDRunning: false,
				JoinSource:      "ns_handles",
			},
			isFSErr: true,
			expectedSubstrings: []string{
				"joined rootless mount namespace active",
				"mount ns: mnt:[4026532000]",
				"pause PID: 54321 (not running)",
				"join source: ns_handles",
				"podman system migrate",
			},
		},
		{
			name: "rootless unjoined without filesystem error",
			info: &RootlessNamespaceInfo{
				IsRootless:  true,
				HasJoinedNS: false,
				MountNS:     "mnt:[4026531840]",
			},
			isFSErr: false,
			expectedSubstrings: []string{
				"rootless mode active (joined namespace: inactive)",
				"mount ns: mnt:[4026531840]",
			},
			excludedSubstrings: []string{
				"podman system migrate",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.info.ContextString(tc.isFSErr)
			for _, exp := range tc.expectedSubstrings {
				assert.Contains(t, result, exp)
			}
			for _, excl := range tc.excludedSubstrings {
				assert.NotContains(t, result, excl)
			}
		})
	}
}

func TestGetRootlessNamespaceInfo_StateDirDiscovery(t *testing.T) {
	tempDir := t.TempDir()
	pauseFile := filepath.Join(tempDir, "pause.pid")
	err := os.WriteFile(pauseFile, []byte("99999\n"), 0o600)
	require.NoError(t, err)

	nsFile := filepath.Join(tempDir, "ns_handles")
	err = os.WriteFile(nsFile, []byte("test_handle"), 0o600)
	require.NoError(t, err)

	t.Setenv("_CONTAINERS_ROOTLESS_UID", "1000")
	info := GetRootlessNamespaceInfo(tempDir)
	assert.True(t, info.IsRootless)
	assert.Equal(t, 99999, info.PausePID)
	assert.Equal(t, pauseFile, info.PausePIDPath)
	assert.Equal(t, nsFile, info.NSHandlesPath)
	assert.Contains(t, []string{"pause.pid", "ns_handles"}, info.JoinSource)

	ctxStr := info.ContextString(true)
	assert.True(t, strings.Contains(ctxStr, "pause PID: 99999"))
}
