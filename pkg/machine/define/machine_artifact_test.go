package define

import (
	"testing"
)

func Test_artifact_String(t *testing.T) {
	tests := []struct {
		name string
		a    Artifact
		want string
	}{
		{
			name: "qemu",
			a:    Qemu,
			want: "qemu",
		},
		{
			name: "hyperv",
			a:    HyperV,
			want: "hyperv",
		},
		{
			name: "applehv",
			a:    AppleHV,
			want: "applehv",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
