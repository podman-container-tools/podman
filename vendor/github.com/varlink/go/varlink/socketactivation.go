//go:build !windows

package varlink

import (
	"net"
	"os"
	"strconv"
	"strings"
)

func activationListener() net.Listener {
	pid, err := strconv.Atoi(os.Getenv("LISTEN_PID"))
	if err != nil || pid != os.Getpid() {
		return nil
	}

	nfds, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
	if err != nil || nfds < 1 {
		return nil
	}

	fd := -1

	// If more than one file descriptor is passed, find the
	// "varlink" tag. The first file descriptor is always 3.
	if nfds > 1 {
		fdnames, set := os.LookupEnv("LISTEN_FDNAMES")
		if !set {
			return nil
		}

		names := strings.Split(fdnames, ":")
		if len(names) != nfds {
			return nil
		}

		for i, name := range names {
			if name == "varlink" {
				fd = 3 + i
				break
			}
		}

		if fd < 0 {
			return nil
		}

	} else {
		fd = 3
	}

	file := os.NewFile(uintptr(fd), "varlink")
	defer file.Close()

	listener, err := net.FileListener(file)
	if err != nil {
		return nil
	}

	return listener
}
