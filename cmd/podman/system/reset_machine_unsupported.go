//go:build !amd64 && !arm64

package system

func resetMachine() error {
	return nil
}
