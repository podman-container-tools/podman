//go:build !linux || remote

package main

func checkSupportedCgroups() {
	// NOP on Non Linux or Remote
}
