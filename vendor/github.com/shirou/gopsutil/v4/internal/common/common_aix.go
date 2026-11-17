// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package common

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

func BootTimeWithContext(ctx context.Context, invoke Invoker) (btime uint64, err error) {
	ut, err := UptimeWithContext(ctx, invoke)
	if err != nil {
		return 0, err
	}

	if ut <= 0 {
		return 0, errors.New("uptime was not set, so cannot calculate boot time from it")
	}

	return TimeSince(ut), nil
}

// Uses ps to get the elapsed time for PID 1 in DAYS-HOURS:MINUTES:SECONDS format.
// Examples of ps -o etimes -p 1 output:
// 124-01:40:39 (with days)
// 15:03:02 (without days, hours only)
// 01:02 (just-rebooted systems, minutes and seconds)
func UptimeWithContext(ctx context.Context, invoke Invoker) (uint64, error) {
	out, err := invoke.CommandWithContext(ctx, "ps", "-o", "etimes", "-p", "1")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, errors.New("ps output has fewer than 2 rows")
	}

	// Extract the etimes value from the second row, trimming whitespace
	etimes := strings.TrimSpace(lines[1])
	return ParseUptime(etimes), nil
}

// Parses etimes output from ps command into total seconds.
// Handles formats like:
// - "124-01:40:39" (DAYS-HOURS:MINUTES:SECONDS)
// - "15:03:02" (HOURS:MINUTES:SECONDS)
// - "01:02" (MINUTES:SECONDS, from just-rebooted systems)
func ParseUptime(etimes string) uint64 {
	var days, hours, mins, secs uint64

	// Check if days component is present (contains a dash)
	if strings.Contains(etimes, "-") {
		parts := strings.Split(etimes, "-")
		if len(parts) != 2 {
			return 0
		}

		var err error
		days, err = strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return 0
		}

		// Parse the HH:MM:SS portion (after days, must have 3 parts)
		etimes = parts[1]
		timeParts := strings.Split(etimes, ":")
		if len(timeParts) != 3 {
			return 0
		}

		var err2 error
		hours, err2 = strconv.ParseUint(timeParts[0], 10, 64)
		if err2 != nil {
			return 0
		}

		mins, err2 = strconv.ParseUint(timeParts[1], 10, 64)
		if err2 != nil {
			return 0
		}

		secs, err2 = strconv.ParseUint(timeParts[2], 10, 64)
		if err2 != nil {
			return 0
		}
	} else {
		// Parse time portions (either HH:MM:SS or MM:SS) when no days present
		timeParts := strings.Split(etimes, ":")
		switch len(timeParts) {
		case 3:
			// HH:MM:SS format
			var err error
			hours, err = strconv.ParseUint(timeParts[0], 10, 64)
			if err != nil {
				return 0
			}

			mins, err = strconv.ParseUint(timeParts[1], 10, 64)
			if err != nil {
				return 0
			}

			secs, err = strconv.ParseUint(timeParts[2], 10, 64)
			if err != nil {
				return 0
			}
		case 2:
			// MM:SS format (just-rebooted systems)
			var err error
			mins, err = strconv.ParseUint(timeParts[0], 10, 64)
			if err != nil {
				return 0
			}

			secs, err = strconv.ParseUint(timeParts[1], 10, 64)
			if err != nil {
				return 0
			}
		default:
			return 0
		}
	}

	// Convert to total seconds
	totalSeconds := (days * 24 * 60 * 60) + (hours * 60 * 60) + (mins * 60) + secs
	return totalSeconds
}
