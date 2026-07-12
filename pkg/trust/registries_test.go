package trust

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAndMergeConfig(t *testing.T) {
	// Non-existent directory returns empty config without error.
	config, err := loadAndMergeConfig(filepath.Join(t.TempDir(), "nonexistent"))
	require.NoError(t, err)
	assert.Empty(t, config.Docker)
	assert.Nil(t, config.DefaultDocker)

	// Empty directory returns empty config.
	emptyDir := t.TempDir()
	config, err = loadAndMergeConfig(emptyDir)
	require.NoError(t, err)
	assert.Empty(t, config.Docker)
	assert.Nil(t, config.DefaultDocker)

	// Existing testdata directory returns valid config.
	config, err = loadAndMergeConfig("./testdata")
	require.NoError(t, err)
	assert.NotEmpty(t, config.Docker)
}
