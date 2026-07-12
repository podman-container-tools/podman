//go:build !linux

package main

func checkSupportedCgroups() {
	// NOP on Non Linux
}
