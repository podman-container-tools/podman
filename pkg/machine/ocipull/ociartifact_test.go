package ocipull

import "testing"

func Test_extractKindAndCompression(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "qcow2",
			args: args{name: "foo.qcow2.xz"},
			want: ".qcow2.xz",
		},
		{
			name: "vhdx",
			args: args{name: "foo.vhdx.zip"},
			want: ".vhdx.zip",
		},
		{
			name: "applehv",
			args: args{name: "foo.raw.gz"},
			want: ".raw.gz",
		},
		{
			name: "lots of extensions with type and compression",
			args: args{name: "foo.bar.homer.simpson.qcow2.xz"},
			want: ".qcow2.xz",
		},
		{
			name: "lots of extensions",
			args: args{name: "foo.bar.homer.simpson"},
			want: ".homer.simpson",
		},
		{
			name: "no extensions",
			args: args{name: "foobar"},
			want: "",
		},
		{
			name: "one extension",
			args: args{name: "foobar.zip"},
			want: ".zip",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractKindAndCompression(tt.args.name); got != tt.want {
				t.Errorf("extractKindAndCompression() = %v, want %v", got, tt.want)
			}
		})
	}
}
