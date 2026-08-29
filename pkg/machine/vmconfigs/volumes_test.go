//go:build !windows

package vmconfigs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitVolume(t *testing.T) {
	tests := []struct {
		name         string
		idx          int
		volume       string
		wantTag      string
		wantSource   string
		wantTarget   string
		wantReadonly bool
		wantSecModel string
	}{
		{"read-only option", 0, "/host:/guest:ro", "vol0", "/host", "/guest", true, "none"},
		{"read-write option", 1, "/host:/guest:rw", "vol1", "/host", "/guest", false, "none"},
		{"security model option", 2, "/host:/guest:security_model=mapped", "vol2", "/host", "/guest", false, "mapped"},
		{"combined options", 0, "/host:/guest:ro,security_model=none", "vol0", "/host", "/guest", true, "none"},
		{"no options uses defaults", 3, "/host:/guest", "vol3", "/host", "/guest", false, "none"},
		{"source only targets itself", 0, "/host", "vol0", "/host", "/host", false, "none"},
		{"unknown option is ignored", 0, "/host:/guest:bogus", "vol0", "/host", "/guest", false, "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag, source, target, readonly, securityModel := SplitVolume(tt.idx, tt.volume)
			assert.Equal(t, tt.wantTag, tag)
			assert.Equal(t, tt.wantSource, source)
			assert.Equal(t, tt.wantTarget, target)
			assert.Equal(t, tt.wantReadonly, readonly)
			assert.Equal(t, tt.wantSecModel, securityModel)
		})
	}
}
