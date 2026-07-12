package dialers

import (
	"net"
)

// AlreadyConnected implements a dialer interface for a connection that was
// established prior to initializing the socket object.  This exists solely
// for backwards compatability with the previous implementation of Libvirt
// that took an already established connection.
type AlreadyConnected struct {
	c net.Conn
}

// NewAlreadyConnected is a noop dialer to simply use a connection previously
// established.  This means any re-dial attempts simply won't work.
func NewAlreadyConnected(c net.Conn) AlreadyConnected {
	return AlreadyConnected{c}
}

// Dial just returns the connection previously established.
// If at some point it is disconnected by the client, this obviously does *not*
// re-dial and will simply return the already closed connection.
func (a AlreadyConnected) Dial() (net.Conn, error) {
	return a.c, nil
}
