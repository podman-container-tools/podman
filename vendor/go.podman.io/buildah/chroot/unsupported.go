//go:build !linux && !freebsd

package chroot

import (
	"context"
	"fmt"
	"io"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// RunUsingChroot is not supported.
//
//go:fix inline
func RunUsingChroot(spec *specs.Spec, bundlePath, homeDir string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	return RunUsingChrootContext(context.TODO(), spec, bundlePath, homeDir, stdin, stdout, stderr)
}

// RunUsingChrootContext is not supported.
func RunUsingChrootContext(ctx context.Context, spec *specs.Spec, bundlePath, homeDir string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return fmt.Errorf("--isolation chroot is not supported on this platform")
}
