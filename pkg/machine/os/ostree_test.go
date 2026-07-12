package os

import (
	"testing"

	"github.com/blang/semver/v4"
)

func Test_compareMajorMinor(t *testing.T) {
	type args struct {
		versionA semver.Version
		versionB semver.Version
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "equal major and minor versions and different patch",
			args: args{
				versionA: semver.MustParse("1.2.3"),
				versionB: semver.MustParse("1.2.5"),
			},
			want: 0,
		},
		{
			name: "A major version less than B",
			args: args{
				versionA: semver.MustParse("1.5.0"),
				versionB: semver.MustParse("2.5.0"),
			},
			want: -1,
		},
		{
			name: "A major version greater than B",
			args: args{
				versionA: semver.MustParse("3.2.0"),
				versionB: semver.MustParse("2.9.0"),
			},
			want: 1,
		},
		{
			name: "A minor version less than B (same major)",
			args: args{
				versionA: semver.MustParse("1.2.0"),
				versionB: semver.MustParse("1.5.0"),
			},
			want: -1,
		},
		{
			name: "A minor version greater than B (same major)",
			args: args{
				versionA: semver.MustParse("1.8.0"),
				versionB: semver.MustParse("1.3.0"),
			},
			want: 1,
		},
		{
			name: "completely equal versions",
			args: args{
				versionA: semver.MustParse("1.2.3"),
				versionB: semver.MustParse("1.2.3"),
			},
			want: 0,
		},
		{
			name: "zero versions",
			args: args{
				versionA: semver.MustParse("0.0.0"),
				versionB: semver.MustParse("0.0.1"),
			},
			want: 0,
		},
		{
			name: "A is zero, B is not",
			args: args{
				versionA: semver.MustParse("0.0.0"),
				versionB: semver.MustParse("1.0.0"),
			},
			want: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareMajorMinor(tt.args.versionA, tt.args.versionB); got != tt.want {
				t.Errorf("compareMajorMinor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseApplyInput(t *testing.T) {
	type args struct {
		arg string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name:    "bare registry reference with tag",
			args:    args{arg: "quay.io/fedora/fedora-bootc:40"},
			want:    "registry",
			want1:   "quay.io/fedora/fedora-bootc:40",
			wantErr: false,
		},
		{
			name:    "docker transport with tag",
			args:    args{arg: "docker://quay.io/fedora/fedora-bootc:40"},
			want:    "registry",
			want1:   "quay.io/fedora/fedora-bootc:40",
			wantErr: false,
		},
		{
			name:    "docker transport with digest",
			args:    args{arg: "docker://quay.io/podman/stable@sha256:9cca0703342e24806a9f64e08c053dca7f2cd90f10529af8ea872afb0a0c77d4"},
			want:    "registry",
			want1:   "quay.io/podman/stable@sha256:9cca0703342e24806a9f64e08c053dca7f2cd90f10529af8ea872afb0a0c77d4",
			wantErr: false,
		},
		{
			name:    "registry transport with tag",
			args:    args{arg: "registry://quay.io/fedora/fedora-bootc:40"},
			want:    "registry",
			want1:   "quay.io/fedora/fedora-bootc:40",
			wantErr: false,
		},
		{
			name:    "registry transport with digest",
			args:    args{arg: "registry://quay.io/podman/stable@sha256:9cca0703342e24806a9f64e08c053dca7f2cd90f10529af8ea872afb0a0c77d4"},
			want:    "registry",
			want1:   "quay.io/podman/stable@sha256:9cca0703342e24806a9f64e08c053dca7f2cd90f10529af8ea872afb0a0c77d4",
			wantErr: false,
		},
		{
			name:    "registry transport with port",
			args:    args{arg: "registry://localhost:5000/myapp:latest"},
			want:    "registry",
			want1:   "localhost:5000/myapp:latest",
			wantErr: false,
		},
		{
			name:    "oci transport var path",
			args:    args{arg: "oci:/var/mnt/usb/bootc-image"},
			want:    "oci",
			want1:   "/var/mnt/usb/bootc-image",
			wantErr: false,
		},
		{
			name:    "oci transport opt path",
			args:    args{arg: "oci:/opt/images/mylayout"},
			want:    "oci",
			want1:   "/opt/images/mylayout",
			wantErr: false,
		},
		{
			name:    "oci transport tmp path",
			args:    args{arg: "oci:/tmp/oci-image"},
			want:    "oci",
			want1:   "/tmp/oci-image",
			wantErr: false,
		},
		{
			name:    "oci transport with reference",
			args:    args{arg: "oci:/path/to/oci-dir:myref"},
			want:    "oci",
			want1:   "/path/to/oci-dir:myref",
			wantErr: false,
		},
		{
			name:    "oci-archive transport tmp tar",
			args:    args{arg: "oci-archive:/tmp/myimage.tar"},
			want:    "oci-archive",
			want1:   "/tmp/myimage.tar",
			wantErr: false,
		},
		{
			name:    "oci-archive transport mnt path",
			args:    args{arg: "oci-archive:/mnt/usb/images/myapp.tar"},
			want:    "oci-archive",
			want1:   "/mnt/usb/images/myapp.tar",
			wantErr: false,
		},
		{
			name:    "oci-archive transport with reference",
			args:    args{arg: "oci-archive:/home/user/archives/image.tar:myref"},
			want:    "oci-archive",
			want1:   "/home/user/archives/image.tar:myref",
			wantErr: false,
		},
		{
			name:    "containers-storage transport with tag",
			args:    args{arg: "containers-storage:quay.io/podman/machine-os:6.0"},
			want:    "containers-storage",
			want1:   "quay.io/podman/machine-os:6.0",
			wantErr: false,
		},
		{
			name:    "containers-storage transport with graph root",
			args:    args{arg: "containers-storage:[/home/core/.local/share/containers/storage]quay.io/podman/machine-os:6.0"},
			want:    "containers-storage",
			want1:   "[/home/core/.local/share/containers/storage]quay.io/podman/machine-os:6.0",
			wantErr: false,
		},
		{
			name:    "good reference bad transport",
			args:    args{"foobar://quay.io/podman/machine-os:6.0"},
			want:    "",
			want1:   "",
			wantErr: true,
		},
		{
			name:    "similar name but incorrect transport",
			args:    args{"oci-dir:/foo/bar/bar.tar"},
			want:    "",
			want1:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := parseApplyInput(tt.args.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseApplyInput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseApplyInput() got = '%v', want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("parseApplyInput() got1 = '%v', want %v", got1, tt.want1)
			}
		})
	}
}
