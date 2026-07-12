package nat

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ParsePortRange parses and validates the specified string as a port range (e.g., "8000-9000").
func ParsePortRange(ports string) (startPort, endPort uint64, _ error) {
	start, end, err := parsePortRange(ports)
	return uint64(start), uint64(end), err
}

// parsePortRange parses and validates the specified string as a port range (e.g., "8000-9000").
func parsePortRange(ports string) (startPort, endPort int, _ error) {
	if ports == "" {
		return 0, 0, errors.New("empty string specified for ports")
	}
	start, end, ok := strings.Cut(ports, "-")

	startPort, err := parsePortNumber(start)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start port '%s': %w", start, err)
	}
	if !ok || start == end {
		return startPort, startPort, nil
	}

	endPort, err = parsePortNumber(end)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end port '%s': %w", end, err)
	}
	if endPort < startPort {
		return 0, 0, errors.New("invalid port range: " + ports)
	}
	return startPort, endPort, nil
}

// parsePortNumber parses rawPort into an int, unwrapping strconv errors
// and returning a single "out of range" error for any value outside 0–65535.
func parsePortNumber(rawPort string) (int, error) {
	if rawPort == "" {
		return 0, errors.New("value is empty")
	}
	port, err := strconv.ParseInt(rawPort, 10, 0)
	if err != nil {
		var numErr *strconv.NumError
		if errors.As(err, &numErr) {
			err = numErr.Err
		}
		return 0, err
	}
	if port < 0 || port > 65535 {
		return 0, errors.New("value out of range (0–65535)")
	}

	return int(port), nil
}
