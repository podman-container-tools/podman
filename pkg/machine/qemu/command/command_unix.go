//go:build dragonfly || freebsd || linux || netbsd || openbsd

package command

import (
	"fmt"
	"strconv"

	"go.podman.io/common/pkg/strongunits"
)

// SetMemory adds the specified amount of memory for the machine
func (q *QemuCmd) SetMemory(m strongunits.MiB) {
	serializedMem := strconv.FormatUint(uint64(m), 10)
	// In order to use virtiofsd, we must enable shared memory
	*q = append(*q, "-object", fmt.Sprintf("memory-backend-memfd,id=mem,size=%sM,share=on", serializedMem))
	*q = append(*q, "-m", serializedMem)
}
