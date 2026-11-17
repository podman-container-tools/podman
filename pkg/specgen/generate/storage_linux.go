//go:build !remote

package generate

import (
	"context"

	"go.podman.io/common/libimage"
)

func imageRunPath(_ context.Context, _ *libimage.Image) (string, error) {
	return "/run", nil
}
