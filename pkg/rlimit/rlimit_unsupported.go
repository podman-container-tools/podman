//go:build !(linux || freebsd) || !cgo

package rlimit

import "errors"

// GetOriginalNofile is unavailable without the native initialization hook.
func GetOriginalNofile() (uint64, uint64, error) {
	return 0, 0, errors.New("original file limits are not supported on this platform")
}
