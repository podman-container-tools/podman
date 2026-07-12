//go:build !libsqlite3 || sqlite_serialize
// +build !libsqlite3 sqlite_serialize

package sqlite3

/*
#ifndef USE_LIBSQLITE3
#include "sqlite3-binding.h"
#else
#include <sqlite3.h>
#endif
#include <stdlib.h>
#include <stdint.h>
*/
import "C"

import (
	"fmt"
	"math"
	"unsafe"
)

// Serialize returns a byte slice that is a serialization of the database.
//
// See https://www.sqlite.org/c3ref/serialize.html
func (c *SQLiteConn) Serialize(schema string) ([]byte, error) {
	if schema == "" {
		schema = "main"
	}
	var zSchema *C.char
	zSchema = C.CString(schema)
	defer C.free(unsafe.Pointer(zSchema))

	var sz C.sqlite3_int64
	ptr := C.sqlite3_serialize(c.db, zSchema, &sz, 0)
	if ptr == nil {
		return nil, fmt.Errorf("serialize failed")
	}
	defer C.sqlite3_free(unsafe.Pointer(ptr))

	if sz > C.sqlite3_int64(math.MaxInt) {
		return nil, fmt.Errorf("serialized database is too large (%d bytes)", sz)
	}

	res := make([]byte, int(sz))
	copy(res, unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(sz)))
	return res, nil
}

// Deserialize causes the connection to disconnect from the current database and
// then re-open as an in-memory database based on the contents of the byte slice.
//
// See https://www.sqlite.org/c3ref/deserialize.html
func (c *SQLiteConn) Deserialize(b []byte, schema string) error {
	if schema == "" {
		schema = "main"
	}
	var zSchema *C.char
	zSchema = C.CString(schema)
	defer C.free(unsafe.Pointer(zSchema))

	tmpBuf := (*C.uchar)(C.sqlite3_malloc64(C.sqlite3_uint64(len(b))))
	copy(unsafe.Slice((*byte)(unsafe.Pointer(tmpBuf)), len(b)), b)

	rc := C.sqlite3_deserialize(c.db, zSchema, tmpBuf, C.sqlite3_int64(len(b)),
		C.sqlite3_int64(len(b)), C.SQLITE_DESERIALIZE_FREEONCLOSE)
	if rc != C.SQLITE_OK {
		return fmt.Errorf("deserialize failed with return %v", rc)
	}
	return nil
}
