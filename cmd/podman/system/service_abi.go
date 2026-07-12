//go:build (linux || freebsd) && !remote

package system

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"go.podman.io/podman/v6/cmd/podman/registry"
	api "go.podman.io/podman/v6/pkg/api/server"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra"
	"golang.org/x/sys/unix"
)

func restService(flags *pflag.FlagSet, cfg *entities.PodmanConfig, opts entities.ServiceOptions) error {
	var (
		listener net.Listener
		err      error
	)

	libpodRuntime, err := infra.GetRuntime(registry.Context(), flags, cfg)
	if err != nil {
		return err
	}

	if opts.URI == "" {
		if _, found := os.LookupEnv("LISTEN_PID"); !found {
			return errors.New("no service URI provided and socket activation protocol is not active")
		}

		listeners, err := activation.Listeners()
		if err != nil {
			return fmt.Errorf("cannot retrieve file descriptors from systemd: %w", err)
		}
		if len(listeners) != 1 {
			return fmt.Errorf("wrong number of file descriptors for socket activation protocol (%d != 1)", len(listeners))
		}
		listener = listeners[0]
		// note that activation.Listeners() returns nil when it cannot listen on the fd (i.e. udp connection)
		if listener == nil {
			return errors.New("unexpected fd received from systemd: cannot listen on it")
		}
		libpodRuntime.SetRemoteURI(listeners[0].Addr().Network() + "://" + listeners[0].Addr().String())
	} else {
		uri, err := url.Parse(opts.URI)
		if err != nil {
			return fmt.Errorf("%s is an invalid socket destination", opts.URI)
		}

		switch uri.Scheme {
		case "unix":
			path, err := filepath.Abs(uri.Path)
			if err != nil {
				return err
			}
			if os.Getenv("LISTEN_FDS") != "" {
				// If it is activated by systemd, use the first LISTEN_FD (3)
				// instead of opening the socket file.
				f := os.NewFile(uintptr(3), "podman.sock")
				listener, err = net.FileListener(f)
				if err != nil {
					return err
				}
			} else {
				listener, err = net.Listen(uri.Scheme, path)
				if err != nil {
					return fmt.Errorf("unable to create socket: %w", err)
				}
			}
		case "tcp":
			// We want to check if the user is requesting a TCP address if TLS is not active.
			// If so, warn that this is insecure.
			// Ignore errors here, the actual backend code will handle them
			// better than we can here.
			if opts.TLSKeyFile == "" || opts.TLSCertFile == "" {
				logrus.Warnf("Using the Podman API service with TCP sockets without TLS is not recommended, please see `podman system service` manpage for details")
			}

			host := uri.Host
			if host == "" {
				// For backward compatibility, support "tcp:<host>:<port>" and "tcp://<host>:<port>"
				host = uri.Opaque
			}
			listener, err = net.Listen(uri.Scheme, host)
			if err != nil {
				return fmt.Errorf("unable to create socket %v: %w", host, err)
			}
		default:
			return fmt.Errorf("API Service endpoint scheme %q is not supported. Try tcp://%s or unix://%s", uri.Scheme, opts.URI, opts.URI)
		}
		libpodRuntime.SetRemoteURI(uri.String())
	}

	// bugzilla.redhat.com/show_bug.cgi?id=2180483:
	//
	// Disable leaking the LISTEN_* into containers which
	// are observed to be passed by systemd even without
	// being socket activated as described in
	// https://access.redhat.com/solutions/6512011.
	for _, val := range []string{"LISTEN_FDS", "LISTEN_PID", "LISTEN_FDNAMES"} {
		if err := os.Unsetenv(val); err != nil {
			return fmt.Errorf("unsetting %s: %v", val, err)
		}
	}

	// Set stdin to /dev/null, so shortnames will not prompt
	devNullfile, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	if err := unix.Dup2(int(devNullfile.Fd()), int(os.Stdin.Fd())); err != nil {
		devNullfile.Close()
		return err
	}
	// Close the fd right away to not leak it during the entire time of the service.
	devNullfile.Close()

	maybeMoveToSubCgroup()

	maybeStartServiceReaper()
	server, err := api.NewServerWithSettings(libpodRuntime, listener, opts)
	if err != nil {
		return err
	}
	defer func() {
		if err := server.Shutdown(true); err != nil {
			logrus.Warnf("Error when stopping API service: %s", err)
		}
	}()

	err = server.Serve()
	if listener != nil {
		_ = listener.Close()
	}
	return err
}
