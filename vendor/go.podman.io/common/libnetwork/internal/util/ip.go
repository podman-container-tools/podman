package util

import (
	"net"
)

func NetworkIntersectsWithNetworks(n *net.IPNet, networklist []*net.IPNet) bool {
	for _, nw := range networklist {
		if networkIntersect(n, nw) {
			return true
		}
	}
	return false
}

func networkIntersect(n1, n2 *net.IPNet) bool {
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}
