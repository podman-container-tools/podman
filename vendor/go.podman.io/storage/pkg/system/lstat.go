package system

import (
	"os"

	"go.podman.io/storage/internal/stat"
)

// RootLstat takes fsPath within root and returns
// a system.StatT type pertaining to that file.
func RootLstat(root *os.Root, fsPath string) (*StatT, error) {
	fi, err := root.Lstat(fsPath)
	if err != nil {
		return nil, err
	}
	return stat.FromFileInfo(fi), nil
}
