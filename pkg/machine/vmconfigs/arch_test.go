package vmconfigs

import "testing"

func TestNormalizeMachineArch(t *testing.T) {
	type args struct {
		arch string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "x86_64",
			args: args{
				arch: "x86_64",
			},
			want:    "x86_64",
			wantErr: false,
		},
		{
			name: "amd64",
			args: args{
				arch: "amd64",
			},
			want:    "x86_64",
			wantErr: false,
		},
		{
			name: "aarch64",
			args: args{
				arch: "aarch64",
			},
			want:    "aarch64",
			wantErr: false,
		},
		{
			name: "arm64",
			args: args{
				arch: "arm64",
			},
			want:    "aarch64",
			wantErr: false,
		},
		{
			name: "Unknown arch should fail",
			args: args{
				arch: "foobar",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeMachineArch(tt.args.arch)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeMachineArch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NormalizeMachineArch() got = %v, want %v", got, tt.want)
			}
		})
	}
}
