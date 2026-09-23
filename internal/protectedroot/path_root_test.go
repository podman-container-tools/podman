package protectedroot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathWithoutProtection(t *testing.T) {
	path := "/var/lib/containers/storage/overlay/abc/merged"
	pr := NewPathRoot(path)
	assert.Equal(t, path, pr.PathWithoutProtection())
}
