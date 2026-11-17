package specgenutil

import (
	"slices"
	"testing"

	"go.podman.io/podman/v6/libpod/define"
)

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

func Test_parseMountOptionsReadOnlyValue(t *testing.T) {
	tests := []struct {
		name    string
		option  string
		want    []string
		wantErr bool
	}{
		{name: "ro without value", option: "ro", want: []string{"ro"}},
		{name: "ro=true", option: "ro=true", want: []string{"ro"}},
		{name: "readonly=true", option: "readonly=true", want: []string{"ro"}},
		{name: "ro=false", option: "ro=false"},
		{name: "rw=false", option: "rw=false", want: []string{"ro"}},
		{name: "rw=true", option: "rw=true", want: []string{"rw"}},
		{name: "ro=invalid", option: "ro=invalid", wantErr: true},
		{name: "readonly=invalid", option: "readonly=invalid", wantErr: true},
		{name: "rw=invalid", option: "rw=invalid", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mnt, err := parseMountOptions(define.TypeBind, []string{"src=/a", "dst=/b", tt.option})
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseMountOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !slices.Equal(mnt.mount.Options, tt.want) {
				t.Errorf("parseMountOptions() options = %v, want %v", mnt.mount.Options, tt.want)
			}
		})
	}
}
