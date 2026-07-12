//go:build !remote

package libpod

import (
	"testing"
)

func TestHasSELinuxContextOption(t *testing.T) {
	tests := []struct {
		name    string
		options []string
		want    bool
	}{
		// Positive cases — each SELinux context prefix should be detected
		{name: "context=", options: []string{"context=system_u:object_r:container_t:s0"}, want: true},
		{name: "fscontext=", options: []string{"fscontext=system_u:object_r:container_t:s0"}, want: true},
		{name: "defcontext=", options: []string{"defcontext=system_u:object_r:container_t:s0"}, want: true},
		{name: "rootcontext=", options: []string{"rootcontext=system_u:object_r:container_t:s0"}, want: true},

		// Context option mixed with other options
		{name: "context= mixed", options: []string{"size=64m", "context=system_u:object_r:tmp_t:s0", "mode=1777"}, want: true},
		{name: "fscontext= mixed", options: []string{"noexec", "fscontext=system_u:object_r:tmp_t:s0"}, want: true},

		// Negative cases — no SELinux context option
		{name: "nil slice", options: nil, want: false},
		{name: "empty slice", options: []string{}, want: false},
		{name: "no SELinux option", options: []string{"size=64m", "mode=1777", "noexec"}, want: false},
		{name: "single non-SELinux option", options: []string{"nosuid"}, want: false},

		// Edge cases — partial prefix matches should NOT count
		{name: "partial prefix context", options: []string{"context"}, want: false},
		{name: "partial prefix fscontext", options: []string{"fscontext"}, want: false},
		{name: "partial prefix defcontext", options: []string{"defcontext"}, want: false},
		{name: "partial prefix rootcontext", options: []string{"rootcontext"}, want: false},

		// Related but different options should NOT match
		{name: "context without equals", options: []string{"context"}, want: false},
		{name: "seclabel", options: []string{"seclabel"}, want: false},
		{name: "label=disabled", options: []string{"label:disabled"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSELinuxContextOption(tt.options)
			if got != tt.want {
				t.Errorf("hasSELinuxContextOption(%v) = %v, want %v", tt.options, got, tt.want)
			}
		})
	}
}
