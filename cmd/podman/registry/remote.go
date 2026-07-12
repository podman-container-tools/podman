package registry

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/pflag"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// Value for --remote given on command line
var remoteFromCLI = struct {
	Value bool
	sync  sync.Once
}{}

const PodmanSh = "podmansh"

// IsRemote returns true if podman was built to run remote or --remote flag given on CLI
// Use in init() functions as an initialization check
func IsRemote() bool {
	// remote conflicts with podmansh in how the `-c` option gets parsed
	// This is noticeable if a user with shell set to podmansh were to execute
	// a command using ssh like so:
	// ssh user@host id
	if strings.HasSuffix(filepath.Base(os.Args[0]), PodmanSh) {
		return false
	}
	remoteFromCLI.sync.Do(func() {
		remote := false
		if _, ok := os.LookupEnv("CONTAINER_HOST"); ok {
			remote = true
		} else if _, ok := os.LookupEnv("CONTAINER_CONNECTION"); ok {
			remote = true
		}
		fs := pflag.NewFlagSet("remote", pflag.ContinueOnError)
		fs.ParseErrorsAllowlist.UnknownFlags = true
		fs.Usage = func() {}
		fs.SetInterspersed(false)
		fs.BoolVarP(&remoteFromCLI.Value, "remote", "r", remote, "")
		connectionFlagName := "connection"
		fs.StringP(connectionFlagName, "c", "", "")
		contextFlagName := "context"
		fs.String(contextFlagName, "", "")
		hostFlagName := "host"
		fs.StringP(hostFlagName, "H", "", "")
		urlFlagName := "url"
		fs.String(urlFlagName, "", "")

		_ = fs.Parse(os.Args[parseIndex():])
		// --connection or --url implies --remote
		remoteFromCLI.Value = remoteFromCLI.Value || fs.Changed(connectionFlagName) || fs.Changed(urlFlagName) || fs.Changed(hostFlagName) || fs.Changed(contextFlagName)
	})
	return podmanOptions.EngineMode == entities.TunnelMode || remoteFromCLI.Value
}
