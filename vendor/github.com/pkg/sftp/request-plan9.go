//go:build plan9
// +build plan9

package sftp

import (
	"syscall"
)

func fakeFileInfoSys() any {
	return &syscall.Dir{}
}

func testOsSys(sys any) error {
	return nil
}
