package define

import (
	"fmt"
	"strings"
)

type VMType int64

const (
	QemuVirt VMType = iota
	WSLVirt
	AppleHvVirt
	HyperVVirt
	LibKrun
	UnknownVirt
)

// these constants are not exported due to a conflict with the constants defined
// in define/machine_artifact.go
const (
	wsl     = "wsl"
	qemu    = "qemu"
	appleHV = "applehv"
	hyperV  = "hyperv"
	libkrun = "libkrun"
)

func (v VMType) String() string {
	switch v {
	case WSLVirt:
		return wsl
	case AppleHvVirt:
		return appleHV
	case HyperVVirt:
		return hyperV
	case LibKrun:
		return libkrun
	}
	return qemu
}

// DiskType returns a string representation that matches the OCI artifact
// type on the container image registry
func (v VMType) DiskType() string {
	switch v {
	case WSLVirt:
		return wsl
		// Both AppleHV and Libkrun use same raw disk flavor
	case AppleHvVirt, LibKrun:
		return appleHV
	case HyperVVirt:
		return hyperV
	}
	return qemu
}

func (v VMType) ImageFormat() ImageFormat {
	switch v {
	case WSLVirt:
		return Tar
	case AppleHvVirt:
		return Raw
	case HyperVVirt:
		return Vhdx
	case LibKrun:
		return Raw
	}
	return Qcow
}

func ParseVMType(input string, emptyFallback VMType) (VMType, error) {
	switch strings.TrimSpace(strings.ToLower(input)) {
	case qemu:
		return QemuVirt, nil
	case wsl:
		return WSLVirt, nil
	case appleHV:
		return AppleHvVirt, nil
	case libkrun:
		return LibKrun, nil
	case hyperV:
		return HyperVVirt, nil
	case "":
		return emptyFallback, nil
	default:
		return UnknownVirt, fmt.Errorf("unknown VMType `%s`", input)
	}
}
