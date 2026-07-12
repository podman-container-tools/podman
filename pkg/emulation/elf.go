//go:build linux && !remote

package emulation

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

type elfPlatform struct {
	platform string
	osabi    []elf.OSABI
	class    elf.Class
	data     elf.Data
	alsoNone bool // also try with data=none,version=0
	machine  elf.Machine
	flags    []uint32
}

var (
	// knownELFPlatformHeaders is a mapping from target platform names and
	// plausible headers for the binaries built for those platforms.  Call
	// getKnownELFPlatformHeaders() instead of reading this map directly.
	knownELFPlatformHeaders     = make(map[string][][]byte)
	knownELFPlatformHeadersOnce sync.Once
	// knownELFPlatforms is a table of target platforms that we built a
	// trivial program for, and the other fields are filled in based on
	// what we got when we ran eu-readelf -h against the results.
	knownELFPlatforms = []elfPlatform{
		{
			platform: "linux/386",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS32,
			data:     elf.ELFDATA2LSB,
			alsoNone: true,
			machine:  elf.EM_386,
		},
		{
			platform: "linux/amd64",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS64,
			data:     elf.ELFDATA2LSB,
			alsoNone: true,
			machine:  elf.EM_X86_64,
		},
		{
			platform: "linux/arm",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS32,
			data:     elf.ELFDATA2LSB,
			machine:  elf.EM_ARM,
		},
		{
			platform: "linux/arm64",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS64,
			data:     elf.ELFDATA2LSB,
			machine:  elf.EM_AARCH64,
		},
		{
			platform: "linux/arm64be",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS64,
			data:     elf.ELFDATA2MSB,
			machine:  elf.EM_AARCH64,
		},
		{
			platform: "linux/loong64",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS64,
			data:     elf.ELFDATA2LSB,
			machine:  elf.EM_LOONGARCH,
		},
		{
			platform: "linux/mips",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS32,
			data:     elf.ELFDATA2MSB,
			machine:  elf.EM_MIPS,
			flags:    []uint32{0, 2}, // elf.EF_MIPS_PIC set, or not
		},
		{
			platform: "linux/mipsle",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS32,
			data:     elf.ELFDATA2LSB,
			machine:  elf.EM_MIPS_RS3_LE,
			flags:    []uint32{0, 2}, // elf.EF_MIPS_PIC set, or not
		},
		{
			platform: "linux/mips64",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS64,
			data:     elf.ELFDATA2MSB,
			machine:  elf.EM_MIPS,
			flags:    []uint32{0, 2}, // elf.EF_MIPS_PIC set, or not
		},
		{
			platform: "linux/mips64le",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS64,
			data:     elf.ELFDATA2LSB,
			machine:  elf.EM_MIPS_RS3_LE,
			flags:    []uint32{0, 2}, // elf.EF_MIPS_PIC set, or not
		},
		{
			platform: "linux/ppc",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS32,
			data:     elf.ELFDATA2MSB,
			machine:  elf.EM_PPC,
		},
		{
			platform: "linux/ppc64",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS64,
			data:     elf.ELFDATA2MSB,
			machine:  elf.EM_PPC64,
		},
		{
			platform: "linux/ppc64le",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS64,
			data:     elf.ELFDATA2LSB,
			machine:  elf.EM_PPC64,
		},
		{
			platform: "linux/riscv32",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS32,
			data:     elf.ELFDATA2LSB,
			machine:  elf.EM_RISCV,
		},
		{
			platform: "linux/riscv64",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS64,
			data:     elf.ELFDATA2LSB,
			machine:  elf.EM_RISCV,
		},
		{
			platform: "linux/s390x",
			osabi:    []elf.OSABI{elf.ELFOSABI_NONE, elf.ELFOSABI_LINUX},
			class:    elf.ELFCLASS64,
			data:     elf.ELFDATA2MSB,
			machine:  elf.EM_S390,
		},
	}
)

// header generates an approximation of what the initial N bytes of a binary
// built for a given target looks like
func (e *elfPlatform) header() ([][]byte, error) {
	var headers [][]byte
	osabi := e.osabi
	if len(osabi) == 0 {
		osabi = []elf.OSABI{elf.ELFOSABI_NONE}
	}
	for i := range osabi {
		flags := e.flags
		if len(flags) == 0 {
			flags = []uint32{0}
		}
		for f := range flags {
			var endian binary.ByteOrder
			var entrySize, phoffSize, shoffSize int
			header := make([]byte, 40)
			copy(header, elf.ELFMAG)
			switch e.class {
			case elf.ELFCLASS32:
				entrySize, phoffSize, shoffSize = 2, 2, 2
			case elf.ELFCLASS64:
				entrySize, phoffSize, shoffSize = 4, 4, 4
			}
			switch e.data {
			case elf.ELFDATA2LSB:
				endian = binary.LittleEndian
			case elf.ELFDATA2MSB:
				endian = binary.BigEndian
			default:
				return nil, fmt.Errorf("internal error in entry for %q", e.platform)
			}
			header[elf.EI_OSABI] = byte(osabi[i])
			header[elf.EI_CLASS] = byte(e.class)
			header[elf.EI_DATA] = byte(e.data)
			header[elf.EI_VERSION] = byte(elf.EV_CURRENT)
			header[elf.EI_ABIVERSION] = 0
			endian.PutUint16(header[16:], uint16(elf.ET_EXEC))
			endian.PutUint16(header[18:], uint16(e.machine))
			endian.PutUint32(header[20:], uint32(elf.EV_CURRENT))
			endian.PutUint32(header[24+entrySize+phoffSize+shoffSize:], flags[f])
			headers = append(headers, append([]byte{}, header...))
			if e.alsoNone {
				header[elf.EI_DATA] = byte(elf.ELFDATANONE)
				header[elf.EI_VERSION] = byte(elf.EV_NONE)
				endian.PutUint32(header[20:], uint32(elf.EV_NONE))
				headers = append(headers, append([]byte{}, header...))
			}
		}
	}
	return headers, nil
}

func getKnownELFPlatformHeaders() map[string][][]byte {
	knownELFPlatformHeadersOnce.Do(func() {
		for _, p := range knownELFPlatforms {
			headerList, err := p.header()
			if err != nil {
				logrus.Errorf("generating headers for %q: %v\n", p.platform, err)
				continue
			}
			knownELFPlatformHeaders[p.platform] = headerList
		}
	})
	return knownELFPlatformHeaders
}
