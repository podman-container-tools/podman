//go:build (linux || freebsd) && cgo

package rlimit

import "syscall"

/*
#include "rlimit.h"
*/
import "C"

// GetOriginalNofile returns the limits inherited before Go raised RLIMIT_NOFILE.
func GetOriginalNofile() (soft, hard uint64, err error) {
	var limit C.struct_rlimit
	if errno := C.podman_get_original_nofile(&limit); errno != 0 {
		return 0, 0, syscall.Errno(errno)
	}
	return uint64(limit.rlim_cur), uint64(limit.rlim_max), nil
}
