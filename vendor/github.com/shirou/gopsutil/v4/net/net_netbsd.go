// SPDX-License-Identifier: BSD-3-Clause
//go:build netbsd

package net

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/shirou/gopsutil/v4/internal/common"
)

var portMatch = regexp.MustCompile(`(.*)\.(\d+)$`)

// parseNetstat parses the output of "netstat -inb" (mode "inb") or
// "netstat -ind" (mode "ind") and merges results into iocs.
//
// NetBSD netstat column layout (0-indexed fields after strings.Fields):
//
//	-inb with Address    (6 fields):  Name Mtu Network Address   Ibytes  Obytes
//	-inb without Address (5 fields):  Name Mtu Network          Ibytes  Obytes
//
//	-ind with Address    (11 fields): Name Mtu Network Address   Ipkts Ierrs Idrops Opkts Oerrs Colls Odrops
//	-ind without Address (10 fields): Name Mtu Network          Ipkts Ierrs Idrops Opkts Oerrs Colls Odrops
//
// The Address field is present for non-loopback interfaces and absent for
// loopback (lo0). We detect this via field count and set base accordingly.
//
// Reference: https://man.netbsd.org/netstat.1
func parseNetstat(output, mode string, iocs map[string]IOCountersStat) error {
	// Minimum field counts when Address is absent.
	minFields := map[string]int{
		"inb": 5,
		"ind": 10,
	}
	// Field count when Address is present (base = 1).
	addrFields := map[string]int{
		"inb": 6,
		"ind": 11,
	}

	seen := make([]string, 0)

	for _, line := range strings.Split(output, "\n") {
		values := strings.Fields(line)
		if len(values) < 1 || values[0] == "Name" {
			continue
		}
		if common.StringsHas(seen, values[0]) {
			continue
		}
		if len(values) < minFields[mode] {
			continue
		}

		base := 0
		if len(values) >= addrFields[mode] {
			base = 1
		}

		seen = append(seen, values[0])

		n, present := iocs[values[0]]
		if !present {
			n = IOCountersStat{Name: values[0]}
		}

		switch mode {
		case "inb":
			recv, err := parseUint(values[base+3])
			if err != nil {
				return err
			}
			sent, err := parseUint(values[base+4])
			if err != nil {
				return err
			}
			n.BytesRecv = recv
			n.BytesSent = sent

		case "ind":
			// Ipkts Ierrs Idrops Opkts Oerrs Colls Odrops
			pktsRecv, err := parseUint(values[base+3])
			if err != nil {
				return err
			}
			errin, err := parseUint(values[base+4])
			if err != nil {
				return err
			}
			dropin, err := parseUint(values[base+5])
			if err != nil {
				return err
			}
			pktsSent, err := parseUint(values[base+6])
			if err != nil {
				return err
			}
			errout, err := parseUint(values[base+7])
			if err != nil {
				return err
			}
			// values[base+8] = Colls (not mapped to IOCountersStat)
			dropout, err := parseUint(values[base+9])
			if err != nil {
				return err
			}
			n.PacketsRecv = pktsRecv
			n.Errin = errin
			n.Dropin = dropin
			n.PacketsSent = pktsSent
			n.Errout = errout
			n.Dropout = dropout
		}

		iocs[n.Name] = n
	}
	return nil
}

func parseUint(s string) (uint64, error) {
	if s == "-" {
		return 0, nil
	}
	return strconv.ParseUint(s, 10, 64)
}

// Deprecated: use process.PidsWithContext instead
func PidsWithContext(_ context.Context) ([]int32, error) {
	return nil, common.ErrNotImplementedError
}

func IOCountersWithContext(ctx context.Context, pernic bool) ([]IOCountersStat, error) {
	netstat, err := exec.LookPath("netstat")
	if err != nil {
		return nil, err
	}

	outBytes, err := invoke.CommandWithContext(ctx, netstat, "-inb")
	if err != nil {
		return nil, err
	}
	outPackets, err := invoke.CommandWithContext(ctx, netstat, "-ind")
	if err != nil {
		return nil, err
	}

	iocs := make(map[string]IOCountersStat)

	if err := parseNetstat(string(outBytes), "inb", iocs); err != nil {
		return nil, err
	}
	if err := parseNetstat(string(outPackets), "ind", iocs); err != nil {
		return nil, err
	}

	ret := make([]IOCountersStat, 0, len(iocs))
	for _, ioc := range iocs {
		ret = append(ret, ioc)
	}

	if !pernic {
		return getIOCountersAll(ret), nil
	}

	return ret, nil
}

func IOCountersByFileWithContext(ctx context.Context, pernic bool, _ string) ([]IOCountersStat, error) {
	return IOCountersWithContext(ctx, pernic)
}

func FilterCountersWithContext(_ context.Context) ([]FilterStat, error) {
	return nil, common.ErrNotImplementedError
}

func ConntrackStatsWithContext(_ context.Context, _ bool) ([]ConntrackStat, error) {
	return nil, common.ErrNotImplementedError
}

func ProtoCountersWithContext(_ context.Context, _ []string) ([]ProtoCountersStat, error) {
	return nil, common.ErrNotImplementedError
}

