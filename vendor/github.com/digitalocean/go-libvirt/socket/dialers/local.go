package dialers

import (
	"net"
	"time"
)

const (
	// defaultSocket specifies the default path to the libvirt unix socket.
	defaultSocket = "/var/run/libvirt/libvirt-sock"

	// defaultLocalTimeout specifies the default libvirt dial timeout.
	defaultLocalTimeout = 15 * time.Second
)

// Local implements connecting to a local libvirtd over the unix socket.
type Local struct {
	timeout time.Duration
	socket  string
}

// LocalOption is a function for setting local socket options.
type LocalOption func(*Local)

// WithLocalTimeout sets the dial timeout.
func WithLocalTimeout(timeout time.Duration) LocalOption {
	return func(l *Local) {
		l.timeout = timeout
	}
}

// WithSocket sets the path to the local libvirt socket.
func WithSocket(socket string) LocalOption {
	return func(l *Local) {
		l.socket = socket
	}
}

// NewLocal is a default dialer to simply connect to a locally running libvirt's
// socket.
func NewLocal(opts ...LocalOption) *Local {
	l := &Local{
		timeout: defaultLocalTimeout,
		socket:  defaultSocket,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Dial connects to a local socket
func (l *Local) Dial() (net.Conn, error) {
	return net.DialTimeout("unix", l.socket, l.timeout)
}
