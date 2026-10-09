//go:build !remote && linux && !cgo

package libpod

import (
	"context"
	"errors"
)

// Top gathers statistics about the running processes in a container. It returns a
// []string for output
func (c *Container) Top(ctx context.Context, descriptors []string) ([]string, error) {
	return nil, errors.New("not implemented (*Container) Top")
}
