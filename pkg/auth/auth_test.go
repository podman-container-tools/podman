//go:build !windows

package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/pkg/docker/config"
	"go.podman.io/image/v5/types"
)

const largeAuthFile = `{"auths":{
	"docker.io/vendor": {"auth": "ZG9ja2VyOnZlbmRvcg=="},
	"https://index.docker.io/v1": {"auth": "ZG9ja2VyOnRvcA=="},
	"quay.io/libpod": {"auth": "cXVheTpsaWJwb2Q="},
	"quay.io": {"auth": "cXVheTp0b3A="}
}}`

// Semantics of largeAuthFile
var largeAuthFileValues = map[string]types.DockerAuthConfig{
	"docker.io/vendor": {Username: "docker", Password: "vendor"},
	"docker.io":        {Username: "docker", Password: "top"},
	"quay.io/libpod":   {Username: "quay", Password: "libpod"},
	"quay.io":          {Username: "quay", Password: "top"},
}

// systemContextForAuthFile returns a types.SystemContext with AuthFilePath pointing
// to a temporary file with fileContents, or nil if fileContents is empty; and a cleanup
// function the caller must arrange to call.
func systemContextForAuthFile(t *testing.T, fileContents string) *types.SystemContext {
	if fileContents == "" {
		return nil
	}

	f, err := os.CreateTemp(t.TempDir(), "auth.json")
	require.NoError(t, err)
	path := f.Name()
	err = os.WriteFile(path, []byte(fileContents), 0o700)
	require.NoError(t, err)
	return &types.SystemContext{AuthFilePath: path}
}

// Test that GetCredentials() correctly parses what MakeXRegistryConfigHeader() produces
func TestMakeXRegistryConfigHeaderGetCredentialsRoundtrip(t *testing.T) {
	for _, tc := range []struct {
		name               string
		fileContents       string
		username, password string
		expectedOverride   *types.DockerAuthConfig
		expectedFileValues map[string]types.DockerAuthConfig
	}{
		{
			name:               "no data",
			fileContents:       "",
			username:           "",
			password:           "",
			expectedOverride:   nil,
			expectedFileValues: nil,
		},
		{
			name:               "file data",
			fileContents:       largeAuthFile,
			username:           "",
			password:           "",
			expectedOverride:   nil,
			expectedFileValues: largeAuthFileValues,
		},
		{
			name:               "file data + override",
			fileContents:       largeAuthFile,
			username:           "override-user",
			password:           "override-pass",
			expectedOverride:   &types.DockerAuthConfig{Username: "override-user", Password: "override-pass"},
			expectedFileValues: largeAuthFileValues,
		},
	} {
		sys := systemContextForAuthFile(t, tc.fileContents)
		headers, err := MakeXRegistryConfigHeader(sys, tc.username, tc.password)
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost, "/", nil)
		require.NoError(t, err, tc.name)
		for _, v := range headers.Values(xRegistryConfigHeader) {
			req.Header.Add(xRegistryConfigHeader, v)
		}

		override, resPath, err := GetCredentials(req)
		require.NoError(t, err, tc.name)
		defer RemoveAuthfile(resPath)
		if tc.expectedOverride == nil {
			assert.Nil(t, override, tc.name)
		} else {
			require.NotNil(t, override, tc.name)
			assert.Equal(t, *tc.expectedOverride, *override, tc.name)
		}
		for key, expectedAuth := range tc.expectedFileValues {
			auth, err := config.GetCredentials(&types.SystemContext{AuthFilePath: resPath}, key)
			require.NoError(t, err, tc.name)
			assert.Equal(t, expectedAuth, auth, "%s, key %s", tc.name, key)
		}
	}
}

