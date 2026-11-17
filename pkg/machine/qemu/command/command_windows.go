//go:build windows

package command

import (
	"strconv"

	"go.podman.io/common/pkg/strongunits"
)

// SetMemory adds the specified amount of memory for the machine
func (q *QemuCmd) SetMemory(m strongunits.MiB) {
	serializedMem := strconv.FormatUint(uint64(m), 10)
	*q = append(*q, "-m", serializedMem)
}
