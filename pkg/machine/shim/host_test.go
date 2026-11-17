package shim

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_validateDestinationPaths(t *testing.T) {
	tests := []struct {
		name    string
		dest    string
		wantErr bool
	}{
		{
			name:    "Expect fail - /tmp",
			dest:    "/tmp",
			wantErr: true,
		},
		{
			name:    "Expect fail trailing /",
			dest:    "/tmp/",
			wantErr: true,
		},
		{
			name:    "Expect fail double /",
			dest:    "//tmp",
			wantErr: true,
		},
		{
			name:    "/var should fail",
			dest:    "/var",
			wantErr: true,
		},
		{
			name:    "/etc should fail",
			dest:    "/etc",
			wantErr: true,
		},
		{
			name:    "/tmp subdir OK",
			dest:    "/tmp/foobar",
			wantErr: false,
		},
		{
			name:    "/foobar OK",
			dest:    "/foobar",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDestinationPaths(tt.dest)
			if tt.wantErr {
				assert.ErrorContainsf(t, err, "onsider another location or a subdirectory of an existing location", "illegal mount target")
			} else {
				assert.NoError(t, err, "mounts to subdirs or non-critical dirs should succeed")
			}
		})
	}
}
