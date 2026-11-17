package specgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitVolumeString(t *testing.T) {
	tests := []struct {
		name   string
		volume string
		expect []string
	}{
		// relative host paths
		{
			name:   "relative host path",
			volume: "./hello:/container",
			expect: []string{"./hello", "/container"},
		},
		{
			name:   "relative host path with options",
			volume: "./hello:/container:ro",
			expect: []string{"./hello", "/container", "ro"},
		},
		// absolute host path
		{
			name:   "absolute host path",
			volume: "C:\\hello:/container",
			expect: []string{"C:\\hello", "/container"},
		},
		{
			name:   "absolute host path with option",
			volume: "C:\\hello:/container:ro",
			expect: []string{"C:\\hello", "/container", "ro"},
		},
		{
			name:   "absolute host path with option",
			volume: "C:\\hello:/container:ro",
			expect: []string{"C:\\hello", "/container", "ro"},
		},
		{
			name:   "absolute extended host path",
			volume: `\\?\C:\hello:/container`,
			expect: []string{`\\?\C:\hello`, "/container"},
		},
		// volume source
		{
			name:   "volume without option",
			volume: "vol-name:/container",
			expect: []string{"vol-name", "/container"},
		},
		{
			name:   "volume with option",
			volume: "vol-name:/container:ro",
			expect: []string{"vol-name", "/container", "ro"},
		},
		{
			name:   "single letter volume without option",
			volume: "a:/container",
			expect: []string{"a", "/container"},
		},
		{
			name:   "single letter volume with option",
			volume: "a:/container:ro",
			expect: []string{"a", "/container", "ro"},
		},
	}

	t.Run("shouldResolveWinPaths() returns true", func(t *testing.T) {
		assert.True(t, shouldResolveWinPaths())
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := SplitVolumeString(tt.volume)

			assert.Equal(t, tt.expect, parts, tt.name)
		})
	}
}
