package config

import (
	"io/fs"
	"os"
	"path/filepath"
)

var defaultHelperBinariesDir = []string{
	// FindHelperBinaries(), as a convention, interprets $BINDIR as the
	// directory where the current process binary (i.e. podman) is located.
	"$BINDIR",
}

func safeEvalSymlinks(filePath string) (string, error) {
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		return "", err
	}
	if fileInfo.Mode()&fs.ModeSymlink != 0 {
		// Only call filepath.EvalSymlinks if it is a symlink.
		// Starting with v1.23, EvalSymlinks returns an error for mount points.
		// See https://go-review.googlesource.com/c/go/+/565136 for reference.
		filePath, err = filepath.EvalSymlinks(filePath)
		if err != nil {
			return "", err
		}
	} else {
		// Call filepath.Clean when filePath is not a symlink. That's for
		// consistency with the symlink case (filepath.EvalSymlinks calls
		// Clean after evaluating filePath).
		filePath = filepath.Clean(filePath)
	}
	return filePath, nil
}
