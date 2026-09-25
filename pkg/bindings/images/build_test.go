package images

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/buildah/define"
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
