package createpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CleanAbsPath removes any ".." and "." from the path
// and ensures it starts with string(os.PathSeparator).  If the path refers to the root
// directory, it returns string(os.PathSeparator).
func CleanAbsPath(path string) string {
	return filepath.Clean(string(os.PathSeparator) + path)
}

// SplitPath takes a file path as input and returns two components: dir and base.
// Differently than filepath.Split(), this function handles some edge cases,
// designed to be safely used for creating files in untrusted paths, while interpreting
// them restricted to a root.
//
// The returned dir always starts with string(os.PathSeparator) (relative paths are interpreted
// relative to string(os.PathSeparator), not to $PWD), and contains no ".." or "." components.
//
// The returned base value is never empty, it never contains any slash and the
// value ".."; it can be "." only if the path refers to the root directory.
//
// The caller is expected to open / parse the returned dir restricted to a root
// (using securejoin.SecureJoin, or root.Open after avoiding the leading string(os.PathSeparator)),
// and then base can be resolved against the opened parent directory without worrying about the root
// any more (because base is never "..").
//
// Warnings:
//   - The caller must resolve dir using code that restricts the resolution to the desired root:
//     this function does not resolve symlinks nor prevent existence of escaping symlinks
//   - Within dir, base can still refer to an existing symbolic link; the caller must ensure
//     that any operations don't resolve such symbolic links.
func SplitPath(path string) (string, string, error) {
	path = CleanAbsPath(path)
	dir, base := filepath.Split(path)
	if base == "" {
		base = "."
	}
	// Remove trailing slashes from dir, but make sure that "/" is preserved.
	dir = strings.TrimSuffix(dir, string(os.PathSeparator))
	if dir == "" {
		dir = string(os.PathSeparator)
	}

	if strings.Contains(base, string(os.PathSeparator)) {
		// This should never happen, but be safe as the base is passed to *at syscalls.
		return "", "", fmt.Errorf("internal error: SplitPath(%q) contains a path separator", path)
	}
	return dir, base, nil
}
