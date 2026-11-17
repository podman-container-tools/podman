//go:build windows

package ioutils

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// rename renames oldpath to newpath, replacing newpath if it exists.
//
// It tries os.Rename (MoveFileEx) first. That fails with "Access is denied"
// when other processes have newpath open, even with FILE_SHARE_DELETE. In
// that case it retries with POSIX semantics, which replaces newpath as long
// as those processes opened it with FILE_SHARE_DELETE.
//
// If the OS or file system doesn't support POSIX semantics (older Windows,
// FAT, some network shares) the os.Rename error is returned.
func rename(oldpath, newpath string) error {
	err := os.Rename(oldpath, newpath)
	if err == nil || !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return err
	}

	perr := posixRename(oldpath, newpath)
	if perr == nil {
		return nil
	}
	if errors.Is(perr, windows.ERROR_INVALID_PARAMETER) ||
		errors.Is(perr, windows.ERROR_NOT_SUPPORTED) ||
		errors.Is(perr, windows.ERROR_INVALID_FUNCTION) {
		return err
	}
	return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: perr}
}

func posixRename(oldpath, newpath string) error {
	from, err := windows.UTF16PtrFromString(oldpath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16FromString(newpath)
	if err != nil {
		return err
	}

	// Open the file using `CreateFileW`
	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew
	h, err := windows.CreateFile(from,
		windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		// With OPEN_EXISTING CreateFile fails if the file doesn't exist
		windows.OPEN_EXISTING,
		// FILE_FLAG_OPEN_REPARSE_POINT renames a symlink itself, like MoveFileEx.
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h) //nolint:errcheck

	// Create a `FILE_RENAME_INFORMATION` structure with `FILE_RENAME_POSIX_SEMANTICS`
	// https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information
	type fileRenameInfo struct {
		Flags          uint32
		RootDirectory  windows.Handle
		FileNameLength uint32
		// In C FileName is a variable-length array that must be stored inside the
		// structure, so it's sized for the longest path Windows supports.
		FileName [windows.MAX_LONG_PATH]uint16
	}
	if len(to) > len(fileRenameInfo{}.FileName) {
		return windows.ERROR_FILENAME_EXCED_RANGE
	}
	info := new(fileRenameInfo)
	info.Flags = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	// FileNameLength is in bytes and excludes the terminating NUL.
	info.FileNameLength = uint32(len(to)-1) * 2
	copy(info.FileName[:], to)

	// Finally do the actual renaming with `SetFileInformationByHandle`
	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-setfileinformationbyhandle
	return windows.SetFileInformationByHandle(h, windows.FileRenameInfoEx,
		(*byte)(unsafe.Pointer(info)), uint32(unsafe.Sizeof(*info)))
}
