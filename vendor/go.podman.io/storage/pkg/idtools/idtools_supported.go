//go:build linux && cgo && libsubid

package idtools

import (
	"errors"
	"os/user"
	"runtime"
	"sync"
	"unsafe"
)

/*
#cgo LDFLAGS: -l subid
#include <shadow/subid.h>
#include <stdlib.h>
#include <stdio.h>

struct subid_range get_range(struct subid_range *ranges, int i)
{
    return ranges[i];
}

#if !defined(SUBID_ABI_MAJOR) || (SUBID_ABI_MAJOR < 4)
# define subid_init libsubid_init
# define subid_get_uid_ranges get_subuid_ranges
# define subid_get_gid_ranges get_subgid_ranges
#endif

#if !defined(SUBID_ABI_MAJOR) || (SUBID_ABI_MAJOR < 5)
# define subid_free free
#endif

// Define our own static init function here instead of calling subid_init
// from the go code so we can have the storage string inlined here.
// static variables cannot be referenced from the go code.
static void containers_storage_subid_init() {
    subid_init("storage", stderr);
}

*/
import "C"

var (
	libsubidLock     sync.Mutex
	subidInitialized bool
)

func readSubid(username string, isUser bool) ([]subIDRange, error) {
	var ret []subIDRange
	uidstr := ""

	if username == "ALL" {
		return nil, errors.New("username ALL not supported")
	}

	if u, err := user.Lookup(username); err == nil {
		uidstr = u.Uid
	}

	// libsubid is not thread safe currently, concurrent calls often fail.
	// https://github.com/shadow-maint/shadow/issues/1362
	libsubidLock.Lock()
	defer libsubidLock.Unlock()

	// Avoid the goroutine being switched to different threads during our calls.
	// While right now the library does not seem to depend on any thread local state
	// lets be future proof in case it will be.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if !subidInitialized {
		C.containers_storage_subid_init()
		subidInitialized = true
	}

	cUsername := C.CString(username)
	defer C.free(unsafe.Pointer(cUsername))

	cuidstr := C.CString(uidstr)
	defer C.free(unsafe.Pointer(cuidstr))

	var nRanges C.int
	var cRanges *C.struct_subid_range
	if isUser {
		nRanges = C.subid_get_uid_ranges(cUsername, &cRanges)
		if nRanges <= 0 {
			nRanges = C.subid_get_uid_ranges(cuidstr, &cRanges)
		}
	} else {
		nRanges = C.subid_get_gid_ranges(cUsername, &cRanges)
		if nRanges <= 0 {
			nRanges = C.subid_get_gid_ranges(cuidstr, &cRanges)
		}
	}
	if nRanges < 0 {
		return nil, errors.New("cannot read subids")
	}
	defer C.subid_free(unsafe.Pointer(cRanges))

	for i := C.int(0); i < nRanges; i++ {
		r := C.get_range(cRanges, i)
		newRange := subIDRange{
			Start:  int(r.start),
			Length: int(r.count),
		}
		ret = append(ret, newRange)
	}
	return ret, nil
}

func readSubuid(username string) ([]subIDRange, error) {
	return readSubid(username, true)
}

func readSubgid(username string) ([]subIDRange, error) {
	return readSubid(username, false)
}
