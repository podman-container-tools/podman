// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package net

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/internal/common"
)

func parseNetstatI(output string) ([]IOCountersStat, error) {
	lines := strings.Split(string(output), "\n")
	ret := make([]IOCountersStat, 0, len(lines)-1)
	exists := make([]string, 0, len(ret))

	// Check first line is header
	if len(lines) > 0 && strings.Fields(lines[0])[0] != "Name" {
		return nil, errors.New("not a 'netstat -i' output")
	}

	for _, line := range lines[1:] {
		values := strings.Fields(line)
		if len(values) < 1 || values[0] == "Name" {
			continue
		}
		if common.StringsHas(exists, values[0]) {
			// skip if already get
			continue
		}
		exists = append(exists, values[0])

		if len(values) < 9 {
			continue
		}

		base := 1
		// sometimes Address is omitted
		if len(values) < 10 {
			base = 0
		}

		parsed := make([]uint64, 0, 5)
		vv := []string{
			values[base+3], // Ipkts == PacketsRecv
			values[base+4], // Ierrs == Errin
			values[base+5], // Opkts == PacketsSent
			values[base+6], // Oerrs == Errout
			values[base+8], // Drops == Dropout
		}

		for _, target := range vv {
			if target == "-" {
				parsed = append(parsed, 0)
				continue
			}

			t, err := strconv.ParseUint(target, 10, 64)
			if err != nil {
				return nil, err
			}
			parsed = append(parsed, t)
		}

		n := IOCountersStat{
			Name:        values[0],
			PacketsRecv: parsed[0],
			Errin:       parsed[1],
			PacketsSent: parsed[2],
			Errout:      parsed[3],
			Dropout:     parsed[4],
		}
		ret = append(ret, n)
	}
	return ret, nil
}

// parseEntstat extracts BytesSent and BytesRecv from entstat output.
// The entstat two-column Transmit/Receive format (including the "Bytes:"
// line) has been stable across AIX 4.3 through 7.3 (over 25 years).
// The output has a two-column layout with Transmit on the left and Receive
// on the right, e.g.:
//
//	Bytes: 3509236040                             Bytes: 4547812126
func parseEntstat(output string) (bytesSent, bytesRecv uint64) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "Bytes:") {
			continue
		}
		// Split on "Bytes:" to get: ["", " 3509236040                             ", " 4547812126"]
		parts := strings.Split(line, "Bytes:")
		if len(parts) >= 2 {
			if v, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64); err == nil {
				bytesSent = v
			}
		}
		if len(parts) >= 3 {
			if v, err := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 64); err == nil {
				bytesRecv = v
			}
		}
		return
	}
	return
}

func IOCountersWithContext(ctx context.Context, pernic bool) ([]IOCountersStat, error) {
	out, err := invoke.CommandWithContext(ctx, "netstat", "-idn")
	if err != nil {
		return nil, err
	}

	iocounters, err := parseNetstatI(string(out))
	if err != nil {
		return nil, err
	}

	// Populate BytesSent/BytesRecv via entstat for each interface.
	// entstat only works on hardware/virtual ethernet adapters; it fails
	// on loopback (lo0) with errno 19, which is silently skipped. Errors
	// on other interfaces are propagated since they indicate a real problem.
	for i := range iocounters {
		entOut, err := invoke.CommandWithContext(ctx, "entstat", iocounters[i].Name)
		if err != nil {
			// entstat fails on loopback (lo0) with errno 19 — this is expected.
			// For other interfaces, propagate the error.
			if iocounters[i].Name == "lo0" {
				continue
			}
			return nil, err
		}
		iocounters[i].BytesSent, iocounters[i].BytesRecv = parseEntstat(string(entOut))
	}

	if !pernic {
		return getIOCountersAll(iocounters), nil
	}
	return iocounters, nil
}
