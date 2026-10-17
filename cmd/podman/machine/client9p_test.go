//go:build linux && (amd64 || arm64)

package machine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient9pMountOptions(t *testing.T) {
	tests := []struct {
		name     string
		readOnly bool
		want     string
	}{
		{
			name:     "read-write mount options",
			readOnly: false,
			want:     "trans=fd,rfdno=3,wfdno=3,version=9p2000.L",
		},
		{
			name:     "read-only mount options",
			readOnly: true,
			want:     "trans=fd,rfdno=3,wfdno=3,version=9p2000.L,ro",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client9pMountOptions(tt.readOnly)
			assert.Equal(t, tt.want, got)
		})
	}
}
