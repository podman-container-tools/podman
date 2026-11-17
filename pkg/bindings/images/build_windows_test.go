package images

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/buildah/define"
)

func TestConvertAdditionalBuildContextsWindowsPath(t *testing.T) {
	additionalBuildContexts := map[string]*define.AdditionalBuildContext{
		"context1": {
			IsURL:           false,
			IsImage:         false,
			Value:           "C:\\test",
			DownloadedCache: "",
		},
	}

	convertAdditionalBuildContexts(additionalBuildContexts)

	assert.Equal(t, "/mnt/c/test", additionalBuildContexts["context1"].Value)
}