// Test that GetCredentials() correctly parses what MakeXRegistryAuthHeader() produces
func TestMakeXRegistryAuthHeaderGetCredentialsRoundtrip(t *testing.T) {
	for _, tc := range []struct {
		name               string
		fileContents       string
		username, password string
		expectedOverride   *types.DockerAuthConfig
		expectedFileValues map[string]types.DockerAuthConfig
	}{
		{
			name:               "override",
			fileContents:       "",
			username:           "override-user",
			password:           "override-pass",
			expectedOverride:   &types.DockerAuthConfig{Username: "override-user", Password: "override-pass"},
			expectedFileValues: nil,
		},
		{
			name:               "file data",
			fileContents:       largeAuthFile,
			username:           "",
			password:           "",
			expectedFileValues: largeAuthFileValues,
		},
	} {
		sys := systemContextForAuthFile(t, tc.fileContents)
		headers, err := MakeXRegistryAuthHeader(sys, tc.username, tc.password)
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPost, "/", nil)
		require.NoError(t, err, tc.name)
		for _, v := range headers.Values(xRegistryAuthHeader) {
			req.Header.Set(xRegistryAuthHeader, v)
		}

		override, resPath, err := GetCredentials(req)
		require.NoError(t, err, tc.name)
		defer RemoveAuthfile(resPath)
		if tc.expectedOverride == nil {
			assert.Nil(t, override, tc.name)
		} else {
			require.NotNil(t, override, tc.name)
			assert.Equal(t, *tc.expectedOverride, *override, tc.name)
		}
		for key, expectedAuth := range tc.expectedFileValues {
			auth, err := config.GetCredentials(&types.SystemContext{AuthFilePath: resPath}, key)
			require.NoError(t, err, tc.name)
			assert.Equal(t, expectedAuth, auth, "%s, key %s", tc.name, key)
		}
	}
}

func TestMakeXRegistryConfigHeader(t *testing.T) {
	for _, tc := range []struct {
		name               string
		fileContents       string
		username, password string
		shouldErr          bool
		expectedContents   string
	}{
		{
			name:             "no data",
			fileContents:     "",
			username:         "",
			password:         "",
			expectedContents: "",
		},
		{
			name:         "invalid JSON",
			fileContents: "@invalid JSON",
			username:     "",
			password:     "",
			shouldErr:    true,
		},
		{
			name:         "file data",
			fileContents: largeAuthFile,
			username:     "",
			password:     "",
			expectedContents: `{
			"docker.io/vendor": {"username": "docker", "password": "vendor"},
			"docker.io": {"username": "docker", "password": "top"},
			"quay.io/libpod": {"username": "quay", "password": "libpod"},
			"quay.io": {"username": "quay", "password": "top"}
			}`,
		},
		{
			name:         "file data + override",
			fileContents: largeAuthFile,
			username:     "override-user",
			password:     "override-pass",
			expectedContents: `{
				"docker.io/vendor": {"username": "docker", "password": "vendor"},
				"docker.io": {"username": "docker", "password": "top"},
				"quay.io/libpod": {"username": "quay", "password": "libpod"},
				"quay.io": {"username": "quay", "password": "top"},
				"": {"username": "override-user", "password": "override-pass"}
				}`,
		},
	} {
		sys := systemContextForAuthFile(t, tc.fileContents)
		res, err := MakeXRegistryConfigHeader(sys, tc.username, tc.password)
		if tc.shouldErr {
			assert.Error(t, err, tc.name)
		} else {
			require.NoError(t, err, tc.name)
			if tc.expectedContents == "" {
				assert.Empty(t, res, tc.name)
			} else {
				require.Len(t, res, 1, tc.name)
				header, ok := res[xRegistryConfigHeader]
				require.True(t, ok, tc.name)
				decodedHeader, err := base64.URLEncoding.DecodeString(header[0])
				require.NoError(t, err, tc.name)
				// Don't test for a specific JSON representation, just for the expected contents.
				expected := map[string]any{}
				actual := map[string]any{}
				err = json.Unmarshal([]byte(tc.expectedContents), &expected)
				require.NoError(t, err, tc.name)
				err = json.Unmarshal(decodedHeader, &actual)
				require.NoError(t, err, tc.name)
				assert.Equal(t, expected, actual, tc.name)
			}
		}
	}
}

func TestMakeXRegistryAuthHeader(t *testing.T) {
	for _, tc := range []struct {
		name               string
		fileContents       string
		username, password string
		shouldErr          bool
		expectedContents   string
	}{
		{
			name:             "override",
			fileContents:     "",
			username:         "override-user",
			password:         "override-pass",
			expectedContents: `{"username": "override-user", "password": "override-pass"}`,
		},
		{
			name:         "invalid JSON",
			fileContents: "@invalid JSON",
			username:     "",
			password:     "",
			shouldErr:    true,
		},
		{
			name:         "file data",
			fileContents: largeAuthFile,
			username:     "",
			password:     "",
			expectedContents: `{
			"docker.io/vendor": {"username": "docker", "password": "vendor"},
			"docker.io": {"username": "docker", "password": "top"},
			"quay.io/libpod": {"username": "quay", "password": "libpod"},
			"quay.io": {"username": "quay", "password": "top"}
			}`,
		},
	} {
		sys := systemContextForAuthFile(t, tc.fileContents)
		res, err := MakeXRegistryAuthHeader(sys, tc.username, tc.password)
		if tc.shouldErr {
			assert.Error(t, err, tc.name)
		} else {
			require.NoError(t, err, tc.name)
			if tc.expectedContents == "" {
				assert.Empty(t, res, tc.name)
			} else {
				require.Len(t, res, 1, tc.name)
				header, ok := res[xRegistryAuthHeader]
				require.True(t, ok, tc.name)
				decodedHeader, err := base64.URLEncoding.DecodeString(header[0])
				require.NoError(t, err, tc.name)
				// Don't test for a specific JSON representation, just for the expected contents.
				expected := map[string]any{}
				actual := map[string]any{}
				err = json.Unmarshal([]byte(tc.expectedContents), &expected)
				require.NoError(t, err, tc.name)
				err = json.Unmarshal(decodedHeader, &actual)
				require.NoError(t, err, tc.name)
				assert.Equal(t, expected, actual, tc.name)
			}
		}
	}
}

