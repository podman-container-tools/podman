package driver

import (
	"fmt"
	"strings"

	"go.podman.io/storage/pkg/parsers"
)

// isKnownDriverName checks if the given driver is known to this code base.
func isKnownDriverName(driver string) bool {
	// Note we do not use the drivers map here because we want all known drivers
	// not just the ones that were compiled in.
	// Also we can use that to handle the overlay2 special case because the option
	// parser accepts option with that name.
	switch driver {
	case "overlay", "overlay2", "btrfs", "zfs", "vfs":
		return true
	}
	return false
}

// ParseDriverOption parses the given option string of the format "[driver].optname=val"
// and returns the driver name (can be empty in which case the option should be parsed
// by all drivers)
func ParseDriverOption(option string) (string, string, string, error) {
	key, val, err := parsers.ParseKeyValueOpt(option)
	if err != nil {
		return "", "", "", err
	}

	key = strings.ToLower(key)
	driver, optName, ok := strings.Cut(key, ".")
	if !ok {
		optName = driver
		driver = ""
	} else if driver != "" {
		if !isKnownDriverName(driver) {
			return "", "", "", fmt.Errorf("unknown driver %q in option %q", driver, option)
		}
	}
	return driver, optName, val, nil
}
