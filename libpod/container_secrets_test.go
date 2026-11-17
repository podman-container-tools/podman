//go:build !remote && (linux || freebsd)

package libpod

import (
	"slices"
	"testing"

	"github.com/opencontainers/runtime-tools/generate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/pkg/secrets"
)

func TestInjectEnvSecrets(t *testing.T) {
	// Setup a minimal runtime with a secrets manager
	state, manager := getEmptySqliteState(t)
	defer state.Close()

	// ctx := context.Background()
	runtime := &Runtime{
		state:       state,
		lockManager: manager,
	}

	// Create a temporary directory for secrets
	secretsDir := t.TempDir()
	secretsManager, err := secrets.NewManager(secretsDir)
	require.NoError(t, err)
	runtime.secretsManager = secretsManager

	// Create a dummy secret
	secretName := "test-secret"
	secretData := []byte("secret-value")
	_, err = secretsManager.Store(secretName, secretData, "file", secrets.StoreOptions{
		DriverOpts: map[string]string{"path": secretsDir},
	})
	require.NoError(t, err)

	// Define test cases
	tests := []struct {
		name          string
		envSecrets    map[string]*secrets.Secret
		expectedEnv   map[string]string
		expectedError bool
	}{
		{
			name: "Map secret to same name",
			envSecrets: map[string]*secrets.Secret{
				"test-secret": {Name: "test-secret"},
			},
			expectedEnv: map[string]string{
				"test-secret": "secret-value",
			},
		},
		{
			name: "Map secret to different target",
			envSecrets: map[string]*secrets.Secret{
				"MY_TARGET": {Name: "test-secret"},
			},
			expectedEnv: map[string]string{
				"MY_TARGET": "secret-value",
			},
		},
		{
			name: "Missing secret",
			envSecrets: map[string]*secrets.Secret{
				"MISSING_TARGET": {Name: "missing-secret"},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock container with EnvSecrets
			c := &Container{
				config: &ContainerConfig{
					ContainerMiscConfig: ContainerMiscConfig{
						EnvSecrets: tt.envSecrets,
					},
				},
				runtime: runtime,
			}

			// Create a generator
			g, err := generate.New("linux")
			require.NoError(t, err)

			// Execute injectEnvSecrets
			err = c.injectEnvSecrets(&g)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for key, val := range tt.expectedEnv {
					found := slices.Contains(g.Config.Process.Env, key+"="+val)
					assert.True(t, found, "Expected env %s=%s not found", key, val)
				}
			}
		})
	}
}
