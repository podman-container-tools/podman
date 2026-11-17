package vmconfigs

import (
	"strings"

	"go.podman.io/storage/pkg/regexp"
)

var (
	driveLetterMatcher = regexp.Delayed(`^(?:\\\\[.?]\\)?[a-zA-Z]$`)
	dedupRegex         = regexp.Delayed(`//+`)
)

func pathsFromVolume(volume string) []string {
	paths := strings.SplitN(volume, ":", 3)
	if len(paths) > 1 && driveLetterMatcher.MatchString(paths[0]) {
		paths = strings.SplitN(volume, ":", 4)
		paths = append([]string{paths[0] + ":" + paths[1]}, paths[2:]...)
	}
	return paths
}

func extractTargetPath(paths []string) string {
	if len(paths) > 1 {
		return paths[1]
	}
	target := strings.ReplaceAll(paths[0], "\\", "/")
	target = strings.ReplaceAll(target, ":", "/")
	if strings.HasPrefix(target, "//./") || strings.HasPrefix(target, "//?/") {
		target = target[4:]
	}
	return dedupRegex.ReplaceAllLiteralString("/"+target, "/")
}
