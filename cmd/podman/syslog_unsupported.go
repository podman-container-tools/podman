//go:build !linux && !freebsd

package main

import (
	"fmt"
	"os"
	"runtime"

	"go.podman.io/podman/v6/cmd/podman/registry"
)

func syslogHook() {
	if !registry.PodmanConfig().Syslog {
		return
	}

	fmt.Fprintf(os.Stderr, "Logging to Syslog is not supported on %s\n", runtime.GOOS)
	os.Exit(1)
}
