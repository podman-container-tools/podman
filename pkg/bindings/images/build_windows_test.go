//go:build windows

package images

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/internal/remote_build_helpers"
	"go.podman.io/podman/v6/pkg/domain/entities/types"
)

func TestPrepareSecretsWithLocalAPIContext(t *testing.T) {
	hostContext := t.TempDir()
	options := types.BuildOptions{ClientContextDirectory: hostContext}
	options.ContextDirectory = "/mnt/c/context"
	t.Setenv("PODMAN_TEST_BUILD_SECRET", "secret content")

	manager := remote_build_helpers.NewTempFileManager()
	defer manager.Cleanup()

	secrets, tarContent, err := prepareSecrets(
		[]string{"id=test,env=PODMAN_TEST_BUILD_SECRET"},
		options.ClientContextDirectory,
		manager,
	)
	require.NoError(t, err)
	require.Len(t, secrets, 1)
	require.Len(t, tarContent, 1)

	assert.Equal(t, hostContext, filepath.Dir(tarContent[0]))
	assert.Equal(t, "id=test,src="+filepath.Base(tarContent[0]), secrets[0])
	contents, err := os.ReadFile(tarContent[0])
	require.NoError(t, err)
	assert.Equal(t, "secret content", string(contents))
}
