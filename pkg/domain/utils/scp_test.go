package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/pkg/ssh"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func TestValidateSCPArgs(t *testing.T) {
	type args struct {
		locations []*entities.ScpTransferImageOptions
	}
	tests := []struct {
		name    string
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "test args length more than 2",
			args: args{
				locations: []*entities.ScpTransferImageOptions{
					{
						Image: "source image one",
					},
					{
						Image: "source image two",
					},
					{
						Image: "target image one",
					},
					{
						Image: "target image two",
					},
				},
			},
			wantErr: assert.Error,
		},
		{
			name: "test source image is empty",
			args: args{
				locations: []*entities.ScpTransferImageOptions{
					{
						Image: "",
					},
					{
						Image: "target image",
					},
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "test target image is empty",
			args: args{
				locations: []*entities.ScpTransferImageOptions{
					{
						Image: "source image",
					},
					{
						Image: "target image",
					},
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, ValidateSCPArgs(tt.args.locations), fmt.Sprintf("ValidateSCPArgs(%v)", tt.args.locations))
		})
	}
}

// The trailing newline is harmless while the path is last on a command line, but
// anything appending to it splices the newline into the middle.
func TestTrimRemotePath(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{
			name: "path as mktemp prints it",
			out:  "/tmp/tmp.5CGFmzWnCu\n",
			want: "/tmp/tmp.5CGFmzWnCu",
		},
		{
			name: "path with a carriage return",
			out:  "/tmp/tmp.5CGFmzWnCu\r\n",
			want: "/tmp/tmp.5CGFmzWnCu",
		},
		{
			name: "path already trimmed",
			out:  "/tmp/tmp.5CGFmzWnCu",
			want: "/tmp/tmp.5CGFmzWnCu",
		},
		{
			name: "empty output",
			out:  "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimRemotePath(tt.out)
			assert.Equal(t, tt.want, got)
			// Appending has to stay on one line: this is what the trim is for.
			assert.NotContains(t, got+".gz", "\n")
		})
	}
}

func TestParseImageSCPArg(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantUser string
	}{
		{
			name:     "user without domain",
			arg:      "user@localhost::alpine",
			wantUser: "user",
		},
		{
			name:     "username containing an @ (e.g. Active Directory)",
			arg:      "user@domain@localhost::example.com/foo/bar:latest",
			wantUser: "user@domain",
		},
		{
			name:     "no username before @localhost::",
			arg:      "@localhost::alpine",
			wantUser: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			location, _, err := ParseImageSCPArg(tt.arg)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantUser, location.User)
		})
	}
}

// fakeRemote records the commands that would have run over ssh and replays a
// canned error for each.
type fakeRemote struct {
	// errs is consulted per call, by index; nil succeeds.
	errs []error
	argv [][]string
}

func (f *fakeRemote) exec(opts *ssh.ConnectionExecOptions, _ ssh.EngineMode) (string, error) {
	f.argv = append(f.argv, opts.Args)
	if len(f.argv) <= len(f.errs) {
		return "", f.errs[len(f.argv)-1]
	}
	return "", nil
}

// -f matters: the cleanup has to tolerate a path that was never created.
func TestRemoveRemoteFiles(t *testing.T) {
	remote := &fakeRemote{}
	removeRemoteFiles(remote.exec, ssh.ConnectionExecOptions{}, ssh.GolangMode, "/tmp/a", "/tmp/b")
	assert.Equal(t, [][]string{{"rm", "-f", "/tmp/a", "/tmp/b"}}, remote.argv)
}
