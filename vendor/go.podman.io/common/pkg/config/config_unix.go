//go:build !windows

package config

import (
	"path/filepath"
)

func safeEvalSymlinks(filePath string) (string, error) {
	return filepath.EvalSymlinks(filePath)
}
