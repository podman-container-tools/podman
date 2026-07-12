package images

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/buildah/define"
)

func TestBuildMatchIID(t *testing.T) {
	assert.True(t, iidRegex.MatchString("a883dafc480d466ee04e0d6da986bd78eb1fdd2178d04693723da3a8f95d42f4"))
	assert.True(t, iidRegex.MatchString("3da3a8f95d42"))
	assert.False(t, iidRegex.MatchString("3da3"))
}

func TestBuildNotMatchStatusMessage(t *testing.T) {
	assert.False(t, iidRegex.MatchString("Copying config a883dafc480d466ee04e0d6da986bd78eb1fdd2178d04693723da3a8f95d42f4"))
}

func TestConvertAdditionalBuildContexts(t *testing.T) {
	additionalBuildContexts := map[string]*define.AdditionalBuildContext{
		"context1": {
			IsURL:           false,
			IsImage:         false,
			Value:           "C:\\test",
			DownloadedCache: "",
		},
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
		"context1": "/mnt/c/test",
		"context2": "/test",
		"context3": "https://a.com/b.tar",
		"context4": "quay.io/a/b:c",
	}

	for key, value := range additionalBuildContexts {
		assert.Equal(t, expectedGuestValues[key], value.Value)
	}
}
