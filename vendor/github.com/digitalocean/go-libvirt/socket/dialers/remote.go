package dialers

import (
	"net"
	"time"
)

const (
	// defaultRemotePort specifies the default libvirtd port.
	defaultRemotePort = "16509"

	// defaultRemoteTimeout specifies the default libvirt dial timeout.
	defaultRemoteTimeout = 20 * time.Second
)

// Remote implements connecting to a remote server's libvirt using tcp
type Remote struct {
	timeout    time.Duration
	host, port string
}

// RemoteOption is a function for setting remote dialer options.
type RemoteOption func(*Remote)

// WithRemoteTimeout sets the dial timeout.
func WithRemoteTimeout(timeout time.Duration) RemoteOption {
	return func(r *Remote) {
		r.timeout = timeout
	}
}

// UsePort sets the port to dial for libirt on the target host server.
func UsePort(port string) RemoteOption {
	return func(r *Remote) {
		r.port = port
	}
}

// NewRemote is a dialer for connecting to libvirt running on another server.
func NewRemote(hostAddr string, opts ...RemoteOption) *Remote {
	r := &Remote{
		timeout: defaultRemoteTimeout,
		host:    hostAddr,
		port:    defaultRemotePort,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Dial connects to libvirt running on another server.
func (r *Remote) Dial() (net.Conn, error) {
	return net.DialTimeout(
		"tcp",
		net.JoinHostPort(r.host, r.port),
		r.timeout,
	)
}
