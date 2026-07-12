//go:build !linux

package etchosts

// wslHostIP returns an empty string when running on OSes other than Linux
// (should never happen).
func wslHostIP() string {
	return ""
}
