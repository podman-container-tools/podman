//go:build (amd64 && !windows) || (arm64 && !windows)

package define

import "testing"

func TestParseVMType(t *testing.T) {
	type fields struct {
		input    string
		fallback VMType
	}

	tests := []struct {
		name   string
		fields fields
		want   VMType
	}{
		{
			name:   "Qemu input",
			fields: fields{"qemu", QemuVirt},
			want:   QemuVirt,
		},
		{
			name:   "Applehv input",
			fields: fields{"applehv", QemuVirt},
			want:   AppleHvVirt,
		},
		{
			name:   "Hyperv input",
			fields: fields{"hyperv", QemuVirt},
			want:   HyperVVirt,
		},
		{
			name:   "WSL input",
			fields: fields{"wsl", QemuVirt},
			want:   WSLVirt,
		},
		{
			name:   "Qemu empty fallback",
			fields: fields{"", QemuVirt},
			want:   QemuVirt,
		},
		{
			name:   "Invalid input",
			fields: fields{"riscv", AppleHvVirt},
			want:   UnknownVirt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := ParseVMType(tt.fields.input, tt.fields.fallback); got != tt.want {
				t.Errorf("ParseVMType(%s, %v) = %v, want %v", tt.fields.input, tt.fields.fallback, got, tt.want)
			}
		})
	}
}

func TestVMType_ImageFormat(t *testing.T) {
	tests := []struct {
		name string
		v    VMType
		want ImageFormat
	}{
		{
			name: "wsl",
			v:    WSLVirt,
			want: Tar,
		},
		{
			name: "applehv",
			v:    AppleHvVirt,
			want: Raw,
		},
		{
			name: "qemu",
			v:    QemuVirt,
			want: Qcow,
		},
		{
			name: "hyperv",
			v:    HyperVVirt,
			want: Vhdx,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.ImageFormat(); got != tt.want {
				t.Errorf("ImageFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}
