package compression

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/machine/define"
)

func Test_compressionFromFile(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want ImageCompression
	}{
		{
			name: "xz",
			args: args{
				path: "/tmp/foo.xz",
			},
			want: Xz,
		},
		{
			name: "gzip",
			args: args{
				path: "/tmp/foo.gz",
			},
			want: Gz,
		},
		{
			name: "bz2",
			args: args{
				path: "/tmp/foo.bz2",
			},
			want: Bz2,
		},
		{
			name: "default is zstd",
			args: args{
				path: "/tmp/foo",
			},
			want: Zstd,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KindFromFile(tt.args.path); got != tt.want {
				t.Errorf("KindFromFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageCompression_String(t *testing.T) {
	tests := []struct {
		name string
		c    ImageCompression
		want string
	}{
		{
			name: "xz",
			c:    Xz,
			want: "xz",
		},
		{
			name: "gz",
			c:    Gz,
			want: "gz",
		},
		{
			name: "bz2",
			c:    Bz2,
			want: "bz2",
		},
		{
			name: "zip",
			c:    Zip,
			want: "zip",
		},
		{
			name: "zstd is default",
			c:    99,
			want: "zst",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Decompress(t *testing.T) {
	type args struct {
		src string
		dst string
	}

	type want string

	tests := []struct {
		name string
		args args
		want want
	}{
		{name: "zip", args: args{src: "./testdata/sample.zip", dst: "./testdata/hellozip"}, want: "zip\n"},
		{name: "zip with trailing zeros", args: args{src: "./testdata/sample-withzeros.zip", dst: "./testdata/hellozip-withzeros"}, want: "zip\n\x00\x00\x00\x00\x00\x00"},
		{name: "xz", args: args{src: "./testdata/sample.xz", dst: "./testdata/helloxz"}, want: "xz\n"},
		{name: "xz with trailing zeros", args: args{src: "./testdata/sample-withzeros.xz", dst: "./testdata/helloxz-withzeros"}, want: "xz\n\x00\x00\x00\x00\x00\x00\x00"},
		{name: "gzip", args: args{src: "./testdata/sample.gz", dst: "./testdata/hellogz"}, want: "gzip\n"},
		{name: "gzip with trailing zeros", args: args{src: "./testdata/sample-withzeros.gz", dst: "./testdata/hellogzip-withzeros"}, want: "gzip\n\x00\x00\x00\x00\x00"},
		{name: "bzip2", args: args{src: "./testdata/sample.bz2", dst: "./testdata/hellobz2"}, want: "bzip2\n"},
		{name: "bzip2 with trailing zeros", args: args{src: "./testdata/sample-withzeros.bz2", dst: "./testdata/hellobz2-withzeros"}, want: "bzip2\n\x00\x00\x00\x00"},
		{name: "zstd", args: args{src: "./testdata/sample.zst", dst: "./testdata/hellozstd"}, want: "zstd\n"},
		{name: "zstd with trailing zeros", args: args{src: "./testdata/sample-withzeros.zst", dst: "./testdata/hellozstd-withzeros"}, want: "zstd\n\x00\x00\x00\x00\x00"},
		{name: "uncompressed", args: args{src: "./testdata/sample.uncompressed", dst: "./testdata/hellouncompressed"}, want: "uncompressed\n"},
		{name: "uncompressed with trailing zeros", args: args{src: "./testdata/sample-withzeros.uncompressed", dst: "./testdata/hellozuncompressed-withzeros"}, want: "uncompressed\n\x00\x00\x00\x00\x00\x00\x00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcVMFile := &define.VMFile{Path: tt.args.src}
			dstFilePath := tt.args.dst
			defer os.Remove(dstFilePath)
			err := Decompress(srcVMFile, dstFilePath)
			require.NoError(t, err)
			data, err := os.ReadFile(dstFilePath)
			require.NoError(t, err)
			assert.Equal(t, string(tt.want), string(data))
		})
	}
}
