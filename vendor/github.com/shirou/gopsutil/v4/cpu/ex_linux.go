// SPDX-License-Identifier: BSD-3-Clause
//go:build linux

package cpu

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/shirou/gopsutil/v4/internal/common"
)

// ExTimesStat contains the unscaled Linux CPU time counters from /proc/stat.
// Counters are in USER_HZ ticks, rather than the seconds used by TimesStat.
// Divide a counter by ClocksPerSec to get seconds, or use ToTimesStat.
//
// User includes Guest, and Nice includes GuestNice, because the kernel
// accounts guest time into both. Subtract Guest and GuestNice when summing
// all fields.
//
// Counters are reported as read. A counter can decrease, in particular Iowait.
// Check that the new value is not smaller than the old one before you subtract.
type ExTimesStat struct {
	CPU       string `json:"cpu"`
	User      uint64 `json:"user"`
	System    uint64 `json:"system"`
	Idle      uint64 `json:"idle"`
	Nice      uint64 `json:"nice"`
	Iowait    uint64 `json:"iowait"`
	Irq       uint64 `json:"irq"`
	Softirq   uint64 `json:"softirq"`
	Steal     uint64 `json:"steal"`
	Guest     uint64 `json:"guest"`
	GuestNice uint64 `json:"guestNice"`
}

func (c ExTimesStat) String() string {
	data, _ := json.Marshal(c)
	return string(data)
}

// ToTimesStat converts the counters to the seconds returned by the
// package-level Times function.
func (c ExTimesStat) ToTimesStat() TimesStat {
	return TimesStat{
		CPU:       c.CPU,
		User:      float64(c.User) / ClocksPerSec,
		System:    float64(c.System) / ClocksPerSec,
		Idle:      float64(c.Idle) / ClocksPerSec,
		Nice:      float64(c.Nice) / ClocksPerSec,
		Iowait:    float64(c.Iowait) / ClocksPerSec,
		Irq:       float64(c.Irq) / ClocksPerSec,
		Softirq:   float64(c.Softirq) / ClocksPerSec,
		Steal:     float64(c.Steal) / ClocksPerSec,
		Guest:     float64(c.Guest) / ClocksPerSec,
		GuestNice: float64(c.GuestNice) / ClocksPerSec,
	}
}

type ExLinux struct{}

func NewExLinux() *ExLinux {
	return &ExLinux{}
}

func (ex *ExLinux) Times(percpu bool) ([]ExTimesStat, error) {
	return ex.TimesWithContext(context.Background(), percpu)
}

// TimesWithContext returns raw CPU tick counters without floating-point conversion.
// CPU rows are selected and parsed like the package-level Times function; absent optional counters are zero.
// Unlike the legacy API, read errors and invalid selected CPU rows are returned instead of skipped.
func (*ExLinux) TimesWithContext(ctx context.Context, percpu bool) ([]ExTimesStat, error) {
	file, err := os.Open(common.HostProcWithContext(ctx, "stat"))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	ret := []ExTimesStat{}
	if percpu {
		if _, err := reader.ReadString('\n'); err != nil {
			if errors.Is(err, io.EOF) {
				return ret, nil
			}
			return nil, err
		}
	}
	for {
		if percpu {
			prefix, err := reader.Peek(3)
			if err != nil && !errors.Is(err, io.EOF) {
				return nil, err
			}
			if string(prefix) != "cpu" {
				return ret, nil
			}
		}
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if line == "" {
			return ret, nil
		}
		stat, parseErr := parseStatLine(line)
		if parseErr != nil {
			return nil, parseErr
		}
		ret = append(ret, stat)
		if !percpu || errors.Is(err, io.EOF) {
			return ret, nil
		}
	}
}
