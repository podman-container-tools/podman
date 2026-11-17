//go:build !remote && (linux || freebsd)

package abi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/libimage"
)

// This is really intended to verify what happens with a
// nil pointer in layer.Created, but we'll just sanity
// check round tripping 42.
func TestToDomainHistoryLayer(t *testing.T) {
	var layer libimage.ImageHistory
	layer.Size = 42
	newLayer := toDomainHistoryLayer(&layer)
	assert.Equal(t, layer.Size, newLayer.Size)
}
