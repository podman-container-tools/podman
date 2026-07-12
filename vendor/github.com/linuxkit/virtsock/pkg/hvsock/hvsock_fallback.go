// +build !linux,!windows

package hvsock

import (
	"fmt"
	"net"
	"runtime"
)

// Supported returns if hvsocks are supported on your platform
func Supported() bool {
	return false
}

func Dial(raddr Addr) (Conn, error) {
	return nil, fmt.Errorf("Dial() not implemented on %s", runtime.GOOS)
}

func Listen(addr Addr) (net.Listener, error) {
	return nil, fmt.Errorf("Listen() not implemented on %s", runtime.GOOS)
}
