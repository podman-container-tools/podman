package vmconfigs

import (
	"fmt"
	"strings"
)

type VolumeMountType int

const (
	NineP VolumeMountType = iota
	VirtIOFS
	Unknown
)

func (v VolumeMountType) String() string {
	switch v {
	case NineP:
		return "9p"
	case VirtIOFS:
		return "virtiofs"
	default:
		return "unknown"
	}
}

func extractSourcePath(paths []string) string {
	return paths[0]
}

func extractMountOptions(paths []string) (bool, string) {
	readonly := false
	securityModel := "none"
	if len(paths) > 2 {
		options := paths[2]
		volopts := strings.SplitSeq(options, ",")
		for o := range volopts {
			switch {
			case o == "rw":
				readonly = false
			case o == "ro":
				readonly = true
			case strings.HasPrefix(o, "security_model="):
				securityModel = strings.Split(o, "=")[1]
			default:
				fmt.Printf("Unknown option: %s\n", o)
			}
		}
	}
	return readonly, securityModel
}

func SplitVolume(idx int, volume string) (string, string, string, bool, string) {
	tag := fmt.Sprintf("vol%d", idx)
	paths := pathsFromVolume(volume)
	source := extractSourcePath(paths)
	target := extractTargetPath(paths)
	readonly, securityModel := extractMountOptions(paths)
	return tag, source, target, readonly, securityModel
}
