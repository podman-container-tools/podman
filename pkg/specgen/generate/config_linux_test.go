//go:build !remote

package generate

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldMask(t *testing.T) {
	tests := []struct {
		mask       string
		unmask     []string
		shouldMask bool
	}{
		{"/proc/foo", []string{"all"}, false},
		{"/proc/foo", []string{"ALL"}, false},
		{"/proc/foo", []string{"/proc/foo"}, false},
		{"/proc/foo", []string{"/proc/*"}, false},
		{"/proc/foo", []string{"/proc/bar", "all"}, false},
		{"/proc/foo", []string{"/proc/f*"}, false},
		{"/proc/foo", []string{"/proc/b*"}, true},
		{"/proc/foo", []string{}, true},
	}
	for _, test := range tests {
		val := shouldMask(test.mask, test.unmask)
		assert.Equal(t, val, test.shouldMask)
	}
}

func TestIsGIDInSubgid(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "subgid")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// All values derived from these — no magic numbers
	const (
		username   = "testuser"
		rangeStart = 299000
		rangeCount = 1000
	)
	rangeEnd := rangeStart + rangeCount
	gidInside := rangeStart + 500 // 299500 — clearly inside
	gidOutside := rangeStart - 1  // 298999 — clearly outside
	baseContent := fmt.Sprintf("%s:%d:%d\n", username, rangeStart, rangeCount)

	tests := []struct {
		name     string
		gid      int
		content  string
		expected bool // true = GID is missing from subgid
	}{
		{
			name:     "GID inside range",
			gid:      gidInside,
			content:  baseContent,
			expected: false, // mapped, NOT missing
		},
		{
			name:     "GID outside range",
			gid:      gidOutside,
			content:  baseContent,
			expected: true, // NOT mapped, missing
		},
		{
			name:     "GID at range start boundary",
			gid:      rangeStart,
			content:  baseContent,
			expected: false, // boundary included, NOT missing
		},
		{
			name:     "GID at range end boundary",
			gid:      rangeEnd - 1,
			content:  baseContent,
			expected: false, // last valid GID, NOT missing
		},
		{
			name:     "GID explicitly added as single entry",
			gid:      gidOutside,
			content:  baseContent + fmt.Sprintf("%s:%d:1\n", username, gidOutside),
			expected: false, // explicitly mapped, NOT missing
		},
		{
			name:     "empty subgid file",
			gid:      gidOutside,
			content:  "",
			expected: true, // nothing mapped, missing
		},
		{
			name:     "wrong username in subgid",
			gid:      gidInside,
			content:  fmt.Sprintf("otheruser:%d:%d\n", rangeStart, rangeCount),
			expected: true, // mapped for different user, missing for testuser
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.WriteFile(tmpFile.Name(), []byte(tt.content), 0o644)
			assert.NoError(t, err)

			found := isGIDInSubgid(username, tt.gid, tmpFile.Name())
			assert.Equal(t, tt.expected, !found)
		})
	}
}
