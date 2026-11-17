//go:build !linux && !windows

package sysinfo

import "runtime"

func numCPU() int {
	return runtime.NumCPU()
}
