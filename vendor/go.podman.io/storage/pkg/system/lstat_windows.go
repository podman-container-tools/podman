package system

import (
	"os"

	"go.podman.io/storage/internal/stat"
)

// Lstat calls os.Lstat to get a fileinfo interface back.
// This is then copied into our own locally defined structure.
func Lstat(path string) (*StatT, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	return stat.FromFileInfo(fi), nil
}
