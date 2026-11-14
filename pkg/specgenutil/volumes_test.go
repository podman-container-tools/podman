package specgenutil

import "testing"

func Test_validChownFlag(t *testing.T) {
	type args struct {
		flag string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "lower-case true",
			args: args{
				flag: "true",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "case-insensitive true",
			args: args{
				flag: "True",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "lower-case false",
			args: args{
				flag: "false",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "case-insensitive false",
			args: args{
				flag: "falsE",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "garbage value should fail",
			args: args{
				flag: "foobar",
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validChownFlag(tt.args.flag)
			if (err != nil) != tt.wantErr {
				t.Errorf("validChownFlag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validChownFlag() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNamedVolumeDriver(t *testing.T) {
	volume, err := getNamedVolume([]string{
		"source=test-volume",
		"target=/data",
		"volume-driver=test-plugin",
		"volume-opt=type=tmpfs",
	})
	if err != nil {
		t.Fatalf("getNamedVolume() error = %v", err)
	}
	if volume.Driver != "test-plugin" {
		t.Errorf("getNamedVolume() driver = %q, want %q", volume.Driver, "test-plugin")
	}
	if len(volume.Options) != 1 || volume.Options[0] != "volume-opt=type=tmpfs" {
		t.Errorf("getNamedVolume() options = %v, want [volume-opt=type=tmpfs]", volume.Options)
	}
}

func TestParseMountOptionsVolumeDriverErrors(t *testing.T) {
	tests := []struct {
		name      string
		mountType string
		options   []string
	}{
		{
			name:      "driver requires a value",
			mountType: "volume",
			options:   []string{"target=/data", "volume-driver="},
		},
		{
			name:      "driver cannot be set more than once",
			mountType: "volume",
			options:   []string{"target=/data", "volume-driver=one", "volume-driver=two"},
		},
		{
			name:      "driver is only valid for volume mounts",
			mountType: "bind",
			options:   []string{"target=/data", "volume-driver=test-plugin"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseMountOptions(test.mountType, test.options); err == nil {
				t.Fatal("parseMountOptions() expected an error")
			}
		})
	}
}
