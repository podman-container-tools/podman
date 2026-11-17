//go:build !linux && !darwin

package util //nolint:revive,nolintlint

import (
	"os"
)

func UID(st os.FileInfo) int {
	return 0
}

func GID(st os.FileInfo) int {
	return 0
}
