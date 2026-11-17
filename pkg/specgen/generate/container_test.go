//go:build !remote && (linux || freebsd)

package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseImageEnvs(t *testing.T) {
	tests := []struct {
		name      string
		imageEnvs []string
		want      map[string]string
		wantErr   bool
	}{
		{
			name: "no env",
			want: map[string]string{},
		},
		{
			name:      "single env",
			imageEnvs: []string{"TEST=1"},
			want: map[string]string{
				"TEST": "1",
			},
		},
		{
			name:      "multiple envs",
			imageEnvs: []string{"TEST=1", "ABC=b", "PATH=/bin"},
			want: map[string]string{
				"TEST": "1",
				"ABC":  "b",
				"PATH": "/bin",
			},
		},
		{
			name:      "invalid env without value",
			imageEnvs: []string{"HOST"},
			wantErr:   true,
		},
		{
			name:      "invalid env asterisk",
			imageEnvs: []string{"*"},
			wantErr:   true,
		},
		{
			name:      "invalid env no key",
			imageEnvs: []string{"=123"},
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseImageEnvs(tt.imageEnvs)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