func parseNetstatLine(line string) (ConnectionStat, error) {
	f := strings.Fields(line)
	if len(f) < 5 {
		return ConnectionStat{}, fmt.Errorf("wrong line,%s", line)
	}

	var netType, netFamily uint32
	switch f[0] {
	case "tcp":
		netType = syscall.SOCK_STREAM
		netFamily = syscall.AF_INET
	case "udp":
		netType = syscall.SOCK_DGRAM
		netFamily = syscall.AF_INET
	case "tcp6":
		netType = syscall.SOCK_STREAM
		netFamily = syscall.AF_INET6
	case "udp6":
		netType = syscall.SOCK_DGRAM
		netFamily = syscall.AF_INET6
	default:
		return ConnectionStat{}, fmt.Errorf("unknown type, %s", f[0])
	}

	laddr, raddr, err := parseNetstatAddr(f[3], f[4], netFamily)
	if err != nil {
		return ConnectionStat{}, fmt.Errorf("failed to parse netaddr, %s %s", f[3], f[4])
	}

	n := ConnectionStat{
		Fd:     uint32(0), // not supported
		Family: uint32(netFamily),
		Type:   uint32(netType),
		Laddr:  laddr,
		Raddr:  raddr,
		Pid:    int32(0), // not supported
	}
	if len(f) == 6 {
		n.Status = f[5]
	}

	return n, nil
}

func parseAddr(l string, family uint32) (Addr, error) {
	matches := portMatch.FindStringSubmatch(l)
	if matches == nil {
		return Addr{}, fmt.Errorf("wrong addr, %s", l)
	}
	host := matches[1]
	port := matches[2]
	if host == "*" {
		switch family {
		case syscall.AF_INET:
			host = "0.0.0.0"
		case syscall.AF_INET6:
			host = "::"
		default:
			return Addr{}, fmt.Errorf("unknown family, %d", family)
		}
	}
	lport, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return Addr{}, err
	}
	return Addr{IP: host, Port: uint32(lport)}, nil
}

func parseNetstatAddr(local, remote string, family uint32) (laddr, raddr Addr, err error) {
	laddr, err = parseAddr(local, family)
	if err != nil {
		return laddr, raddr, err
	}
	if remote != "*.*" {
		raddr, err = parseAddr(remote, family)
		if err != nil {
			return laddr, raddr, err
		}
	}
	return laddr, raddr, err
}

func ConnectionsWithContext(ctx context.Context, kind string) ([]ConnectionStat, error) {
	var ret []ConnectionStat

	args := []string{"-na"}
	switch strings.ToLower(kind) {
	default:
		fallthrough
	case "", "all", "inet":
		// nothing to add
	case "inet4":
		args = append(args, "-finet")
	case "inet6":
		args = append(args, "-finet6")
	case "tcp":
		args = append(args, "-ptcp")
	case "tcp4":
		args = append(args, "-ptcp", "-finet")
	case "tcp6":
		args = append(args, "-ptcp", "-finet6")
	case "udp":
		args = append(args, "-pudp")
	case "udp4":
		args = append(args, "-pudp", "-finet")
	case "udp6":
		args = append(args, "-pudp", "-finet6")
	case "unix":
		return ret, common.ErrNotImplementedError
	}

	netstat, err := exec.LookPath("netstat")
	if err != nil {
		return nil, err
	}
	out, err := invoke.CommandWithContext(ctx, netstat, args...)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "tcp") && !strings.HasPrefix(line, "udp") {
			continue
		}
		n, err := parseNetstatLine(line)
		if err != nil {
			continue
		}
		ret = append(ret, n)
	}

	return ret, nil
}

func ConnectionsPidWithContext(_ context.Context, _ string, _ int32) ([]ConnectionStat, error) {
	return nil, common.ErrNotImplementedError
}

func ConnectionsMaxWithContext(_ context.Context, _ string, _ int) ([]ConnectionStat, error) {
	return nil, common.ErrNotImplementedError
}

func ConnectionsPidMaxWithContext(_ context.Context, _ string, _ int32, _ int) ([]ConnectionStat, error) {
	return nil, common.ErrNotImplementedError
}

func ConnectionsWithoutUidsWithContext(ctx context.Context, kind string) ([]ConnectionStat, error) {
	return ConnectionsMaxWithoutUidsWithContext(ctx, kind, 0)
}

func ConnectionsMaxWithoutUidsWithContext(ctx context.Context, kind string, maxConn int) ([]ConnectionStat, error) {
	return ConnectionsPidMaxWithoutUidsWithContext(ctx, kind, 0, maxConn)
}

func ConnectionsPidWithoutUidsWithContext(ctx context.Context, kind string, pid int32) ([]ConnectionStat, error) {
	return ConnectionsPidMaxWithoutUidsWithContext(ctx, kind, pid, 0)
}

func ConnectionsPidMaxWithoutUidsWithContext(ctx context.Context, kind string, pid int32, maxConn int) ([]ConnectionStat, error) {
	return connectionsPidMaxWithoutUidsWithContext(ctx, kind, pid, maxConn)
}

func connectionsPidMaxWithoutUidsWithContext(_ context.Context, _ string, _ int32, _ int) ([]ConnectionStat, error) {
	return nil, common.ErrNotImplementedError
}
