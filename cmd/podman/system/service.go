//go:build (linux || freebsd) && !remote

package system

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/systemd"
	"go.podman.io/podman/v6/pkg/util"
)

var (
	srvDescription = `Run an API service

Enable a listening service for API access to Podman commands.
`

	srvCmd = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "service [options] [URI]",
		Args:              cobra.MaximumNArgs(1),
		Short:             "Run API service",
		Long:              srvDescription,
		RunE:              service,
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example: `podman system service --time=0 unix:///tmp/podman.sock
podman system service --time=0 tcp://localhost:8888
podman system service --time=0 --tls-cert=tls.crt --tls-key=tls.key tcp://localhost:8888
podman system service --time=0 --tls-cert=tls.crt --tls-key=tls.key --tls-client-ca=ca.crt tcp://localhost:8888`,
	}

	srvArgs = struct {
		CorsHeaders     string
		PProfAddr       string
		Timeout         uint
		TLSCertFile     string
		TLSKeyFile      string
		TLSClientCAFile string
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: srvCmd,
		Parent:  systemCmd,
	})

	flags := srvCmd.Flags()
	cfg := registry.PodmanConfig()

	timeFlagName := "time"
	flags.UintVarP(&srvArgs.Timeout, timeFlagName, "t", cfg.ContainersConfDefaultsRO.Engine.ServiceTimeout,
		"Time until the service session expires in seconds.  Use 0 to disable the timeout")
	_ = srvCmd.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)
	flags.SetNormalizeFunc(aliasTimeoutFlag)

	flags.StringVarP(&srvArgs.CorsHeaders, "cors", "", "", "Set CORS Headers")
	_ = srvCmd.RegisterFlagCompletionFunc("cors", completion.AutocompleteNone)

	flags.StringVarP(&srvArgs.PProfAddr, "pprof-address", "", "",
		"Binding network address for pprof profile endpoints, default: do not expose endpoints")
	_ = flags.MarkHidden("pprof-address")

	flags.StringVarP(&srvArgs.TLSCertFile, "tls-cert", "", "",
		"PEM file containing TLS serving certificate.")
	_ = srvCmd.RegisterFlagCompletionFunc("tls-cert", completion.AutocompleteDefault)
	flags.StringVarP(&srvArgs.TLSKeyFile, "tls-key", "", "",
		"PEM file containing TLS serving certificate private key")
	_ = srvCmd.RegisterFlagCompletionFunc("tls-key", completion.AutocompleteDefault)
	flags.StringVarP(&srvArgs.TLSClientCAFile, "tls-client-ca", "", "",
		"Only trust client connections with certificates signed by this CA PEM file")
	_ = srvCmd.RegisterFlagCompletionFunc("tls-client-ca", completion.AutocompleteDefault)
}

func aliasTimeoutFlag(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	if name == "timeout" {
		name = "time"
	}
	return pflag.NormalizedName(name)
}

func service(cmd *cobra.Command, args []string) error {
	apiURI, err := resolveAPIURI(args)
	if err != nil {
		return err
	}

	// Clean up any old existing unix domain socket
	if len(apiURI) > 0 {
		uri, err := url.Parse(apiURI)
		if err != nil {
			return err
		}

		// socket activation uses a unix:// socket in the shipped unit files but apiURI is coded as "" at this layer.
		if uri.Scheme == "unix" && !registry.IsRemote() {
			if err := syscall.Unlink(uri.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			mask := syscall.Umask(0o177)
			defer syscall.Umask(mask)
		}
	}

	if len(srvArgs.TLSCertFile) != 0 && len(srvArgs.TLSKeyFile) == 0 {
		return fmt.Errorf("--tls-cert provided without --tls-key")
	}
	if len(srvArgs.TLSKeyFile) != 0 && len(srvArgs.TLSCertFile) == 0 {
		return fmt.Errorf("--tls-key provided without --tls-cert")
	}

	return restService(cmd.Flags(), registry.PodmanConfig(), entities.ServiceOptions{
		CorsHeaders:     srvArgs.CorsHeaders,
		PProfAddr:       srvArgs.PProfAddr,
		Timeout:         time.Duration(srvArgs.Timeout) * time.Second,
		URI:             apiURI,
		TLSCertFile:     srvArgs.TLSCertFile,
		TLSKeyFile:      srvArgs.TLSKeyFile,
		TLSClientCAFile: srvArgs.TLSClientCAFile,
	})
}

func resolveAPIURI(uri []string) (string, error) {
	// When determining _*THE*_ listening endpoint --
	// 1) User input wins always
	// 2) systemd socket activation
	// 3) rootless honors XDG_RUNTIME_DIR
	// 4) lastly adapter.DefaultAPIAddress

	if len(uri) == 0 {
		if v, found := os.LookupEnv("PODMAN_SOCKET"); found {
			logrus.Debugf("PODMAN_SOCKET=%q used to determine API endpoint", v)
			uri = []string{v}
		}
	}

	switch {
	case len(uri) > 0 && uri[0] != "":
		return uri[0], nil
	case systemd.SocketActivated():
		logrus.Info("Using systemd socket activation to determine API endpoint")
		return "", nil
	case rootless.IsRootless():
		xdg, err := util.GetRootlessRuntimeDir()
		if err != nil {
			return "", err
		}

		socketName := "podman.sock"
		socketPath := filepath.Join(xdg, "podman", socketName)
		if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
			return "", err
		}
		return "unix://" + socketPath, nil
	default:
		if err := os.MkdirAll(filepath.Dir(registry.DefaultRootAPIPath), 0o700); err != nil {
			return "", err
		}
		return registry.DefaultRootAPIAddress, nil
	}
}
