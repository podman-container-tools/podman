package system

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/bindings"
)

var dialStdioCommand = &cobra.Command{
	Use:    "dial-stdio",
	Short:  "Proxy the stdio stream to the daemon connection. Should not be invoked manually.",
	Args:   validate.NoArgs,
	Hidden: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		return runDialStdio()
	},
	Example: "podman system dial-stdio",
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: dialStdioCommand,
		Parent:  systemCmd,
	})
}

func runDialStdio() error {
	ctx := registry.Context()
	cfg := registry.PodmanConfig()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	bindCtx, err := bindings.NewConnection(ctx, cfg.URI)
	if err != nil {
		return fmt.Errorf("failed to open connection to podman: %w", err)
	}
	conn, err := bindings.GetClient(bindCtx)
	if err != nil {
		return fmt.Errorf("failed to get connection after initialization: %w", err)
	}
	netConn, err := conn.GetDialer(bindCtx)
	if err != nil {
		return fmt.Errorf("failed to open the raw stream connection: %w", err)
	}
	defer netConn.Close()

	var connHalfCloser halfCloser
	switch t := netConn.(type) {
	case halfCloser:
		connHalfCloser = t
	case halfReadWriteCloser:
		connHalfCloser = &nopCloseReader{t}
	default:
		return errors.New("the raw stream connection does not implement halfCloser")
	}

	stdin2conn := make(chan error, 1)
	conn2stdout := make(chan error, 1)
	go func() {
		stdin2conn <- copier(connHalfCloser, &halfReadCloserWrapper{os.Stdin}, "stdin to stream")
	}()
	go func() {
		conn2stdout <- copier(&halfWriteCloserWrapper{os.Stdout}, connHalfCloser, "stream to stdout")
	}()
	select {
	case err = <-stdin2conn:
		if err != nil {
			return err
		}
		// wait for stdout
		err = <-conn2stdout
	case err = <-conn2stdout:
		// return immediately
	}
	return err
}

// Below portion taken from original docker CLI
// https://github.com/docker/cli/blob/v20.10.9/cli/command/system/dial_stdio.go
func copier(to halfWriteCloser, from halfReadCloser, debugDescription string) error {
	defer func() {
		if err := from.CloseRead(); err != nil {
			logrus.Errorf("while CloseRead (%s): %v", debugDescription, err)
		}
		if err := to.CloseWrite(); err != nil {
			logrus.Errorf("while CloseWrite (%s): %v", debugDescription, err)
		}
	}()
	if _, err := io.Copy(to, from); err != nil {
		return fmt.Errorf("while Copy (%s): %w", debugDescription, err)
	}
	return nil
}

type halfReadCloser interface {
	io.Reader
	CloseRead() error
}

type halfWriteCloser interface {
	io.Writer
	CloseWrite() error
}

type halfCloser interface {
	halfReadCloser
	halfWriteCloser
}

type halfReadWriteCloser interface {
	io.Reader
	halfWriteCloser
}

type nopCloseReader struct {
	halfReadWriteCloser
}

func (x *nopCloseReader) CloseRead() error {
	return nil
}

type halfReadCloserWrapper struct {
	io.ReadCloser
}

func (x *halfReadCloserWrapper) CloseRead() error {
	return x.Close()
}

type halfWriteCloserWrapper struct {
	io.WriteCloser
}

func (x *halfWriteCloserWrapper) CloseWrite() error {
	return x.Close()
}
