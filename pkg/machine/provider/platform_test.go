package provider

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/pkg/machine/define"
)

func TestHasPermsForProvider(t *testing.T) {
	provider, err := Get()
	assert.NoError(t, err)
	assert.True(t, HasPermsForProvider(provider.VMType()))
}

func TestHasBadPerms(t *testing.T) {
	switch runtime.GOOS {
	case "darwin":
		assert.False(t, HasPermsForProvider(define.QemuVirt))
	case "windows":
		assert.False(t, HasPermsForProvider(define.QemuVirt))
	case "linux":
		assert.False(t, HasPermsForProvider(define.AppleHvVirt))
	}
}
