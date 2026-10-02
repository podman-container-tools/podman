// SPDX-License-Identifier: BSD-3-Clause
//go:build darwin && arm64

package cpu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/shirou/gopsutil/v4/internal/common"
)

// Keep IOKit and CoreFoundation libraries open for the process lifetime.
// See: https://github.com/shirou/gopsutil/issues/1832
var (
	cpuLibOnce sync.Once
	cpuIOKit   *common.IOKitLib
	cpuCF      *common.CoreFoundationLib
	cpuLibErr  error
)

func initCPULibraries() {
	cpuIOKit, cpuLibErr = common.NewIOKitLib()
	if cpuLibErr != nil {
		return
	}
	cpuCF, cpuLibErr = common.NewCoreFoundationLib()
}

// https://github.com/shoenig/go-m1cpu/blob/v0.1.6/cpu.go
func getFrequency() (float64, error) {
	cpuLibOnce.Do(initCPULibraries)
	if cpuLibErr != nil {
		return 0, cpuLibErr
	}

	iokit := cpuIOKit
	corefoundation := cpuCF

	matching := iokit.IOServiceMatching("AppleARMIODevice")

	var iterator uint32
	if status := iokit.IOServiceGetMatchingServices(common.KIOMainPortDefault, uintptr(matching), &iterator); status != common.KERN_SUCCESS {
		return 0.0, fmt.Errorf("IOServiceGetMatchingServices error=%d", status)
	}
	defer iokit.IOObjectRelease(iterator)

	pCorekey := corefoundation.CFStringCreateWithCString(common.KCFAllocatorDefault, "voltage-states5-sram", common.KCFStringEncodingUTF8)
	defer corefoundation.CFRelease(uintptr(pCorekey))

	var pCoreHz uint32
	for {
		service := iokit.IOIteratorNext(iterator)
		if service <= 0 {
			break
		}

		buf := common.NewCStr(512)
		iokit.IORegistryEntryGetName(service, buf)

		if buf.GoString() == "pmgr" {
			pCoreRef := iokit.IORegistryEntryCreateCFProperty(service, uintptr(pCorekey), common.KCFAllocatorDefault, common.KNilOptions)
			if pCoreRef == nil {
				iokit.IOObjectRelease(service)
				return 0, errors.New("pmgr has no voltage-states5-sram property")
			}
			length := corefoundation.CFDataGetLength(uintptr(pCoreRef))
			data := corefoundation.CFDataGetBytePtr(uintptr(pCoreRef))

			var raw []byte
			if data != nil && length > 0 {
				raw = unsafe.Slice((*byte)(data), length)
			}

			var err error
			pCoreHz, err = parsePCoreHz(raw)
			corefoundation.CFRelease(uintptr(pCoreRef))
			iokit.IOObjectRelease(service)
			if err != nil {
				return 0, err
			}
			break
		}

		iokit.IOObjectRelease(service)
	}

	return float64(pCoreHz / 1_000_000), nil
}

// parsePCoreHz returns the highest P-core frequency in Hz from the raw
// voltage-states5-sram data, which is stored in the second-to-last
// 4-byte little-endian word.
func parsePCoreHz(buf []byte) (uint32, error) {
	if len(buf) < 8 {
		return 0, fmt.Errorf("voltage-states5-sram data too short: %d bytes", len(buf))
	}
	return binary.LittleEndian.Uint32(buf[len(buf)-8 : len(buf)-4]), nil
}
