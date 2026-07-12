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
