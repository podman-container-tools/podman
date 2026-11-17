package registry

import (
	"os"
	"path/filepath"
	"strings"

	"go.podman.io/podman/v6/pkg/domain/entities"
)

const PodmanSh = "podmansh"

func isPodmanSh(args []string) bool {
	return len(args) > 0 && strings.HasSuffix(filepath.Base(args[0]), PodmanSh)
}

func isRemote(mode entities.EngineMode, args []string) bool {
	return !isPodmanSh(args) && mode == entities.TunnelMode
}

// IsRemote returns true if podman was built to run remote or --remote flag given on CLI
// Use in init() functions as an initialization check
func IsRemote() bool {
	// remote conflicts with podmansh in how the `-c` option gets parsed
	// This is noticeable if a user with shell set to podmansh were to execute
	// a command using ssh like so:
	// ssh user@host id
	return isRemote(PodmanConfig().EngineMode, os.Args)
}
