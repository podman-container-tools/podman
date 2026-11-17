package sftp

import (
	"syscall"
)

func fakeFileInfoSys() any {
	return syscall.Win32FileAttributeData{}
}

func testOsSys(sys any) error {
	return nil
}
