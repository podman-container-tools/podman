//go:build amd64 || arm64

package connection

import (
	"net"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_makeSSHURL(t *testing.T) {
	var (
		host     = "foobar"
		path     = "/path/to/socket"
		rc       = "ssh"
		username = "core"
	)
	type args struct {
		host     string
		path     string
		port     string
		userName string
	}
	tests := []struct {
		name string
		args args
		want *url.URL
	}{
		{
			name: "Good no port",
			args: args{
				host:     host,
				path:     path,
				port:     "",
				userName: username,
			},
			want: &url.URL{
				Scheme: rc,
				User:   url.User(username),
				Host:   host,
				Path:   path,
			},
		},
		{
			name: "Good with port",
			args: args{
				host:     host,
				path:     path,
				port:     "222",
				userName: username,
			},
			want: &url.URL{
				Scheme: rc,
				User:   url.User(username),
				Host:   net.JoinHostPort(host, "222"),
				Path:   path,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeSSHURL(tt.args.host, tt.args.path, tt.args.port, tt.args.userName)
			assert.Equal(t, tt.want, got, "URL matches")
		})
	}
}
