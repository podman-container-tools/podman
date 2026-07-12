package vmconfigs

import (
	"regexp"
	"strings"
)

func pathsFromVolume(volume string) []string {
	paths := strings.SplitN(volume, ":", 3)
	driveLetterMatcher := regexp.MustCompile(`^(?:\\\\[.?]\\)?[a-zA-Z]$`)
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
	dedup := regexp.MustCompile(`//+`)
	return dedup.ReplaceAllLiteralString("/"+target, "/")
}
