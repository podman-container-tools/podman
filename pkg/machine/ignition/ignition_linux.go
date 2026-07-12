package ignition

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func getLocalTimeZone() (string, error) {
	path, err := filepath.EvalSymlinks("/etc/localtime")
	if err != nil {
		// of the path does not exist, ignore it as the code default to UTC then
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	// Allow using TZDIR per:
	// https://sourceware.org/git/?p=glibc.git;a=blob;f=time/tzfile.c;h=8a923d0cccc927a106dc3e3c641be310893bab4e;hb=HEAD#l149
	zoneinfo := os.Getenv("TZDIR")
	if zoneinfo == "" {
		// default zoneinfo location
		zoneinfo = "/usr/share/zoneinfo"
	}
	// Trim of the TZDIR part to extract the actual timezone name
	return strings.TrimPrefix(path, filepath.Clean(zoneinfo)+"/"), nil
}
