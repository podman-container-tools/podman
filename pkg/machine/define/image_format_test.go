package define

import "testing"

func TestImageFormat_Kind(t *testing.T) {
	tests := []struct {
		name string
		imf  ImageFormat
		want string
	}{
		{
			name: "vhdx",
			imf:  Vhdx,
			want: "vhdx",
		},
		{
			name: "qcow2",
			imf:  Qcow,
			want: "qcow2",
		},
		{
			name: "raw",
			imf:  Raw,
			want: "raw",
		},
		{
			name: "tar",
			imf:  Tar,
			want: "tar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.imf.Kind(); got != tt.want {
				t.Errorf("Kind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageFormat_KindWithCompression(t *testing.T) {
	tests := []struct {
		name string
		imf  ImageFormat
		want string
	}{
		{
			name: "vhdx",
			imf:  Vhdx,
			want: "vhdx.zst",
		},
		{
			name: "qcow2",
			imf:  Qcow,
			want: "qcow2.zst",
		},
		{
			name: "raw",
			imf:  Raw,
			want: "raw.zst",
		},
		{
			name: "tar.zst",
			imf:  Tar,
			want: "tar.zst",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.imf.KindWithCompression(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
