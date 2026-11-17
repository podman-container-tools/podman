//go:build !windows

package define

import "os"

// openShareDelete opens path for reading.
func openShareDelete(path string) (*os.File, error) {
	return os.Open(path)
}
