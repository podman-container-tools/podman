//go:build !windows

package terminal

// SetConsole for non-windows environments is a no-op.
func SetConsole() error {
	return nil
}