func TestAuthConfigsToAuthFile(t *testing.T) {
	for _, tc := range []struct {
		name             string
		server           string
		shouldErr        bool
		expectedContains string
	}{
		{
			name:             "empty auth configs",
			server:           "",
			shouldErr:        false,
			expectedContains: "{}",
		},
		{
			name:             "registry with a namespace prefix",
			server:           "my-registry.local/username",
			shouldErr:        false,
			expectedContains: `"my-registry.local/username":`,
		},
		{
			name:             "URLs are interpreted as full registries",
			server:           "http://my-registry.local/username",
			shouldErr:        false,
			expectedContains: `"my-registry.local":`,
		},
		{
			name:             "the old-style docker registry URL is normalized",
			server:           "http://index.docker.io/v1/",
			shouldErr:        false,
			expectedContains: `"docker.io":`,
		},
		{
			name:             "docker.io vendor namespace",
			server:           "docker.io/vendor",
			shouldErr:        false,
			expectedContains: `"docker.io/vendor":`,
		},
	} {
		configs := map[string]types.DockerAuthConfig{}
		if tc.server != "" {
			configs[tc.server] = types.DockerAuthConfig{}
		}

		filePath, err := authConfigsToAuthFile(configs)

		if tc.shouldErr {
			assert.Error(t, err)
			assert.Empty(t, filePath)
		} else {
			assert.NoError(t, err)
			content, err := os.ReadFile(filePath)
			require.NoError(t, err)
			assert.Contains(t, string(content), tc.expectedContains)
			os.Remove(filePath)
		}
	}
}

func TestParseSingleAuthHeader(t *testing.T) {
	for _, tc := range []struct {
		input     string
		shouldErr bool
		expected  types.DockerAuthConfig
	}{
		{
			input:    "", // An empty (or missing) header
			expected: types.DockerAuthConfig{},
		},
		{
			input:    "null",
			expected: types.DockerAuthConfig{},
		},
		// Invalid JSON
		{input: "@", shouldErr: true},
		// Success
		{
			input:    base64.URLEncoding.EncodeToString([]byte(`{"username":"u1","password":"p1"}`)),
			expected: types.DockerAuthConfig{Username: "u1", Password: "p1"},
		},
	} {
		res, err := parseSingleAuthHeader(tc.input)
		if tc.shouldErr {
			assert.Error(t, err, tc.input)
		} else {
			require.NoError(t, err, tc.input)
			assert.Equal(t, tc.expected, res, tc.input)
		}
	}
}

func TestParseMultiAuthHeader(t *testing.T) {
	for _, tc := range []struct {
		input     string
		shouldErr bool
		expected  map[string]types.DockerAuthConfig
	}{
		// Empty header
		{input: "", expected: nil},
		// Empty JSON object {}=e30= base64-encoded
		{input: "e30=", expected: nil},
		// "null"
		{input: "null", expected: nil},
		// Invalid JSON
		{input: "@", shouldErr: true},
		// Success
		{
			input: base64.URLEncoding.EncodeToString([]byte(
				`{"https://index.docker.io/v1/":{"username":"u1","password":"p1"},` +
					`"quay.io/libpod":{"username":"u2","password":"p2"}}`)),
			expected: map[string]types.DockerAuthConfig{
				"https://index.docker.io/v1/": {Username: "u1", Password: "p1"},
				"quay.io/libpod":              {Username: "u2", Password: "p2"},
			},
		},
	} {
		res, err := parseMultiAuthHeader(tc.input)
		if tc.shouldErr {
			assert.Error(t, err, tc.input)
		} else {
			require.NoError(t, err, tc.input)
			assert.Equal(t, tc.expected, res, tc.input)
		}
	}
}
