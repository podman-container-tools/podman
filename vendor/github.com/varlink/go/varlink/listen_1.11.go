// +build go1.11

package varlink

import (
	"context"
	"net"
)

func listen(ctx context.Context, network, address string) (net.Listener, error) {
	var lc net.ListenConfig
	return lc.Listen(ctx, network, address)
}