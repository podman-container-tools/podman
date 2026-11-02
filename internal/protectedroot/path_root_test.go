package protectedroot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathWithoutProtection(t *testing.T) {
	path := "/var/lib/containers/storage/overlay/abc/merged"
	pr, err := NewPathRoot(path)
	assert.NoError(t, err)
	assert.Equal(t, path, pr.PathWithoutProtection())
}
