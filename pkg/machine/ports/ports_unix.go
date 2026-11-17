//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package ports

import (
	"net"
	"syscall"
)

func getPortCheckListenConfig() *net.ListenConfig {
	return &net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) (cerr error) {
			if err := c.Control(func(fd uintptr) {
				// Prevent listening socket from holding over in TIME_WAIT in the rare case a connection
				// attempt occurs in the short window the socket is listening. This ensures the registration
				// will be gone when close() completes, freeing it up for the real subsequent listen by another
				// process
				cerr = syscall.SetsockoptLinger(int(fd), syscall.SOL_SOCKET, syscall.SO_LINGER, &syscall.Linger{
					Onoff:  1,
					Linger: 0,
				})
			}); err != nil {
				cerr = err
			}
			return cerr
		},
	}
}
