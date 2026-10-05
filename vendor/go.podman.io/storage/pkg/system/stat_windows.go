package system

import (
	"os"

	"go.podman.io/storage/internal/stat"
)

// StatT type contains status of a file. It contains metadata
// like permission, size, etc about a file.
type StatT = stat.StatT

// Stat takes a path to a file and returns
// a system.StatT type pertaining to that file.
//
// Throws an error if the file does not exist
func Stat(path string) (*StatT, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return stat.FromFileInfo(fi), nil
}
