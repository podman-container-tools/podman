//go:build !amd64 && !arm64

package system

import "context"

func resetMachine(context.Context) error {
	return nil
}
