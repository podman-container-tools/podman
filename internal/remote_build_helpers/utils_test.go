package remote_build_helpers

import (
	"os"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"
)

func TestSupportsLocalBuildSecrets(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{version: "0.0.0", want: false},
		{version: "5.6.0", want: false},
		{version: "6.2.9", want: false},
		{version: "6.3.0-dev", want: true},
		{version: "6.3.0", want: true},
		{version: "7.0.0", want: true},
	} {
		t.Run(tc.version, func(t *testing.T) {
			version := semver.MustParse(tc.version)
			assert.Equal(t, tc.want, SupportsLocalBuildSecrets(&version))
		})
	}
}

func TestTempFileManager(t *testing.T) {
	manager := NewTempFileManager()

	t.Run("CreateTempFileFromReader", func(t *testing.T) {
		content := "test content"
		r := strings.NewReader(content)

		filename, err := manager.CreateTempFileFromReader("", "podman-build-stdin-*", r)
		assert.NoError(t, err)
		assert.FileExists(t, filename)

		data, err := os.ReadFile(filename)
		assert.NoError(t, err)
		assert.Equal(t, content, string(data))

		manager.Cleanup()

		assert.NoFileExists(t, filename)
	})
}
