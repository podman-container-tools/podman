//go:build linux && cgo && libsubid

package idtools

import (
	"errors"
	"os/user"
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

// helper for stderr to avoid referencing C.stderr from Go code,
// which breaks cgo on musl due to stderr being declared as FILE *const
static FILE *subid_stderr(void) {
    return stderr;
}

#if !defined(SUBID_ABI_MAJOR) || (SUBID_ABI_MAJOR < 4)
# define subid_init libsubid_init
# define subid_get_uid_ranges get_subuid_ranges
# define subid_get_gid_ranges get_subgid_ranges
#endif

*/
import "C"

var onceInit sync.Once

func readSubid(username string, isUser bool) ([]subIDRange, error) {
	var ret []subIDRange
	uidstr := ""

	if username == "ALL" {
		return nil, errors.New("username ALL not supported")
	}

	if u, err := user.Lookup(username); err == nil {
		uidstr = u.Uid
	}

	onceInit.Do(func() {
		C.subid_init(C.CString("storage"), C.subid_stderr())
	})

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
	defer C.free(unsafe.Pointer(cRanges))

	for i := 0; i < int(nRanges); i++ {
		r := C.get_range(cRanges, C.int(i))
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
