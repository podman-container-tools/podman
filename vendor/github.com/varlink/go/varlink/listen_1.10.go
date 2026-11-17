// +build !go1.11

package varlink

import (
	"context"
	"net"
)

func listen(ctx context.Context, network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}