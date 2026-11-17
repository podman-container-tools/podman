//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package vmconfigs

import "strings"

func pathsFromVolume(volume string) []string {
	return strings.SplitN(volume, ":", 3)
}

func extractTargetPath(paths []string) string {
	if len(paths) > 1 {
		return paths[1]
	}
	return paths[0]
}
