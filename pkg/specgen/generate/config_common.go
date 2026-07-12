//go:build !remote

package generate

import (
	"fmt"
	"strings"
)

// ParseDevice parses device mapping string to a src, dest & permissions string
func ParseDevice(device string) (string, string, string, error) {
	var src string
	var dst string
	permissions := "rwm"
	arr := strings.Split(device, ":")
	switch len(arr) {
	case 3:
		if arr[2] == "" {
			return "", "", "", fmt.Errorf("empty device mode in device specification: %s", device)
		}
		if !IsValidDeviceMode(arr[2]) {
			return "", "", "", fmt.Errorf("invalid device mode %q in device %q", arr[2], device)
		}
		permissions = arr[2]
		fallthrough
	case 2:
		if IsValidDeviceMode(arr[1]) {
			permissions = arr[1]
		} else {
			if len(arr[1]) > 0 && arr[1][0] != '/' {
				return "", "", "", fmt.Errorf("invalid device mode %q in device %q", arr[1], device)
			}
			dst = arr[1]
		}
		fallthrough
	case 1:
		src = arr[0]
	default:
		return "", "", "", fmt.Errorf("invalid device specification: %s", device)
	}

	if dst == "" {
		dst = src
	}
	return src, dst, permissions, nil
}

// IsValidDeviceMode checks if the mode for device is valid or not.
// IsValid mode is a composition of r (read), w (write), and m (mknod).
func IsValidDeviceMode(mode string) bool {
	legalDeviceMode := map[rune]bool{
		'r': true,
		'w': true,
		'm': true,
	}
	if mode == "" {
		return false
	}
	for _, c := range mode {
		if !legalDeviceMode[c] {
			return false
		}
		legalDeviceMode[c] = false
	}
	return true
}
