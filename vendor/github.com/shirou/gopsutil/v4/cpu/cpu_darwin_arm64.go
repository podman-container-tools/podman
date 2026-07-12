// SPDX-License-Identifier: BSD-3-Clause
//go:build darwin && arm64

package cpu

import (
	"encoding/binary"
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
			length := corefoundation.CFDataGetLength(uintptr(pCoreRef))
			data := corefoundation.CFDataGetBytePtr(uintptr(pCoreRef))

			// composite uint32 from the byte array
			buf := unsafe.Slice((*byte)(data), length)

			// combine the bytes into a uint32 value
			b := buf[length-8 : length-4]
			pCoreHz = binary.LittleEndian.Uint32(b)
			corefoundation.CFRelease(uintptr(pCoreRef))
			iokit.IOObjectRelease(service)
			break
		}

		iokit.IOObjectRelease(service)
	}

	return float64(pCoreHz / 1_000_000), nil
}
