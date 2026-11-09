package images

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/buildah/define"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/domain/entities/types"
)

// The local build API is sent the context directory as the server sees it: RemoteContextDirectory
// when localapi translated it, otherwise ContextDirectory as given.
func TestPrepareLocalRequestBodyContextDirectory(t *testing.T) {
	translated := types.BuildOptions{RemoteContextDirectory: "/mnt/c/Users/me/project"}
	translated.ContextDirectory = `C:\Users\me\project`
	direct := types.BuildOptions{}
	direct.ContextDirectory = "/srv/project"

	tests := []struct {
		name    string
		options types.BuildOptions
		want    string
	}{
		{name: "translated by localapi", options: translated, want: "/mnt/c/Users/me/project"},
		{name: "server path passed directly", options: direct, want: "/srv/project"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, err := prepareLocalRequestBody(context.Background(), &RequestParts{Params: url.Values{}}, nil, tt.options)
			require.NoError(t, err)
			assert.Equal(t, tt.want, parts.Params.Get("localcontextdir"))
		})
	}
}

func TestBuildSecrets(t *testing.T) {
	fixture := t.TempDir()
	contextDir := filepath.Join(fixture, "context")
	require.NoError(t, os.Mkdir(contextDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(contextDir, "Containerfile"), []byte("FROM scratch\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(contextDir, "ignored"), []byte("ignored"), 0o600))
	secretFile := filepath.Join(fixture, "secret")
	require.NoError(t, os.WriteFile(secretFile, []byte("file secret"), 0o600))
	t.Setenv("PODMAN_TEST_BUILD_SECRET", "env secret")
	t.Setenv("TMPDIR", filepath.Join(fixture, "missing"))

	remote, secrets, err := prepareSecrets([]string{
		"id=file,type=file,src=" + secretFile,
		"id=env,env=PODMAN_TEST_BUILD_SECRET",
	})
	require.NoError(t, err)
	require.Len(t, remote, 2)
	require.Len(t, secrets, 2)
	for i, option := range remote {
		id, name, found := strings.Cut(option, ",src=")
		require.True(t, found)
		assert.Equal(t, secrets[i].name, name)
		assert.Equal(t, filepath.Base(name), name)
		assert.Equal(t, map[string]string{"id=file": "file secret", "id=env": "env secret"}[id], string(secrets[i].contents))
	}
	for _, local := range []bool{false, true} {
		var body io.ReadCloser
		if local {
			options := types.BuildOptions{RemoteContextDirectory: "/server/context"}
			parts, err := prepareLocalRequestBody(t.Context(), &RequestParts{Params: url.Values{}}, &BuildFilePaths{secrets: secrets}, options)
			require.NoError(t, err)
			assert.Equal(t, "/server/context", parts.Params.Get("localcontextdir"))
			body = parts.Body
		} else {
			body, err = nTar([]string{"*", "!Containerfile"}, secrets, contextDir)
			require.NoError(t, err)
		}
		defer body.Close()
		files := readBuildTar(t, body)
		require.NoError(t, body.Close())
		for _, secret := range secrets {
			assert.Equal(t, string(secret.contents), files[secret.name])
		}
		if local {
			assert.Len(t, files, 2, "the shared context must not be uploaded")
		} else {
			assert.Len(t, files, 3)
			assert.Equal(t, "FROM scratch\n", files["Containerfile"])
		}
	}
	for directory, count := range map[string]int{fixture: 2, contextDir: 2} {
		entries, err := os.ReadDir(directory)
		require.NoError(t, err)
		assert.Len(t, entries, count, "unexpected files in %s", directory)
	}
}

func readBuildTar(t *testing.T, body io.Reader) map[string]string {
	t.Helper()
	reader, err := gzip.NewReader(body)
	require.NoError(t, err)
	defer reader.Close()
	tr := tar.NewReader(reader)
	files := map[string]string{}
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		contents, err := io.ReadAll(tr)
		require.NoError(t, err)
		files[header.Name] = string(contents)
		if strings.HasPrefix(header.Name, "podman-build-secret-") {
			assert.EqualValues(t, 0o600, header.Mode)
		}
	}
	_, err = io.Copy(io.Discard, reader)
	require.NoError(t, err)
	return files
}

func TestBuildFromServerContextSecretsUnsupported(t *testing.T) {
	options := types.BuildOptions{}
	options.CommonBuildOpts = &define.CommonBuildOptions{Secrets: []string{"id=secret"}}
	_, err := BuildFromServerContext(t.Context(), nil, options)
	var versionErr *bindings.APIVersionError
	require.ErrorAs(t, err, &versionErr)
	assert.ErrorContains(t, err, "6.3.0")
}

func TestBuildMatchIID(t *testing.T) {
	assert.True(t, iidRegex.MatchString("a883dafc480d466ee04e0d6da986bd78eb1fdd2178d04693723da3a8f95d42f4"))
	assert.True(t, iidRegex.MatchString("3da3a8f95d42"))
	assert.False(t, iidRegex.MatchString("3da3"))
}

func TestBuildNotMatchStatusMessage(t *testing.T) {
	assert.False(t, iidRegex.MatchString("Copying config a883dafc480d466ee04e0d6da986bd78eb1fdd2178d04693723da3a8f95d42f4"))
}

// Windows host paths are only rewritten when the client runs on Windows or
// inside a WSL/Hyper-V guest, so the drive letter case is covered separately in
// build_windows_test.go. The values below are left alone on every platform.
func TestConvertAdditionalBuildContexts(t *testing.T) {
	additionalBuildContexts := map[string]*define.AdditionalBuildContext{
		"context2": {
			IsURL:           false,
			IsImage:         false,
			Value:           "/test",
			DownloadedCache: "",
		},
		"context3": {
			IsURL:           true,
			IsImage:         false,
			Value:           "https://a.com/b.tar",
			DownloadedCache: "",
		},
		"context4": {
			IsURL:           false,
			IsImage:         true,
			Value:           "quay.io/a/b:c",
			DownloadedCache: "",
		},
	}

	convertAdditionalBuildContexts(additionalBuildContexts)

	expectedGuestValues := map[string]string{
		"context2": "/test",
		"context3": "https://a.com/b.tar",
		"context4": "quay.io/a/b:c",
	}

	for key, value := range additionalBuildContexts {
		assert.Equal(t, expectedGuestValues[key], value.Value)
	}
}
