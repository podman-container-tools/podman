//go:build !windows

package system

import (
	"os"
	"syscall"
)

// Lstat takes a path to a file and returns
// a system.StatT type pertaining to that file.
//
// Throws an error if the file does not exist
func Lstat(path string) (*StatT, error) {
	s := &syscall.Stat_t{}
	if err := syscall.Lstat(path, s); err != nil {
		return nil, &os.PathError{Op: "Lstat", Path: path, Err: err}
	}
	return fromStatT(s)
}

// RootLstat takes fsPath within root and returns
// a system.StatT type pertaining to that file.
func RootLstat(root *os.Root, fsPath string) (*StatT, error) {
	fi, err := root.Lstat(fsPath)
	if err != nil {
		return nil, err
	}
	return fromStatT(fi.Sys().(*syscall.Stat_t))
}
