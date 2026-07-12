// Copyright (C) 2019 Yasuhiro Matsumoto <mattn.jp@gmail.com>.
// Copyright (C) 2018 G.J.R. Timmer <gjr.timmer@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//go:build cgo
// +build cgo

package sqlite3

/*
#cgo CFLAGS: -std=gnu99
#cgo CFLAGS: -DSQLITE_ENABLE_RTREE
#cgo CFLAGS: -DSQLITE_THREADSAFE=1
#cgo CFLAGS: -DHAVE_USLEEP=1
#cgo CFLAGS: -DSQLITE_ENABLE_FTS3
#cgo CFLAGS: -DSQLITE_ENABLE_FTS3_PARENTHESIS
#cgo CFLAGS: -DSQLITE_TRACE_SIZE_LIMIT=15
#cgo CFLAGS: -DSQLITE_OMIT_DEPRECATED
#cgo CFLAGS: -DSQLITE_DEFAULT_WAL_SYNCHRONOUS=1
#cgo CFLAGS: -DSQLITE_ENABLE_UPDATE_DELETE_LIMIT
#cgo CFLAGS: -Wno-deprecated-declarations
#cgo openbsd CFLAGS: -I/usr/local/include
#cgo openbsd LDFLAGS: -L/usr/local/lib
#ifndef USE_LIBSQLITE3
#include "sqlite3-binding.h"
#else
#include <sqlite3.h>
#endif
#include <stdlib.h>
#include <string.h>

#ifdef __CYGWIN__
# include <errno.h>
#endif

#ifndef SQLITE_OPEN_READWRITE
# define SQLITE_OPEN_READWRITE 0
#endif

#ifndef SQLITE_OPEN_FULLMUTEX
# define SQLITE_OPEN_FULLMUTEX 0
#endif

#ifndef SQLITE_DETERMINISTIC
# define SQLITE_DETERMINISTIC 0
#endif

#if defined(HAVE_PREAD64) && defined(HAVE_PWRITE64)
# undef USE_PREAD
# undef USE_PWRITE
# define USE_PREAD64 1
# define USE_PWRITE64 1
#elif defined(HAVE_PREAD) && defined(HAVE_PWRITE)
# undef USE_PREAD
# undef USE_PWRITE
# define USE_PREAD64 1
# define USE_PWRITE64 1
#endif

static int
_sqlite3_open_v2(const char *filename, sqlite3 **ppDb, int flags, const char *zVfs) {
#ifdef SQLITE_OPEN_URI
  return sqlite3_open_v2(filename, ppDb, flags | SQLITE_OPEN_URI, zVfs);
#else
  return sqlite3_open_v2(filename, ppDb, flags, zVfs);
#endif
}

static int
_sqlite3_bind_text(sqlite3_stmt *stmt, int n, char *p, sqlite3_uint64 np) {
  return sqlite3_bind_text64(stmt, n, p, np, SQLITE_TRANSIENT, SQLITE_UTF8);
}

static int
_sqlite3_bind_blob(sqlite3_stmt *stmt, int n, void *p, sqlite3_uint64 np) {
  return sqlite3_bind_blob64(stmt, n, p, np, SQLITE_TRANSIENT);
}

typedef struct {
  int typ;
  sqlite3_int64 i64;
  double f64;
  const void *ptr;
  int n;
} sqlite3_go_col;

static void
_sqlite3_column_values(sqlite3_stmt *stmt, int ncol, sqlite3_go_col *cols) {
  for (int i = 0; i < ncol; i++) {
    sqlite3_go_col *col = &cols[i];
    col->typ = sqlite3_column_type(stmt, i);
    col->ptr = 0;
    col->n = 0;
    switch (col->typ) {
    case SQLITE_INTEGER:
      col->i64 = sqlite3_column_int64(stmt, i);
      break;
    case SQLITE_FLOAT:
      col->f64 = sqlite3_column_double(stmt, i);
      break;
    case SQLITE_BLOB:
      col->ptr = sqlite3_column_blob(stmt, i);
      col->n = sqlite3_column_bytes(stmt, i);
      break;
    case SQLITE_TEXT:
      col->ptr = sqlite3_column_text(stmt, i);
      col->n = sqlite3_column_bytes(stmt, i);
      break;
    default:
      break;
    }
  }
}

#include <stdio.h>
#include <stdint.h>

static int
_sqlite3_exec(sqlite3* db, const char* pcmd, long long* rowid, long long* changes)
{
  int rv = sqlite3_exec(db, pcmd, 0, 0, 0);
  *rowid = (long long) sqlite3_last_insert_rowid(db);
  *changes = (long long) sqlite3_changes(db);
  return rv;
}

// Combined reset + clear_bindings in a single C call to reduce CGO crossings.
static int
_sqlite3_reset_clear(sqlite3_stmt* stmt)
{
  int rv = sqlite3_reset(stmt);
  sqlite3_clear_bindings(stmt);
  return rv;
}

#ifdef SQLITE_ENABLE_UNLOCK_NOTIFY
extern int _sqlite3_step_blocking(sqlite3_stmt *stmt);
extern int _sqlite3_step_row_blocking(sqlite3_stmt* stmt, long long* rowid, long long* changes);
extern int _sqlite3_prepare_v2_blocking(sqlite3 *db, const char *zSql, int nBytes, sqlite3_stmt **ppStmt, const char **pzTail);
#endif

// Combined prepare+step+finalize for simple exec without parameters.
// Reduces CGO crossings from ~6 to 1 for the common no-args exec case.
static int
_sqlite3_exec_no_args(sqlite3* db, const char* zSql, int nBytes, long long* rowid, long long* changes, const char** pzTail)
{
  sqlite3_stmt *stmt = 0;
  const char *tail = 0;
#ifdef SQLITE_ENABLE_UNLOCK_NOTIFY
  int rv = _sqlite3_prepare_v2_blocking(db, zSql, nBytes, &stmt, &tail);
#else
  int rv = sqlite3_prepare_v2(db, zSql, nBytes, &stmt, &tail);
#endif
  if (rv != SQLITE_OK) {
    *pzTail = 0;
    return rv;
  }
  if (stmt == 0) {
    // Empty statement
    *rowid = 0;
    *changes = 0;
    *pzTail = tail;
    return SQLITE_OK;
  }
#ifdef SQLITE_ENABLE_UNLOCK_NOTIFY
  rv = _sqlite3_step_row_blocking(stmt, rowid, changes);
#else
  rv = sqlite3_step(stmt);
  *rowid = (long long) sqlite3_last_insert_rowid(db);
  *changes = (long long) sqlite3_changes(db);
#endif
  sqlite3_finalize(stmt);
  *pzTail = tail;
  return rv;
}

#ifdef SQLITE_ENABLE_UNLOCK_NOTIFY

static int
_sqlite3_step_internal(sqlite3_stmt *stmt)
{
  return _sqlite3_step_blocking(stmt);
}

static int
_sqlite3_step_row_internal(sqlite3_stmt* stmt, long long* rowid, long long* changes)
{
  return _sqlite3_step_row_blocking(stmt, rowid, changes);
}

static int
_sqlite3_prepare_v2_internal(sqlite3 *db, const char *zSql, int nBytes, sqlite3_stmt **ppStmt, const char **pzTail)
{
  return _sqlite3_prepare_v2_blocking(db, zSql, nBytes, ppStmt, pzTail);
}

#else
static int
_sqlite3_step_internal(sqlite3_stmt *stmt)
{
  return sqlite3_step(stmt);
}

static int
_sqlite3_step_row_internal(sqlite3_stmt* stmt, long long* rowid, long long* changes)
{
  int rv = sqlite3_step(stmt);
  sqlite3* db = sqlite3_db_handle(stmt);
  *rowid = (long long) sqlite3_last_insert_rowid(db);
  *changes = (long long) sqlite3_changes(db);
  return rv;
}

static int
_sqlite3_prepare_v2_internal(sqlite3 *db, const char *zSql, int nBytes, sqlite3_stmt **ppStmt, const char **pzTail)
{
  return sqlite3_prepare_v2(db, zSql, nBytes, ppStmt, pzTail);
}
#endif

void _sqlite3_result_text(sqlite3_context* ctx, const char* s, int n) {
  sqlite3_result_text(ctx, s, n, &free);
}

void _sqlite3_result_blob(sqlite3_context* ctx, const void* b, int l) {
  sqlite3_result_blob(ctx, b, l, SQLITE_TRANSIENT);
}


int _sqlite3_create_function(
  sqlite3 *db,
  const char *zFunctionName,
  int nArg,
  int eTextRep,
  uintptr_t pApp,
  void (*xFunc)(sqlite3_context*,int,sqlite3_value**),
  void (*xStep)(sqlite3_context*,int,sqlite3_value**),
  void (*xFinal)(sqlite3_context*)
) {
  return sqlite3_create_function(db, zFunctionName, nArg, eTextRep, (void*) pApp, xFunc, xStep, xFinal);
}

void callbackTrampoline(sqlite3_context*, int, sqlite3_value**);
void stepTrampoline(sqlite3_context*, int, sqlite3_value**);
void doneTrampoline(sqlite3_context*);

int compareTrampoline(void*, int, char*, int, char*);
int commitHookTrampoline(void*);
void rollbackHookTrampoline(void*);
void updateHookTrampoline(void*, int, char*, char*, sqlite3_int64);

int authorizerTrampoline(void*, int, char*, char*, char*, char*);

#ifdef SQLITE_LIMIT_WORKER_THREADS
# define _SQLITE_HAS_LIMIT
# define SQLITE_LIMIT_LENGTH                    0
# define SQLITE_LIMIT_SQL_LENGTH                1
# define SQLITE_LIMIT_COLUMN                    2
# define SQLITE_LIMIT_EXPR_DEPTH                3
# define SQLITE_LIMIT_COMPOUND_SELECT           4
# define SQLITE_LIMIT_VDBE_OP                   5
# define SQLITE_LIMIT_FUNCTION_ARG              6
# define SQLITE_LIMIT_ATTACHED                  7
# define SQLITE_LIMIT_LIKE_PATTERN_LENGTH       8
# define SQLITE_LIMIT_VARIABLE_NUMBER           9
# define SQLITE_LIMIT_TRIGGER_DEPTH            10
# define SQLITE_LIMIT_WORKER_THREADS           11
# else
# define SQLITE_LIMIT_WORKER_THREADS           11
#endif

static int _sqlite3_limit(sqlite3* db, int limitId, int newLimit) {
#ifndef _SQLITE_HAS_LIMIT
  return -1;
#else
  return sqlite3_limit(db, limitId, newLimit);
#endif
}

#if SQLITE_VERSION_NUMBER < 3012000
static int sqlite3_system_errno(sqlite3 *db) {
  return 0;
}
#endif
*/
import "C"
import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// SQLiteTimestampFormats is timestamp formats understood by both this module
// and SQLite.  The first format in the slice will be used when saving time
// values into the database. When parsing a string from a timestamp or datetime
// column, the formats are tried in order.
var SQLiteTimestampFormats = []string{
	// By default, store timestamps with whatever timezone they come with.
	// When parsed, they will be returned with the same timezone.
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

const (
	columnDate      string = "date"
	columnDatetime  string = "datetime"
	columnTimestamp string = "timestamp"
)

// This variable can be replaced with -ldflags like below:
// go build -ldflags="-X 'github.com/mattn/go-sqlite3.driverName=my-sqlite3'"
var driverName = "sqlite3"

func init() {
	if driverName != "" {
		sql.Register(driverName, &SQLiteDriver{})
	}
}

// Version returns SQLite library version information.
func Version() (libVersion string, libVersionNumber int, sourceID string) {
	libVersion = C.GoString(C.sqlite3_libversion())
	libVersionNumber = int(C.sqlite3_libversion_number())
	sourceID = C.GoString(C.sqlite3_sourceid())
	return libVersion, libVersionNumber, sourceID
}

const (
	// used by authorizer and pre_update_hook
	SQLITE_DELETE = C.SQLITE_DELETE
	SQLITE_INSERT = C.SQLITE_INSERT
	SQLITE_UPDATE = C.SQLITE_UPDATE

	// used by authorzier - as return value
	SQLITE_OK     = C.SQLITE_OK
	SQLITE_IGNORE = C.SQLITE_IGNORE
	SQLITE_DENY   = C.SQLITE_DENY

	// different actions query tries to do - passed as argument to authorizer
	SQLITE_CREATE_INDEX        = C.SQLITE_CREATE_INDEX
	SQLITE_CREATE_TABLE        = C.SQLITE_CREATE_TABLE
	SQLITE_CREATE_TEMP_INDEX   = C.SQLITE_CREATE_TEMP_INDEX
	SQLITE_CREATE_TEMP_TABLE   = C.SQLITE_CREATE_TEMP_TABLE
	SQLITE_CREATE_TEMP_TRIGGER = C.SQLITE_CREATE_TEMP_TRIGGER
	SQLITE_CREATE_TEMP_VIEW    = C.SQLITE_CREATE_TEMP_VIEW
	SQLITE_CREATE_TRIGGER      = C.SQLITE_CREATE_TRIGGER
	SQLITE_CREATE_VIEW         = C.SQLITE_CREATE_VIEW
	SQLITE_CREATE_VTABLE       = C.SQLITE_CREATE_VTABLE
	SQLITE_DROP_INDEX          = C.SQLITE_DROP_INDEX
	SQLITE_DROP_TABLE          = C.SQLITE_DROP_TABLE
	SQLITE_DROP_TEMP_INDEX     = C.SQLITE_DROP_TEMP_INDEX
	SQLITE_DROP_TEMP_TABLE     = C.SQLITE_DROP_TEMP_TABLE
	SQLITE_DROP_TEMP_TRIGGER   = C.SQLITE_DROP_TEMP_TRIGGER
	SQLITE_DROP_TEMP_VIEW      = C.SQLITE_DROP_TEMP_VIEW
	SQLITE_DROP_TRIGGER        = C.SQLITE_DROP_TRIGGER
	SQLITE_DROP_VIEW           = C.SQLITE_DROP_VIEW
	SQLITE_DROP_VTABLE         = C.SQLITE_DROP_VTABLE
	SQLITE_PRAGMA              = C.SQLITE_PRAGMA
	SQLITE_READ                = C.SQLITE_READ
	SQLITE_SELECT              = C.SQLITE_SELECT
	SQLITE_TRANSACTION         = C.SQLITE_TRANSACTION
	SQLITE_ATTACH              = C.SQLITE_ATTACH
	SQLITE_DETACH              = C.SQLITE_DETACH
	SQLITE_ALTER_TABLE         = C.SQLITE_ALTER_TABLE
	SQLITE_REINDEX             = C.SQLITE_REINDEX
	SQLITE_ANALYZE             = C.SQLITE_ANALYZE
	SQLITE_FUNCTION            = C.SQLITE_FUNCTION
	SQLITE_SAVEPOINT           = C.SQLITE_SAVEPOINT
	SQLITE_COPY                = C.SQLITE_COPY
	/*SQLITE_RECURSIVE           = C.SQLITE_RECURSIVE*/
)

// Standard File Control Opcodes
// See: https://www.sqlite.org/c3ref/c_fcntl_begin_atomic_write.html
const (
	SQLITE_FCNTL_LOCKSTATE             = int(1)
	SQLITE_FCNTL_GET_LOCKPROXYFILE     = int(2)
	SQLITE_FCNTL_SET_LOCKPROXYFILE     = int(3)
	SQLITE_FCNTL_LAST_ERRNO            = int(4)
	SQLITE_FCNTL_SIZE_HINT             = int(5)
	SQLITE_FCNTL_CHUNK_SIZE            = int(6)
	SQLITE_FCNTL_FILE_POINTER          = int(7)
	SQLITE_FCNTL_SYNC_OMITTED          = int(8)
	SQLITE_FCNTL_WIN32_AV_RETRY        = int(9)
	SQLITE_FCNTL_PERSIST_WAL           = int(10)
	SQLITE_FCNTL_OVERWRITE             = int(11)
	SQLITE_FCNTL_VFSNAME               = int(12)
	SQLITE_FCNTL_POWERSAFE_OVERWRITE   = int(13)
	SQLITE_FCNTL_PRAGMA                = int(14)
	SQLITE_FCNTL_BUSYHANDLER           = int(15)
	SQLITE_FCNTL_TEMPFILENAME          = int(16)
	SQLITE_FCNTL_MMAP_SIZE             = int(18)
	SQLITE_FCNTL_TRACE                 = int(19)
	SQLITE_FCNTL_HAS_MOVED             = int(20)
	SQLITE_FCNTL_SYNC                  = int(21)
	SQLITE_FCNTL_COMMIT_PHASETWO       = int(22)
	SQLITE_FCNTL_WIN32_SET_HANDLE      = int(23)
	SQLITE_FCNTL_WAL_BLOCK             = int(24)
	SQLITE_FCNTL_ZIPVFS                = int(25)
	SQLITE_FCNTL_RBU                   = int(26)
	SQLITE_FCNTL_VFS_POINTER           = int(27)
	SQLITE_FCNTL_JOURNAL_POINTER       = int(28)
	SQLITE_FCNTL_WIN32_GET_HANDLE      = int(29)
	SQLITE_FCNTL_PDB                   = int(30)
	SQLITE_FCNTL_BEGIN_ATOMIC_WRITE    = int(31)
	SQLITE_FCNTL_COMMIT_ATOMIC_WRITE   = int(32)
	SQLITE_FCNTL_ROLLBACK_ATOMIC_WRITE = int(33)
	SQLITE_FCNTL_LOCK_TIMEOUT          = int(34)
	SQLITE_FCNTL_DATA_VERSION          = int(35)
	SQLITE_FCNTL_SIZE_LIMIT            = int(36)
	SQLITE_FCNTL_CKPT_DONE             = int(37)
	SQLITE_FCNTL_RESERVE_BYTES         = int(38)
	SQLITE_FCNTL_CKPT_START            = int(39)
	SQLITE_FCNTL_EXTERNAL_READER       = int(40)
	SQLITE_FCNTL_CKSM_FILE             = int(41)
)

// SQLiteDriver implements driver.Driver.
type SQLiteDriver struct {
	Extensions  []string
	ConnectHook func(*SQLiteConn) error
}

// SQLiteConn implements driver.Conn.
type SQLiteConn struct {
	mu             sync.Mutex
	db             *C.sqlite3
	loc            *time.Location
	txlock         string
	funcs          []*functionInfo
	aggregators    []*aggInfo
	// Prepared-statement cache. The slice is allocated at Open with a
	// fixed capacity equal to the configured cache size; cap bounds the
	// cache, len is the live count, and entries are ordered LRU-first
	// (index 0 is the oldest, the tail is most recently put). Access
	// requires mu; stmtCacheEnabled is immutable after Open and is the
	// only field safe to read without the lock.
	stmtCache        []*SQLiteStmt
	stmtCacheEnabled bool
}

// SQLiteTx implements driver.Tx.
type SQLiteTx struct {
	c *SQLiteConn
}

// SQLiteStmt implements driver.Stmt.
type SQLiteStmt struct {
	mu          sync.Mutex
	c           *SQLiteConn
	s           *C.sqlite3_stmt
	t           string
	closed      bool
	cls         bool // True if the statement was created by SQLiteConn.Query
	namedParams map[string][3]int
	cacheKey    string
	metadata    *sqliteStmtMetadata
}

type sqliteStmtMetadata struct {
	cols     []string
	decltype []string
}

// SQLiteResult implements sql.Result.
type SQLiteResult struct {
	id      int64
	changes int64
}

// SQLiteRows implements driver.Rows.
type SQLiteRows struct {
	s        *SQLiteStmt
	nc       int32 // Number of columns
	cls      bool  // True if we need to close the parent statement in Close
	cols     []string
	decltype []string
	colvals  *C.sqlite3_go_col
	ctx      context.Context // no better alternative to pass context into Next() method
	closemu  sync.Mutex
}

type functionInfo struct {
	f                 reflect.Value
	argConverters     []callbackArgConverter
	variadicConverter callbackArgConverter
	retConverter      callbackRetConverter
}

func (fi *functionInfo) Call(ctx *C.sqlite3_context, argv []*C.sqlite3_value) {
	args, err := callbackConvertArgs(argv, fi.argConverters, fi.variadicConverter)
	if err != nil {
		callbackError(ctx, err)
		return
	}

	ret := fi.f.Call(args)

	if len(ret) == 2 && ret[1].Interface() != nil {
		callbackError(ctx, ret[1].Interface().(error))
		return
	}

	err = fi.retConverter(ctx, ret[0])
	if err != nil {
		callbackError(ctx, err)
		return
	}
}

type aggInfo struct {
	constructor reflect.Value

	// Active aggregator objects for aggregations in flight. The
	// aggregators are indexed by a counter stored in the aggregation
	// user data space provided by sqlite.
	active map[int64]reflect.Value
	next   int64

	stepArgConverters     []callbackArgConverter
	stepVariadicConverter callbackArgConverter

	doneRetConverter callbackRetConverter
}

func (ai *aggInfo) agg(ctx *C.sqlite3_context) (int64, reflect.Value, error) {
	aggIdx := (*int64)(C.sqlite3_aggregate_context(ctx, C.int(8)))
	if *aggIdx == 0 {
		*aggIdx = ai.next
		ret := ai.constructor.Call(nil)
		if len(ret) == 2 && ret[1].Interface() != nil {
			return 0, reflect.Value{}, ret[1].Interface().(error)
		}
		if ret[0].IsNil() {
			return 0, reflect.Value{}, errors.New("aggregator constructor returned nil state")
		}
		ai.next++
		ai.active[*aggIdx] = ret[0]
	}
	return *aggIdx, ai.active[*aggIdx], nil
}

func (ai *aggInfo) Step(ctx *C.sqlite3_context, argv []*C.sqlite3_value) {
	_, agg, err := ai.agg(ctx)
	if err != nil {
		callbackError(ctx, err)
		return
	}

	args, err := callbackConvertArgs(argv, ai.stepArgConverters, ai.stepVariadicConverter)
	if err != nil {
		callbackError(ctx, err)
		return
	}

	ret := agg.MethodByName("Step").Call(args)
	if len(ret) == 1 && ret[0].Interface() != nil {
		callbackError(ctx, ret[0].Interface().(error))
		return
	}
}

func (ai *aggInfo) Done(ctx *C.sqlite3_context) {
	idx, agg, err := ai.agg(ctx)
	if err != nil {
		callbackError(ctx, err)
		return
	}
	defer func() { delete(ai.active, idx) }()

	ret := agg.MethodByName("Done").Call(nil)
	if len(ret) == 2 && ret[1].Interface() != nil {
		callbackError(ctx, ret[1].Interface().(error))
		return
	}

	err = ai.doneRetConverter(ctx, ret[0])
	if err != nil {
		callbackError(ctx, err)
		return
	}
}

// Commit transaction.
func (tx *SQLiteTx) Commit() error {
	_, err := tx.c.exec(context.Background(), "COMMIT", nil)
	if err != nil {
		// sqlite3 may leave the transaction open in this scenario.
		// However, database/sql considers the transaction complete once we
		// return from Commit() - we must clean up to honour its semantics.
		// We don't know if the ROLLBACK is strictly necessary, but according
		// to sqlite's docs, there is no harm in calling ROLLBACK unnecessarily.
		tx.c.exec(context.Background(), "ROLLBACK", nil)
	}
	return err
}

// Rollback transaction.
func (tx *SQLiteTx) Rollback() error {
	_, err := tx.c.exec(context.Background(), "ROLLBACK", nil)
	return err
}

// RegisterCollation makes a Go function available as a collation.
//
// cmp receives two UTF-8 strings, a and b. The result should be 0 if
// a==b, -1 if a < b, and +1 if a > b.
//
// cmp must always return the same result given the same
// inputs. Additionally, it must have the following properties for all
// strings A, B and C: if A==B then B==A; if A==B and B==C then A==C;
// if A<B then B>A; if A<B and B<C then A<C.
//
// If cmp does not obey these constraints, sqlite3's behavior is
// undefined when the collation is used.
func (c *SQLiteConn) RegisterCollation(name string, cmp func(string, string) int) error {
	handle := newHandle(c, cmp)
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	rv := C.sqlite3_create_collation(c.db, cname, C.SQLITE_UTF8, handle, (*[0]byte)(unsafe.Pointer(C.compareTrampoline)))
	if rv != C.SQLITE_OK {
		return c.lastError()
	}
	return nil
}

// RegisterCommitHook sets the commit hook for a connection.
//
// If the callback returns non-zero the transaction will become a rollback.
//
// If there is an existing commit hook for this connection, it will be
// removed. If callback is nil the existing hook (if any) will be removed
// without creating a new one.
func (c *SQLiteConn) RegisterCommitHook(callback func() int) {
	if callback == nil {
		C.sqlite3_commit_hook(c.db, nil, nil)
	} else {
		C.sqlite3_commit_hook(c.db, (*[0]byte)(C.commitHookTrampoline), newHandle(c, callback))
	}
}

// RegisterRollbackHook sets the rollback hook for a connection.
//
// If there is an existing rollback hook for this connection, it will be
// removed. If callback is nil the existing hook (if any) will be removed
// without creating a new one.
func (c *SQLiteConn) RegisterRollbackHook(callback func()) {
	if callback == nil {
		C.sqlite3_rollback_hook(c.db, nil, nil)
	} else {
		C.sqlite3_rollback_hook(c.db, (*[0]byte)(C.rollbackHookTrampoline), newHandle(c, callback))
	}
}

// RegisterUpdateHook sets the update hook for a connection.
//
// The parameters to the callback are the operation (one of the constants
// SQLITE_INSERT, SQLITE_DELETE, or SQLITE_UPDATE), the database name, the
// table name, and the rowid.
//
// If there is an existing update hook for this connection, it will be
// removed. If callback is nil the existing hook (if any) will be removed
// without creating a new one.
func (c *SQLiteConn) RegisterUpdateHook(callback func(int, string, string, int64)) {
	if callback == nil {
		C.sqlite3_update_hook(c.db, nil, nil)
	} else {
		C.sqlite3_update_hook(c.db, (*[0]byte)(C.updateHookTrampoline), newHandle(c, callback))
	}
}

// RegisterAuthorizer sets the authorizer for connection.
//
// The parameters to the callback are the operation (one of the constants
// SQLITE_INSERT, SQLITE_DELETE, or SQLITE_UPDATE), and 1 to 3 arguments,
// depending on operation. More details see:
// https://www.sqlite.org/c3ref/c_alter_table.html
func (c *SQLiteConn) RegisterAuthorizer(callback func(int, string, string, string) int) {
	if callback == nil {
		C.sqlite3_set_authorizer(c.db, nil, nil)
	} else {
		C.sqlite3_set_authorizer(c.db, (*[0]byte)(C.authorizerTrampoline), newHandle(c, callback))
	}
}

// RegisterFunc makes a Go function available as a SQLite function.
//
// The Go function can have arguments of the following types: any
// numeric type except complex, bool, []byte, string and any.
// any arguments are given the direct translation of the SQLite data type:
// int64 for INTEGER, float64 for FLOAT, []byte for BLOB, string for TEXT.
//
// The function can additionally be variadic, as long as the type of
// the variadic argument is one of the above.
//
// If pure is true. SQLite will assume that the function's return
// value depends only on its inputs, and make more aggressive
// optimizations in its queries.
//
// See _example/go_custom_funcs for a detailed example.
func (c *SQLiteConn) RegisterFunc(name string, impl any, pure bool) error {
	var fi functionInfo
	fi.f = reflect.ValueOf(impl)
	t := fi.f.Type()
	if t.Kind() != reflect.Func {
		return errors.New("Non-function passed to RegisterFunc")
	}
	if t.NumOut() != 1 && t.NumOut() != 2 {
		return errors.New("SQLite functions must return 1 or 2 values")
	}
	if t.NumOut() == 2 && !t.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return errors.New("Second return value of SQLite function must be error")
	}

	numArgs := t.NumIn()
	if t.IsVariadic() {
		numArgs--
	}

	for i := 0; i < numArgs; i++ {
		conv, err := callbackArg(t.In(i))
		if err != nil {
			return err
		}
		fi.argConverters = append(fi.argConverters, conv)
	}

	if t.IsVariadic() {
		conv, err := callbackArg(t.In(numArgs).Elem())
		if err != nil {
			return err
		}
		fi.variadicConverter = conv
		// Pass -1 to sqlite so that it allows any number of
		// arguments. The call helper verifies that the minimum number
		// of arguments is present for variadic functions.
		numArgs = -1
	}

	conv, err := callbackRet(t.Out(0))
	if err != nil {
		return err
	}
	fi.retConverter = conv

	// fi must outlast the database connection, or we'll have dangling pointers.
	c.funcs = append(c.funcs, &fi)

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	opts := C.SQLITE_UTF8
	if pure {
		opts |= C.SQLITE_DETERMINISTIC
	}
	rv := sqlite3CreateFunction(c.db, cname, C.int(numArgs), C.int(opts), newHandle(c, &fi), C.callbackTrampoline, nil, nil)
	if rv != C.SQLITE_OK {
		return c.lastError()
	}
	return nil
}

func sqlite3CreateFunction(db *C.sqlite3, zFunctionName *C.char, nArg C.int, eTextRep C.int, pApp unsafe.Pointer, xFunc unsafe.Pointer, xStep unsafe.Pointer, xFinal unsafe.Pointer) C.int {
	return C._sqlite3_create_function(db, zFunctionName, nArg, eTextRep, C.uintptr_t(uintptr(pApp)), (*[0]byte)(xFunc), (*[0]byte)(xStep), (*[0]byte)(xFinal))
}

// RegisterAggregator makes a Go type available as a SQLite aggregation function.
//
// Because aggregation is incremental, it's implemented in Go with a
// type that has 2 methods: func Step(values) accumulates one row of
// data into the accumulator, and func Done() ret finalizes and
// returns the aggregate value. "values" and "ret" may be any type
// supported by RegisterFunc.
//
// RegisterAggregator takes as implementation a constructor function
// that constructs an instance of the aggregator type each time an
// aggregation begins. The constructor must return a pointer to a
// type, or an interface that implements Step() and Done().
//
// The constructor function and the Step/Done methods may optionally
// return an error in addition to their other return values.
//
// See _example/go_custom_funcs for a detailed example.
func (c *SQLiteConn) RegisterAggregator(name string, impl any, pure bool) error {
	var ai aggInfo
	ai.constructor = reflect.ValueOf(impl)
	t := ai.constructor.Type()
	if t.Kind() != reflect.Func {
		return errors.New("non-function passed to RegisterAggregator")
	}
	if t.NumOut() != 1 && t.NumOut() != 2 {
		return errors.New("SQLite aggregator constructors must return 1 or 2 values")
	}
	if t.NumOut() == 2 && !t.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return errors.New("Second return value of SQLite function must be error")
	}
	if t.NumIn() != 0 {
		return errors.New("SQLite aggregator constructors must not have arguments")
	}

	agg := t.Out(0)
	switch agg.Kind() {
	case reflect.Ptr, reflect.Interface:
	default:
		return errors.New("SQlite aggregator constructor must return a pointer object")
	}
	stepFn, found := agg.MethodByName("Step")
	if !found {
		return errors.New("SQlite aggregator doesn't have a Step() function")
	}
	step := stepFn.Type
	if step.NumOut() != 0 && step.NumOut() != 1 {
		return errors.New("SQlite aggregator Step() function must return 0 or 1 values")
	}
	if step.NumOut() == 1 && !step.Out(0).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return errors.New("type of SQlite aggregator Step() return value must be error")
	}

	stepNArgs := step.NumIn()
	start := 0
	if agg.Kind() == reflect.Ptr {
		// Skip over the method receiver
		stepNArgs--
		start++
	}
	if step.IsVariadic() {
		stepNArgs--
	}
	for i := start; i < start+stepNArgs; i++ {
		conv, err := callbackArg(step.In(i))
		if err != nil {
			return err
		}
		ai.stepArgConverters = append(ai.stepArgConverters, conv)
	}
	if step.IsVariadic() {
		conv, err := callbackArg(step.In(start + stepNArgs).Elem())
		if err != nil {
			return err
		}
		ai.stepVariadicConverter = conv
		// Pass -1 to sqlite so that it allows any number of
		// arguments. The call helper verifies that the minimum number
		// of arguments is present for variadic functions.
		stepNArgs = -1
	}

	doneFn, found := agg.MethodByName("Done")
	if !found {
		return errors.New("SQlite aggregator doesn't have a Done() function")
	}
	done := doneFn.Type
	doneNArgs := done.NumIn()
	if agg.Kind() == reflect.Ptr {
		// Skip over the method receiver
		doneNArgs--
	}
	if doneNArgs != 0 {
		return errors.New("SQlite aggregator Done() function must have no arguments")
	}
	if done.NumOut() != 1 && done.NumOut() != 2 {
		return errors.New("SQLite aggregator Done() function must return 1 or 2 values")
	}
	if done.NumOut() == 2 && !done.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return errors.New("second return value of SQLite aggregator Done() function must be error")
	}

	conv, err := callbackRet(done.Out(0))
	if err != nil {
		return err
	}
	ai.doneRetConverter = conv
	ai.active = make(map[int64]reflect.Value)
	ai.next = 1

	// ai must outlast the database connection, or we'll have dangling pointers.
	c.aggregators = append(c.aggregators, &ai)

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	opts := C.SQLITE_UTF8
	if pure {
		opts |= C.SQLITE_DETERMINISTIC
	}
	rv := sqlite3CreateFunction(c.db, cname, C.int(stepNArgs), C.int(opts), newHandle(c, &ai), nil, C.stepTrampoline, C.doneTrampoline)
	if rv != C.SQLITE_OK {
		return c.lastError()
	}
	return nil
}

// AutoCommit return which currently auto commit or not.
func (c *SQLiteConn) AutoCommit() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return int(C.sqlite3_get_autocommit(c.db)) != 0
}

func (c *SQLiteConn) lastError() error {
	return lastError(c.db)
}

// Note: may be called with db == nil
func lastError(db *C.sqlite3) error {
	rv := C.sqlite3_errcode(db) // returns SQLITE_NOMEM if db == nil
	if rv == C.SQLITE_OK {
		return nil
	}
	extrv := C.sqlite3_extended_errcode(db)    // returns SQLITE_NOMEM if db == nil
	errStr := C.GoString(C.sqlite3_errmsg(db)) // returns "out of memory" if db == nil

	// https://www.sqlite.org/c3ref/system_errno.html
	// sqlite3_system_errno is only meaningful if the error code was SQLITE_CANTOPEN,
	// or it was SQLITE_IOERR and the extended code was not SQLITE_IOERR_NOMEM
	var systemErrno syscall.Errno
	if rv == C.SQLITE_CANTOPEN || (rv == C.SQLITE_IOERR && extrv != C.SQLITE_IOERR_NOMEM) {
		systemErrno = syscall.Errno(C.sqlite3_system_errno(db))
	}

	return Error{
		Code:         ErrNo(rv),
		ExtendedCode: ErrNoExtended(extrv),
		SystemErrno:  systemErrno,
		err:          errStr,
	}
}

// Exec implements Execer.
func (c *SQLiteConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	return c.exec(context.Background(), query, valueToNamedValue(args))
}

func (c *SQLiteConn) exec(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	// Fast path: no args, no context cancellation → single CGO call per statement
	if len(args) == 0 && ctx.Done() == nil {
		return c.execNoArgs(query)
	}

	start := 0
	for {
		s, err := c.prepareWithCache(ctx, query)
		if err != nil {
			return nil, err
		}
		var res driver.Result
		if s.(*SQLiteStmt).s != nil {
			na := s.NumInput()
			if len(args)-start < na {
				s.Close()
				return nil, fmt.Errorf("not enough args to execute query: want %d got %d", na, len(args))
			}
			stmtArgs := stmtArgs(args, start, na)
			res, err = s.(*SQLiteStmt).exec(ctx, stmtArgs)
			if err != nil && err != driver.ErrSkip {
				s.Close()
				return nil, err
			}
			start += na
		}
		tail := s.(*SQLiteStmt).t
		s.Close()
		if tail == "" {
			if res == nil {
				// https://github.com/mattn/go-sqlite3/issues/963
				res = &SQLiteResult{0, 0}
			}
			return res, nil
		}
		query = tail
	}
}

// execNoArgs executes a query with no parameters in a single CGO call per statement.
func (c *SQLiteConn) execNoArgs(query string) (driver.Result, error) {
	var res *SQLiteResult
	for len(query) > 0 {
		var rowid, changes C.longlong
		var tail *C.char
		pquery := C.CString(query)
		rv := C._sqlite3_exec_no_args(c.db, pquery, C.int(len(query)), &rowid, &changes, &tail)
		if tail != nil && *tail != '\000' {
			query = strings.TrimSpace(C.GoString(tail))
		} else {
			query = ""
		}
		C.free(unsafe.Pointer(pquery))
		if rv != C.SQLITE_ROW && rv != C.SQLITE_OK && rv != C.SQLITE_DONE {
			return nil, c.lastError()
		}
		res = &SQLiteResult{id: int64(rowid), changes: int64(changes)}
	}
	if res == nil {
		res = &SQLiteResult{0, 0}
	}
	return res, nil
}

// Query implements Queryer.
func (c *SQLiteConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return c.query(context.Background(), query, valueToNamedValue(args))
}

func (c *SQLiteConn) query(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	start := 0
	for {
		s, err := c.prepareWithCache(ctx, query)
		if err != nil {
			return nil, err
		}
		ss := s.(*SQLiteStmt)
		ss.cls = true
		// sqlite3_prepare_v2 returns SQLITE_OK with a NULL statement handle
		// when the input is empty or contains only whitespace/comments.
		if ss.s == nil {
			tail := ss.t
			ss.Close()
			if tail == "" {
				return &SQLiteRows{cls: true, ctx: ctx}, nil
			}
			query = tail
			continue
		}
		na := s.NumInput()
		if len(args)-start < na {
			ss.Close()
			return nil, fmt.Errorf("not enough args to execute query: want %d got %d", na, len(args)-start)
		}
		stmtArgs := stmtArgs(args, start, na)
		rows, err := ss.query(ctx, stmtArgs)
		if err != nil && err != driver.ErrSkip {
			ss.Close()
			return rows, err
		}
		start += na
		tail := ss.t
		if tail == "" {
			return rows, nil
		}
		rows.Close()
		ss.Close()
		query = tail
	}
}

// Begin transaction.
func (c *SQLiteConn) Begin() (driver.Tx, error) {
	return c.begin(context.Background())
}

func (c *SQLiteConn) begin(ctx context.Context) (driver.Tx, error) {
	if _, err := c.exec(ctx, c.txlock, nil); err != nil {
		return nil, err
	}
	return &SQLiteTx{c}, nil
}

// Open database and return a new connection.
//
// A pragma can take either zero or one argument.
// The argument is may be either in parentheses or it may be separated from
// the pragma name by an equal sign. The two syntaxes yield identical results.
// In many pragmas, the argument is a boolean. The boolean can be one of:
//
//	1 yes true on
//	0 no false off
//
// You can specify a DSN string using a URI as the filename.
//
//	test.db
//	file:test.db?cache=shared&mode=memory
//	:memory:
//	file::memory:
//
//	mode
//	  Access mode of the database.
//	  https://www.sqlite.org/c3ref/open.html
//	  Values:
//	   - ro
//	   - rw
//	   - rwc
//	   - memory
//
//	cache
//	  SQLite Shared-Cache Mode
//	  https://www.sqlite.org/sharedcache.html
//	  Values:
//	    - shared
//	    - private
//
//	immutable=Boolean
//	  The immutable parameter is a boolean query parameter that indicates
//	  that the database file is stored on read-only media. When immutable is set,
//	  SQLite assumes that the database file cannot be changed,
//	  even by a process with higher privilege,
//	  and so the database is opened read-only and all locking and change detection is disabled.
//	  Caution: Setting the immutable property on a database file that
//	  does in fact change can result in incorrect query results and/or SQLITE_CORRUPT errors.
//
// go-sqlite3 adds the following query parameters to those used by SQLite:
//
//	_loc=XXX
//	  Specify location of time format. It's possible to specify "auto".
//
//	_mutex=XXX
//	  Specify mutex mode. XXX can be "no", "full".
//
//	_txlock=XXX
//	  Specify locking behavior for transactions.  XXX can be "immediate",
//	  "deferred", "exclusive".
//
//	_auto_vacuum=X | _vacuum=X
//	  0 | none - Auto Vacuum disabled
//	  1 | full - Auto Vacuum FULL
//	  2 | incremental - Auto Vacuum Incremental
//
//	_busy_timeout=XXX"| _timeout=XXX
//	  Specify value for sqlite3_busy_timeout.
//
//	_case_sensitive_like=Boolean | _cslike=Boolean
//	  https://www.sqlite.org/pragma.html#pragma_case_sensitive_like
//	  Default or disabled the LIKE operation is case-insensitive.
//	  When enabling this options behaviour of LIKE will become case-sensitive.
//
//	_defer_foreign_keys=Boolean | _defer_fk=Boolean
//	  Defer Foreign Keys until outermost transaction is committed.
//
//	_foreign_keys=Boolean | _fk=Boolean
//	  Enable or disable enforcement of foreign keys.
//
//	_ignore_check_constraints=Boolean
//	  This pragma enables or disables the enforcement of CHECK constraints.
//	  The default setting is off, meaning that CHECK constraints are enforced by default.
//
//	_journal_mode=MODE | _journal=MODE
//	  Set journal mode for the databases associated with the current connection.
//	  https://www.sqlite.org/pragma.html#pragma_journal_mode
//
//	_locking_mode=X | _locking=X
//	  Sets the database connection locking-mode.
//	  The locking-mode is either NORMAL or EXCLUSIVE.
//	  https://www.sqlite.org/pragma.html#pragma_locking_mode
//
//	_query_only=Boolean
//	  The query_only pragma prevents all changes to database files when enabled.
//
//	_recursive_triggers=Boolean | _rt=Boolean
//	  Enable or disable recursive triggers.
//
//	_secure_delete=Boolean|FAST
//	  When secure_delete is on, SQLite overwrites deleted content with zeros.
//	  https://www.sqlite.org/pragma.html#pragma_secure_delete
//
//	_synchronous=X | _sync=X
//	  Change the setting of the "synchronous" flag.
//	  https://www.sqlite.org/pragma.html#pragma_synchronous
//
//	_writable_schema=Boolean
//	  When this pragma is on, the SQLITE_MASTER tables in which database
//	  can be changed using ordinary UPDATE, INSERT, and DELETE statements.
//	  Warning: misuse of this pragma can easily result in a corrupt database file.
func (d *SQLiteDriver) Open(dsn string) (driver.Conn, error) {
	if C.sqlite3_threadsafe() == 0 {
		return nil, errors.New("sqlite library was not compiled for thread-safe operation")
	}

	var pkey string

	// Options
	var loc *time.Location
	authCreate := false
	authUser := ""
	authPass := ""
	authCrypt := ""
	authSalt := ""
	mutex := C.int(C.SQLITE_OPEN_FULLMUTEX)
	txlock := "BEGIN"

	// PRAGMA's
	autoVacuum := -1
	busyTimeout := 5000
	caseSensitiveLike := -1
	deferForeignKeys := -1
	foreignKeys := -1
	ignoreCheckConstraints := -1
	var journalMode string
	lockingMode := "NORMAL"
	queryOnly := -1
	recursiveTriggers := -1
	secureDelete := "DEFAULT"
	synchronousMode := "NORMAL"
	writableSchema := -1
	vfsName := ""
	var cacheSize *int64
	stmtCacheSize := 0

	pos := strings.IndexRune(dsn, '?')
	if pos >= 1 {
		params, err := url.ParseQuery(dsn[pos+1:])
		if err != nil {
			return nil, err
		}

		// Authentication
		if _, ok := params["_auth"]; ok {
			authCreate = true
		}
		if val := params.Get("_auth_user"); val != "" {
			authUser = val
		}
		if val := params.Get("_auth_pass"); val != "" {
			authPass = val
		}
		if val := params.Get("_auth_crypt"); val != "" {
			authCrypt = val
		}
		if val := params.Get("_auth_salt"); val != "" {
			authSalt = val
		}

		// _loc
		if val := params.Get("_loc"); val != "" {
			switch strings.ToLower(val) {
			case "auto":
				loc = time.Local
			default:
				loc, err = time.LoadLocation(val)
				if err != nil {
					return nil, fmt.Errorf("Invalid _loc: %v: %v", val, err)
				}
			}
		}

		// _mutex
		if val := params.Get("_mutex"); val != "" {
			switch strings.ToLower(val) {
			case "no":
				mutex = C.SQLITE_OPEN_NOMUTEX
			case "full":
				mutex = C.SQLITE_OPEN_FULLMUTEX
			default:
				return nil, fmt.Errorf("Invalid _mutex: %v", val)
			}
		}

		// _txlock
		if val := params.Get("_txlock"); val != "" {
			switch strings.ToLower(val) {
			case "immediate":
				txlock = "BEGIN IMMEDIATE"
			case "exclusive":
				txlock = "BEGIN EXCLUSIVE"
			case "deferred":
				txlock = "BEGIN"
			default:
				return nil, fmt.Errorf("Invalid _txlock: %v", val)
			}
		}

		// Auto Vacuum (_vacuum)
		//
		// https://www.sqlite.org/pragma.html#pragma_auto_vacuum
		//
		pkey = "" // Reset pkey
		if _, ok := params["_auto_vacuum"]; ok {
			pkey = "_auto_vacuum"
		}
		if _, ok := params["_vacuum"]; ok {
			pkey = "_vacuum"
		}
		if val := params.Get(pkey); val != "" {
			switch strings.ToLower(val) {
			case "0", "none":
				autoVacuum = 0
			case "1", "full":
				autoVacuum = 1
			case "2", "incremental":
				autoVacuum = 2
			default:
				return nil, fmt.Errorf("Invalid _auto_vacuum: %v, expecting value of '0 NONE 1 FULL 2 INCREMENTAL'", val)
			}
		}

		// Busy Timeout (_busy_timeout)
		//
		// https://www.sqlite.org/pragma.html#pragma_busy_timeout
		//
		pkey = "" // Reset pkey
		if _, ok := params["_busy_timeout"]; ok {
			pkey = "_busy_timeout"
		}
		if _, ok := params["_timeout"]; ok {
			pkey = "_timeout"
		}
		if val := params.Get(pkey); val != "" {
			iv, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Invalid _busy_timeout: %v: %v", val, err)
			}
			busyTimeout = int(iv)
		}

		// Case Sensitive Like (_cslike)
		//
		// https://www.sqlite.org/pragma.html#pragma_case_sensitive_like
		//
		pkey = "" // Reset pkey
		if _, ok := params["_case_sensitive_like"]; ok {
			pkey = "_case_sensitive_like"
		}
		if _, ok := params["_cslike"]; ok {
			pkey = "_cslike"
		}
		if val := params.Get(pkey); val != "" {
			switch strings.ToLower(val) {
			case "0", "no", "false", "off":
				caseSensitiveLike = 0
			case "1", "yes", "true", "on":
				caseSensitiveLike = 1
			default:
				return nil, fmt.Errorf("Invalid _case_sensitive_like: %v, expecting boolean value of '0 1 false true no yes off on'", val)
			}
		}

		// Defer Foreign Keys (_defer_foreign_keys | _defer_fk)
		//
		// https://www.sqlite.org/pragma.html#pragma_defer_foreign_keys
		//
		pkey = "" // Reset pkey
		if _, ok := params["_defer_foreign_keys"]; ok {
			pkey = "_defer_foreign_keys"
		}
		if _, ok := params["_defer_fk"]; ok {
			pkey = "_defer_fk"
		}
		if val := params.Get(pkey); val != "" {
			switch strings.ToLower(val) {
			case "0", "no", "false", "off":
				deferForeignKeys = 0
			case "1", "yes", "true", "on":
				deferForeignKeys = 1
			default:
				return nil, fmt.Errorf("Invalid _defer_foreign_keys: %v, expecting boolean value of '0 1 false true no yes off on'", val)
			}
		}

		// Foreign Keys (_foreign_keys | _fk)
		//
		// https://www.sqlite.org/pragma.html#pragma_foreign_keys
		//
		pkey = "" // Reset pkey
		if _, ok := params["_foreign_keys"]; ok {
			pkey = "_foreign_keys"
		}
		if _, ok := params["_fk"]; ok {
			pkey = "_fk"
		}
		if val := params.Get(pkey); val != "" {
			switch strings.ToLower(val) {
			case "0", "no", "false", "off":
				foreignKeys = 0
			case "1", "yes", "true", "on":
				foreignKeys = 1
			default:
				return nil, fmt.Errorf("Invalid _foreign_keys: %v, expecting boolean value of '0 1 false true no yes off on'", val)
			}
		}

		// Ignore CHECK Constrains (_ignore_check_constraints)
		//
		// https://www.sqlite.org/pragma.html#pragma_ignore_check_constraints
		//
		if val := params.Get("_ignore_check_constraints"); val != "" {
			switch strings.ToLower(val) {
			case "0", "no", "false", "off":
				ignoreCheckConstraints = 0
			case "1", "yes", "true", "on":
				ignoreCheckConstraints = 1
			default:
				return nil, fmt.Errorf("Invalid _ignore_check_constraints: %v, expecting boolean value of '0 1 false true no yes off on'", val)
			}
		}

		// Journal Mode (_journal_mode | _journal)
		//
		// https://www.sqlite.org/pragma.html#pragma_journal_mode
		//
		pkey = "" // Reset pkey
		if _, ok := params["_journal_mode"]; ok {
			pkey = "_journal_mode"
		}
		if _, ok := params["_journal"]; ok {
			pkey = "_journal"
		}
		if val := params.Get(pkey); val != "" {
			switch strings.ToUpper(val) {
			case "DELETE", "TRUNCATE", "PERSIST", "MEMORY", "OFF":
				journalMode = strings.ToUpper(val)
			case "WAL":
				journalMode = strings.ToUpper(val)

				// For WAL Mode set Synchronous Mode to 'NORMAL'
				// See https://www.sqlite.org/pragma.html#pragma_synchronous
				synchronousMode = "NORMAL"
			default:
				return nil, fmt.Errorf("Invalid _journal: %v, expecting value of 'DELETE TRUNCATE PERSIST MEMORY WAL OFF'", val)
			}
		}

		// Locking Mode (_locking)
		//
		// https://www.sqlite.org/pragma.html#pragma_locking_mode
		//
		pkey = "" // Reset pkey
		if _, ok := params["_locking_mode"]; ok {
			pkey = "_locking_mode"
		}
		if _, ok := params["_locking"]; ok {
			pkey = "_locking"
		}
		if val := params.Get(pkey); val != "" {
			switch strings.ToUpper(val) {
			case "NORMAL", "EXCLUSIVE":
				lockingMode = strings.ToUpper(val)
			default:
				return nil, fmt.Errorf("Invalid _locking_mode: %v, expecting value of 'NORMAL EXCLUSIVE", val)
			}
		}

		// Query Only (_query_only)
		//
		// https://www.sqlite.org/pragma.html#pragma_query_only
		//
		if val := params.Get("_query_only"); val != "" {
			switch strings.ToLower(val) {
			case "0", "no", "false", "off":
				queryOnly = 0
			case "1", "yes", "true", "on":
				queryOnly = 1
			default:
				return nil, fmt.Errorf("Invalid _query_only: %v, expecting boolean value of '0 1 false true no yes off on'", val)
			}
		}

		// Recursive Triggers (_recursive_triggers)
		//
		// https://www.sqlite.org/pragma.html#pragma_recursive_triggers
		//
		pkey = "" // Reset pkey
		if _, ok := params["_recursive_triggers"]; ok {
			pkey = "_recursive_triggers"
		}
		if _, ok := params["_rt"]; ok {
			pkey = "_rt"
		}
		if val := params.Get(pkey); val != "" {
			switch strings.ToLower(val) {
			case "0", "no", "false", "off":
				recursiveTriggers = 0
			case "1", "yes", "true", "on":
				recursiveTriggers = 1
			default:
				return nil, fmt.Errorf("Invalid _recursive_triggers: %v, expecting boolean value of '0 1 false true no yes off on'", val)
			}
		}

		// Secure Delete (_secure_delete)
		//
		// https://www.sqlite.org/pragma.html#pragma_secure_delete
		//
		if val := params.Get("_secure_delete"); val != "" {
			switch strings.ToLower(val) {
			case "0", "no", "false", "off":
				secureDelete = "OFF"
			case "1", "yes", "true", "on":
				secureDelete = "ON"
			case "fast":
				secureDelete = "FAST"
			default:
				return nil, fmt.Errorf("Invalid _secure_delete: %v, expecting boolean value of '0 1 false true no yes off on fast'", val)
			}
		}

		// Synchronous Mode (_synchronous | _sync)
		//
		// https://www.sqlite.org/pragma.html#pragma_synchronous
		//
		pkey = "" // Reset pkey
		if _, ok := params["_synchronous"]; ok {
			pkey = "_synchronous"
		}
		if _, ok := params["_sync"]; ok {
			pkey = "_sync"
		}
		if val := params.Get(pkey); val != "" {
			switch strings.ToUpper(val) {
			case "0", "OFF", "1", "NORMAL", "2", "FULL", "3", "EXTRA":
				synchronousMode = strings.ToUpper(val)
			default:
				return nil, fmt.Errorf("Invalid _synchronous: %v, expecting value of '0 OFF 1 NORMAL 2 FULL 3 EXTRA'", val)
			}
		}

		// Writable Schema (_writeable_schema)
		//
		// https://www.sqlite.org/pragma.html#pragma_writeable_schema
		//
		if val := params.Get("_writable_schema"); val != "" {
			switch strings.ToLower(val) {
			case "0", "no", "false", "off":
				writableSchema = 0
			case "1", "yes", "true", "on":
				writableSchema = 1
			default:
				return nil, fmt.Errorf("Invalid _writable_schema: %v, expecting boolean value of '0 1 false true no yes off on'", val)
			}
		}

		// Cache size (_cache_size)
		//
		// https://sqlite.org/pragma.html#pragma_cache_size
		//
		if val := params.Get("_cache_size"); val != "" {
			iv, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("Invalid _cache_size: %v: %v", val, err)
			}
			cacheSize = &iv
		}

		// _stmt_cache_size sets the maximum number of prepared statements
		// cached per connection. Note that sql.DB is a connection pool, so
		// each connection maintains its own independent cache.
		if val := params.Get("_stmt_cache_size"); val != "" {
			iv, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("Invalid _stmt_cache_size: %v: %v", val, err)
			}
			if iv < 0 {
				return nil, fmt.Errorf("Invalid _stmt_cache_size: %v, expecting non-negative integer", val)
			}
			stmtCacheSize = iv
		}

		if val := params.Get("vfs"); val != "" {
			vfsName = val
		}

		if !strings.HasPrefix(dsn, "file:") {
			dsn = dsn[:pos]
		}
	}

	var db *C.sqlite3
	name := C.CString(dsn)
	defer C.free(unsafe.Pointer(name))
	var vfs *C.char
	if vfsName != "" {
		vfs = C.CString(vfsName)
		defer C.free(unsafe.Pointer(vfs))
	}
	rv := C._sqlite3_open_v2(name, &db,
		mutex|C.SQLITE_OPEN_READWRITE|C.SQLITE_OPEN_CREATE,
		vfs)
	if rv != 0 {
		// Save off the error _before_ closing the database.
		// This is safe even if db is nil.
		err := lastError(db)
		if db != nil {
			C.sqlite3_close_v2(db)
		}
		return nil, err
	}
	if db == nil {
		return nil, errors.New("sqlite succeeded without returning a database")
	}

	exec := func(s string) error {
		cs := C.CString(s)
		rv := C.sqlite3_exec(db, cs, nil, nil, nil)
		C.free(unsafe.Pointer(cs))
		if rv != C.SQLITE_OK {
			return lastError(db)
		}
		return nil
	}

	// Busy timeout
	if err := exec(fmt.Sprintf("PRAGMA busy_timeout = %d;", busyTimeout)); err != nil {
		C.sqlite3_close_v2(db)
		return nil, err
	}

	// USER AUTHENTICATION
	//
	// User Authentication is always performed even when
	// sqlite_userauth is not compiled in, because without user authentication
	// the authentication is a no-op.
	//
	// Workflow
	//	- Authenticate
	//		ON::SUCCESS		=> Continue
	//		ON::SQLITE_AUTH => Return error and exit Open(...)
	//
	//  - Activate User Authentication
	//		Check if the user wants to activate User Authentication.
	//		If so then first create a temporary AuthConn to the database
	//		This is possible because we are already successfully authenticated.
	//
	//	- Check if `sqlite_user`` table exists
	//		YES				=> Add the provided user from DSN as Admin User and
	//						   activate user authentication.
	//		NO				=> Continue
	//

	// Create connection to SQLite
	conn := &SQLiteConn{db: db, loc: loc, txlock: txlock}
	if stmtCacheSize > 0 {
		conn.stmtCache = make([]*SQLiteStmt, 0, stmtCacheSize)
		conn.stmtCacheEnabled = true
	}

	// Password Cipher has to be registered before authentication
	if len(authCrypt) > 0 {
		switch strings.ToUpper(authCrypt) {
		case "SHA1":
			if err := conn.RegisterFunc("sqlite_crypt", CryptEncoderSHA1, true); err != nil {
				return nil, fmt.Errorf("CryptEncoderSHA1: %s", err)
			}
		case "SSHA1":
			if len(authSalt) == 0 {
				return nil, fmt.Errorf("_auth_crypt=ssha1, requires _auth_salt")
			}
			if err := conn.RegisterFunc("sqlite_crypt", CryptEncoderSSHA1(authSalt), true); err != nil {
				return nil, fmt.Errorf("CryptEncoderSSHA1: %s", err)
			}
		case "SHA256":
			if err := conn.RegisterFunc("sqlite_crypt", CryptEncoderSHA256, true); err != nil {
				return nil, fmt.Errorf("CryptEncoderSHA256: %s", err)
			}
		case "SSHA256":
			if len(authSalt) == 0 {
				return nil, fmt.Errorf("_auth_crypt=ssha256, requires _auth_salt")
			}
			if err := conn.RegisterFunc("sqlite_crypt", CryptEncoderSSHA256(authSalt), true); err != nil {
				return nil, fmt.Errorf("CryptEncoderSSHA256: %s", err)
			}
		case "SHA384":
			if err := conn.RegisterFunc("sqlite_crypt", CryptEncoderSHA384, true); err != nil {
				return nil, fmt.Errorf("CryptEncoderSHA384: %s", err)
			}
		case "SSHA384":
			if len(authSalt) == 0 {
				return nil, fmt.Errorf("_auth_crypt=ssha384, requires _auth_salt")
			}
			if err := conn.RegisterFunc("sqlite_crypt", CryptEncoderSSHA384(authSalt), true); err != nil {
				return nil, fmt.Errorf("CryptEncoderSSHA384: %s", err)
			}
		case "SHA512":
			if err := conn.RegisterFunc("sqlite_crypt", CryptEncoderSHA512, true); err != nil {
				return nil, fmt.Errorf("CryptEncoderSHA512: %s", err)
			}
		case "SSHA512":
			if len(authSalt) == 0 {
				return nil, fmt.Errorf("_auth_crypt=ssha512, requires _auth_salt")
			}
			if err := conn.RegisterFunc("sqlite_crypt", CryptEncoderSSHA512(authSalt), true); err != nil {
				return nil, fmt.Errorf("CryptEncoderSSHA512: %s", err)
			}
		}
	}

	// Preform Authentication
	if err := conn.Authenticate(authUser, authPass); err != nil {
		return nil, err
	}

	// Register: authenticate
	// Authenticate will perform an authentication of the provided username
	// and password against the database.
	//
	// If a database contains the SQLITE_USER table, then the
	// call to Authenticate must be invoked with an
	// appropriate username and password prior to enable read and write
	//access to the database.
	//
	// Return SQLITE_OK on success or SQLITE_ERROR if the username/password
	// combination is incorrect or unknown.
	//
	// If the SQLITE_USER table is not present in the database file, then
	// this interface is a harmless no-op returnning SQLITE_OK.
	if err := conn.RegisterFunc("authenticate", conn.authenticate, true); err != nil {
		return nil, err
	}
	//
	// Register: auth_user_add
	// auth_user_add can be used (by an admin user only)
	// to create a new user. When called on a no-authentication-required
	// database, this routine converts the database into an authentication-
	// required database, automatically makes the added user an
	// administrator, and logs in the current connection as that user.
	// The AuthUserAdd only works for the "main" database, not
	// for any ATTACH-ed databases. Any call to AuthUserAdd by a
	// non-admin user results in an error.
	if err := conn.RegisterFunc("auth_user_add", conn.authUserAdd, true); err != nil {
		return nil, err
	}
	//
	// Register: auth_user_change
	// auth_user_change can be used to change a users
	// login credentials or admin privilege.  Any user can change their own
	// login credentials. Only an admin user can change another users login
	// credentials or admin privilege setting. No user may change their own
	// admin privilege setting.
	if err := conn.RegisterFunc("auth_user_change", conn.authUserChange, true); err != nil {
		return nil, err
	}
	//
	// Register: auth_user_delete
	// auth_user_delete can be used (by an admin user only)
	// to delete a user. The currently logged-in user cannot be deleted,
	// which guarantees that there is always an admin user and hence that
	// the database cannot be converted into a no-authentication-required
	// database.
	if err := conn.RegisterFunc("auth_user_delete", conn.authUserDelete, true); err != nil {
		return nil, err
	}

	// Register: auth_enabled
	// auth_enabled can be used to check if user authentication is enabled
	if err := conn.RegisterFunc("auth_enabled", conn.authEnabled, true); err != nil {
		return nil, err
	}

	// Auto Vacuum
	// Moved auto_vacuum command, the user preference for auto_vacuum needs to be implemented directly after
	// the authentication and before the sqlite_user table gets created if the user
	// decides to activate User Authentication because
	// auto_vacuum needs to be set before any tables are created
	// and activating user authentication creates the internal table `sqlite_user`.
	if autoVacuum > -1 {
		if err := exec(fmt.Sprintf("PRAGMA auto_vacuum = %d;", autoVacuum)); err != nil {
			C.sqlite3_close_v2(db)
			return nil, err
		}
	}

	// Check if user wants to activate User Authentication
	if authCreate {
		// Before going any further, we need to check that the user
		// has provided an username and password within the DSN.
		// We are not allowed to continue.
		if len(authUser) == 0 {
			return nil, fmt.Errorf("Missing '_auth_user' while user authentication was requested with '_auth'")
		}
		if len(authPass) == 0 {
			return nil, fmt.Errorf("Missing '_auth_pass' while user authentication was requested with '_auth'")
		}

		// Check if User Authentication is Enabled
		authExists := conn.AuthEnabled()
		if !authExists {
			if err := conn.AuthUserAdd(authUser, authPass, true); err != nil {
				return nil, err
			}
		}
	}

	// Case Sensitive LIKE
	if caseSensitiveLike > -1 {
		if err := exec(fmt.Sprintf("PRAGMA case_sensitive_like = %d;", caseSensitiveLike)); err != nil {
			C.sqlite3_close_v2(db)
			return nil, err
		}
	}

	// Defer Foreign Keys
	if deferForeignKeys > -1 {
		if err := exec(fmt.Sprintf("PRAGMA defer_foreign_keys = %d;", deferForeignKeys)); err != nil {
			C.sqlite3_close_v2(db)
			return nil, err
		}
	}

	// Foreign Keys
	if foreignKeys > -1 {
		if err := exec(fmt.Sprintf("PRAGMA foreign_keys = %d;", foreignKeys)); err != nil {
			C.sqlite3_close_v2(db)
			return nil, err
		}
	}

	// Ignore CHECK Constraints
	if ignoreCheckConstraints > -1 {
		if err := exec(fmt.Sprintf("PRAGMA ignore_check_constraints = %d;", ignoreCheckConstraints)); err != nil {
			C.sqlite3_close_v2(db)
			return nil, err
		}
	}

	// Journal Mode
	if journalMode != "" {
		if err := exec(fmt.Sprintf("PRAGMA journal_mode = %s;", journalMode)); err != nil {
			C.sqlite3_close_v2(db)
			return nil, err
		}
	}

	// Locking Mode
	// Because the default is NORMAL and this is not changed in this package
	// by using the compile time SQLITE_DEFAULT_LOCKING_MODE this PRAGMA can always be executed
	if err := exec(fmt.Sprintf("PRAGMA locking_mode = %s;", lockingMode)); err != nil {
		C.sqlite3_close_v2(db)
		return nil, err
	}

	// Query Only
	if queryOnly > -1 {
		if err := exec(fmt.Sprintf("PRAGMA query_only = %d;", queryOnly)); err != nil {
			C.sqlite3_close_v2(db)
			return nil, err
		}
	}

	// Recursive Triggers
	if recursiveTriggers > -1 {
		if err := exec(fmt.Sprintf("PRAGMA recursive_triggers = %d;", recursiveTriggers)); err != nil {
			C.sqlite3_close_v2(db)
			return nil, err
		}
	}

	// Secure Delete
	//
	// Because this package can set the compile time flag SQLITE_SECURE_DELETE with a build tag
	// the default value for secureDelete var is 'DEFAULT' this way
	// you can compile with secure_delete 'ON' and disable it for a specific database connection.
	if secureDelete != "DEFAULT" {
		if err := exec(fmt.Sprintf("PRAGMA secure_delete = %s;", secureDelete)); err != nil {
			C.sqlite3_close_v2(db)
			return nil, err
		}
	}

	// Synchronous Mode
	//
	// Because default is NORMAL this statement is always executed
	if err := exec(fmt.Sprintf("PRAGMA synchronous = %s;", synchronousMode)); err != nil {
		conn.Close()
		return nil, err
	}

	// Writable Schema
	if writableSchema > -1 {
		if err := exec(fmt.Sprintf("PRAGMA writable_schema = %d;", writableSchema)); err != nil {
			C.sqlite3_close_v2(db)
			return nil, err
		}
	}

	// Cache Size
	if cacheSize != nil {
		if err := exec(fmt.Sprintf("PRAGMA cache_size = %d;", *cacheSize)); err != nil {
			C.sqlite3_close_v2(db)
			return nil, err
		}
	}

	if len(d.Extensions) > 0 {
		if err := conn.loadExtensions(d.Extensions); err != nil {
			conn.Close()
			return nil, err
		}
	}

	if d.ConnectHook != nil {
		if err := d.ConnectHook(conn); err != nil {
			conn.Close()
			return nil, err
		}
	}
	runtime.SetFinalizer(conn, (*SQLiteConn).Close)
	return conn, nil
}

// Close the connection.
func (c *SQLiteConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil
	}
	runtime.SetFinalizer(c, nil)
	c.closeCachedStmtsLocked()
	rv := C.sqlite3_close_v2(c.db)
	if rv != C.SQLITE_OK {
		return lastError(c.db)
	}
	deleteHandles(c)
	c.db = nil
	return nil
}

func (c *SQLiteConn) dbConnOpen() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.db != nil
}

func (c *SQLiteConn) takeCachedStmt(query string) *SQLiteStmt {
	if c == nil || query == "" || !c.stmtCacheEnabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return nil
	}
	// Scan from the MRU end (tail) so that a stmt put just before is
	// found immediately.
	for i := len(c.stmtCache) - 1; i >= 0; i-- {
		s := c.stmtCache[i]
		if s.cacheKey != query {
			continue
		}
		n := len(c.stmtCache)
		copy(c.stmtCache[i:n-1], c.stmtCache[i+1:n])
		c.stmtCache[n-1] = nil
		c.stmtCache = c.stmtCache[:n-1]
		// The stmt was marked closed by Close before being cached, and
		// cls may have been set if Query opened it; reset both so the
		// caller gets a stmt equivalent to a fresh Prepare.
		s.closed = false
		s.cls = false
		return s
	}
	return nil
}

func (c *SQLiteConn) putCachedStmt(s *SQLiteStmt) bool {
	if c == nil || s == nil || s.s == nil || s.cacheKey == "" {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return false
	}
	rv := C._sqlite3_reset_clear(s.s)
	if rv != C.SQLITE_ROW && rv != C.SQLITE_OK && rv != C.SQLITE_DONE {
		return false
	}
	// If full, finalize the LRU entry at index 0 and shift left; the
	// freed tail slot is immediately reused by the append below.
	if len(c.stmtCache) == cap(c.stmtCache) {
		finalizeCachedStmt(c.stmtCache[0])
		copy(c.stmtCache, c.stmtCache[1:])
		c.stmtCache = c.stmtCache[:len(c.stmtCache)-1]
	}
	c.stmtCache = append(c.stmtCache, s)
	return true
}

func (c *SQLiteConn) closeCachedStmtsLocked() {
	for i, s := range c.stmtCache {
		c.stmtCache[i] = nil
		finalizeCachedStmt(s)
	}
	c.stmtCache = c.stmtCache[:0]
}

// finalizeCachedStmt tears down a stmt that was sitting in the connection's
// stmt cache. The caller must hold c.mu. It is safe to pass a nil stmt or a
// stmt whose handle has already been released.
func finalizeCachedStmt(s *SQLiteStmt) {
	if s == nil {
		return
	}
	runtime.SetFinalizer(s, nil)
	if s.s != nil {
		C.sqlite3_finalize(s.s)
		s.s = nil
	}
	s.c = nil
	s.closed = true
}

// Prepare the query string. Return a new statement.
func (c *SQLiteConn) Prepare(query string) (driver.Stmt, error) {
	return c.prepare(context.Background(), query)
}

func (c *SQLiteConn) prepare(ctx context.Context, query string) (driver.Stmt, error) {
	pquery := C.CString(query)
	defer C.free(unsafe.Pointer(pquery))
	var s *C.sqlite3_stmt
	var tail *C.char
	rv := C._sqlite3_prepare_v2_internal(c.db, pquery, C.int(len(query)), &s, &tail)
	if rv != C.SQLITE_OK {
		return nil, c.lastError()
	}
	var t string
	if tail != nil && *tail != '\000' {
		t = strings.TrimSpace(C.GoString(tail))
	}
	ss := &SQLiteStmt{c: c, s: s, t: t}
	runtime.SetFinalizer(ss, (*SQLiteStmt).Close)
	return ss, nil
}

func (c *SQLiteConn) prepareWithCache(ctx context.Context, query string) (driver.Stmt, error) {
	if stmt := c.takeCachedStmt(query); stmt != nil {
		return stmt, nil
	}
	stmt, err := c.prepare(ctx, query)
	if err != nil {
		return nil, err
	}
	ss := stmt.(*SQLiteStmt)
	if ss.t == "" && c.stmtCacheEnabled {
		ss.cacheKey = query
	}
	return ss, nil
}

// Run-Time Limit Categories.
// See: http://www.sqlite.org/c3ref/c_limit_attached.html
const (
	SQLITE_LIMIT_LENGTH              = C.SQLITE_LIMIT_LENGTH
	SQLITE_LIMIT_SQL_LENGTH          = C.SQLITE_LIMIT_SQL_LENGTH
	SQLITE_LIMIT_COLUMN              = C.SQLITE_LIMIT_COLUMN
	SQLITE_LIMIT_EXPR_DEPTH          = C.SQLITE_LIMIT_EXPR_DEPTH
	SQLITE_LIMIT_COMPOUND_SELECT     = C.SQLITE_LIMIT_COMPOUND_SELECT
	SQLITE_LIMIT_VDBE_OP             = C.SQLITE_LIMIT_VDBE_OP
	SQLITE_LIMIT_FUNCTION_ARG        = C.SQLITE_LIMIT_FUNCTION_ARG
	SQLITE_LIMIT_ATTACHED            = C.SQLITE_LIMIT_ATTACHED
	SQLITE_LIMIT_LIKE_PATTERN_LENGTH = C.SQLITE_LIMIT_LIKE_PATTERN_LENGTH
	SQLITE_LIMIT_VARIABLE_NUMBER     = C.SQLITE_LIMIT_VARIABLE_NUMBER
	SQLITE_LIMIT_TRIGGER_DEPTH       = C.SQLITE_LIMIT_TRIGGER_DEPTH
	SQLITE_LIMIT_WORKER_THREADS      = C.SQLITE_LIMIT_WORKER_THREADS
)

// GetFilename returns the absolute path to the file containing
// the requested schema. When passed an empty string, it will
// instead use the database's default schema: "main".
// See: sqlite3_db_filename, https://www.sqlite.org/c3ref/db_filename.html
func (c *SQLiteConn) GetFilename(schemaName string) string {
	if schemaName == "" {
		schemaName = "main"
	}
	cSchema := C.CString(schemaName)
	defer C.free(unsafe.Pointer(cSchema))
	return C.GoString(C.sqlite3_db_filename(c.db, cSchema))
}

// GetLimit returns the current value of a run-time limit.
// See: sqlite3_limit, http://www.sqlite.org/c3ref/limit.html
func (c *SQLiteConn) GetLimit(id int) int {
	return int(C._sqlite3_limit(c.db, C.int(id), C.int(-1)))
}

// SetLimit changes the value of a run-time limits.
// Then this method returns the prior value of the limit.
// See: sqlite3_limit, http://www.sqlite.org/c3ref/limit.html
func (c *SQLiteConn) SetLimit(id int, newVal int) int {
	return int(C._sqlite3_limit(c.db, C.int(id), C.int(newVal)))
}

// SetFileControlInt invokes the xFileControl method on a given database. The
// dbName is the name of the database. It will default to "main" if left blank.
// The op is one of the opcodes prefixed by "SQLITE_FCNTL_". The arg argument
// and return code are both opcode-specific. Please see the SQLite documentation.
//
// This method is not thread-safe as the returned error code can be changed by
// another call if invoked concurrently.
//
// Use SetFileControlInt64 instead if the argument for the opcode is documented
// as a pointer to a sqlite3_int64.
//
// See: sqlite3_file_control, https://www.sqlite.org/c3ref/file_control.html
func (c *SQLiteConn) SetFileControlInt(dbName string, op int, arg int) error {
	if dbName == "" {
		dbName = "main"
	}

	cDBName := C.CString(dbName)
	defer C.free(unsafe.Pointer(cDBName))

	cArg := C.int(arg)
	rv := C.sqlite3_file_control(c.db, cDBName, C.int(op), unsafe.Pointer(&cArg))
	if rv != C.SQLITE_OK {
		return c.lastError()
	}
	return nil
}

// SetFileControlInt64 invokes the xFileControl method on a given database. The
// dbName is the name of the database. It will default to "main" if left blank.
// The op is one of the opcodes prefixed by "SQLITE_FCNTL_". The arg argument
// and return code are both opcode-specific. Please see the SQLite documentation.
//
// This method is not thread-safe as the returned error code can be changed by
// another call if invoked concurrently.
//
// Only use this method if the argument for the opcode is documented as a pointer
// to a sqlite3_int64.
//
// See: sqlite3_file_control, https://www.sqlite.org/c3ref/file_control.html
func (c *SQLiteConn) SetFileControlInt64(dbName string, op int, arg int64) error {
	if dbName == "" {
		dbName = "main"
	}

	cDBName := C.CString(dbName)
	defer C.free(unsafe.Pointer(cDBName))

	cArg := C.sqlite3_int64(arg)
	rv := C.sqlite3_file_control(c.db, cDBName, C.int(op), unsafe.Pointer(&cArg))
	if rv != C.SQLITE_OK {
		return c.lastError()
	}
	return nil
}

// Close the statement.
func (s *SQLiteStmt) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	runtime.SetFinalizer(s, nil)
	conn := s.c
	stmt := s.s
	if stmt == nil {
		s.c = nil
		return nil
	}
	if !conn.dbConnOpen() {
		return errors.New("sqlite statement with already closed database connection")
	}
	if s.cacheKey != "" && conn.putCachedStmt(s) {
		return nil
	}
	s.s = nil
	s.c = nil
	rv := C.sqlite3_finalize(stmt)
	if rv != C.SQLITE_OK {
		return conn.lastError()
	}
	return nil
}

// NumInput return a number of parameters.
func (s *SQLiteStmt) NumInput() int {
	return int(C.sqlite3_bind_parameter_count(s.s))
}

var placeHolder = []byte{0}

func bindText(s *C.sqlite3_stmt, n C.int, v string) C.int {
	if len(v) == 0 {
		return C._sqlite3_bind_text(s, n, (*C.char)(unsafe.Pointer(&placeHolder[0])), C.sqlite3_uint64(0))
	}
	return C._sqlite3_bind_text(s, n, (*C.char)(unsafe.Pointer(unsafe.StringData(v))), C.sqlite3_uint64(len(v)))
}

func bindValue(s *C.sqlite3_stmt, n C.int, value driver.Value) C.int {
	switch v := value.(type) {
	case nil:
		return C.sqlite3_bind_null(s, n)
	case string:
		return bindText(s, n, v)
	case int64:
		return C.sqlite3_bind_int64(s, n, C.sqlite3_int64(v))
	case bool:
		if v {
			return C.sqlite3_bind_int(s, n, 1)
		}
		return C.sqlite3_bind_int(s, n, 0)
	case float64:
		return C.sqlite3_bind_double(s, n, C.double(v))
	case []byte:
		if v == nil {
			return C.sqlite3_bind_null(s, n)
		}
		ln := len(v)
		if ln == 0 {
			v = placeHolder
		}
		return C._sqlite3_bind_blob(s, n, unsafe.Pointer(&v[0]), C.sqlite3_uint64(ln))
	case time.Time:
		var buf [64]byte
		b := v.AppendFormat(buf[:0], SQLiteTimestampFormats[0])
		if len(b) == 0 {
			return C._sqlite3_bind_text(s, n, (*C.char)(unsafe.Pointer(&placeHolder[0])), C.sqlite3_uint64(0))
		}
		return C._sqlite3_bind_text(s, n, (*C.char)(unsafe.Pointer(&b[0])), C.sqlite3_uint64(len(b)))
	default:
		return C.SQLITE_MISUSE
	}
}

func (s *SQLiteStmt) bindNamedIndices(name string) [3]int {
	if s.namedParams == nil {
		s.namedParams = make(map[string][3]int)
	} else if indices, ok := s.namedParams[name]; ok {
		return indices
	}

	// Build ":name\0" once and rewrite prefix byte to avoid 3 C.CString allocs.
	buf := make([]byte, 1+len(name)+1) // prefix + name + null terminator
	copy(buf[1:], name)
	buf[len(buf)-1] = 0
	cname := (*C.char)(unsafe.Pointer(&buf[0]))

	var indices [3]int
	prefixes := [3]byte{':', '@', '$'}
	for i, p := range prefixes {
		buf[0] = p
		indices[i] = int(C.sqlite3_bind_parameter_index(s.s, cname))
	}
	s.namedParams[name] = indices
	return indices
}

func stmtArgs(args []driver.NamedValue, start, na int) []driver.NamedValue {
	if na == 0 {
		return nil
	}

	end := start + na
	hasNamedOutside := false
	for i := range args {
		if args[i].Name != "" && (i < start || i >= end) {
			hasNamedOutside = true
			break
		}
	}
	if start == 0 && !hasNamedOutside {
		return args[start:end]
	}

	stmtArgs := make([]driver.NamedValue, 0, len(args))
	stmtArgs = append(stmtArgs, args[start:end]...)
	for i := range args {
		if args[i].Name != "" && (i < start || i >= end) {
			stmtArgs = append(stmtArgs, args[i])
		}
	}
	for i := range stmtArgs {
		stmtArgs[i].Ordinal = i + 1
	}
	return stmtArgs
}

func (s *SQLiteStmt) bind(args []driver.NamedValue) error {
	rv := C._sqlite3_reset_clear(s.s)
	if rv != C.SQLITE_ROW && rv != C.SQLITE_OK && rv != C.SQLITE_DONE {
		return s.c.lastError()
	}

	hasNamed := false
	for i := range args {
		if args[i].Name != "" {
			hasNamed = true
			break
		}
	}

	if !hasNamed {
		for _, arg := range args {
			n := C.int(arg.Ordinal)
			rv = bindValue(s.s, n, arg.Value)
			if rv != C.SQLITE_OK {
				return s.c.lastError()
			}
		}
		return nil
	}

	for _, arg := range args {
		if arg.Name == "" {
			rv = bindValue(s.s, C.int(arg.Ordinal), arg.Value)
			if rv != C.SQLITE_OK {
				return s.c.lastError()
			}
			continue
		}
		indices := s.bindNamedIndices(arg.Name)
		for _, idx := range indices {
			if idx == 0 {
				continue
			}
			rv = bindValue(s.s, C.int(idx), arg.Value)
			if rv != C.SQLITE_OK {
				return s.c.lastError()
			}
		}
	}
	return nil
}

// Query the statement with arguments. Return records.
func (s *SQLiteStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.query(context.Background(), valueToNamedValue(args))
}

func (s *SQLiteStmt) query(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if err := s.bind(args); err != nil {
		return nil, err
	}

	rows := &SQLiteRows{
		s:        s,
		nc:       int32(C.sqlite3_column_count(s.s)),
		cls:      s.cls,
		cols:     nil,
		decltype: nil,
		colvals:  nil,
		ctx:      ctx,
	}
	if rows.nc > 0 {
		rows.colvals = (*C.sqlite3_go_col)(C.malloc(C.size_t(rows.nc) * C.size_t(unsafe.Sizeof(C.sqlite3_go_col{}))))
		if rows.colvals == nil {
			return nil, errors.New("sqlite3: failed to allocate row buffer")
		}
	}

	return rows, nil
}

// LastInsertId return last inserted ID.
func (r *SQLiteResult) LastInsertId() (int64, error) {
	return r.id, nil
}

// RowsAffected return how many rows affected.
func (r *SQLiteResult) RowsAffected() (int64, error) {
	return r.changes, nil
}

// Exec execute the statement with arguments. Return result object.
func (s *SQLiteStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.exec(context.Background(), valueToNamedValue(args))
}

func valueToNamedValue(args []driver.Value) []driver.NamedValue {
	list := make([]driver.NamedValue, len(args))
	for i, v := range args {
		list[i] = driver.NamedValue{
			Ordinal: i + 1,
			Value:   v,
		}
	}
	return list
}

func isInterruptErr(err error) bool {
	sqliteErr, ok := err.(Error)
	if ok {
		return sqliteErr.Code == ErrInterrupt
	}
	return false
}

// exec executes a query that doesn't return rows. Attempts to honor context timeout.
func (s *SQLiteStmt) exec(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if ctx.Done() == nil {
		return s.execSync(args)
	}

	sema := make(chan struct{})
	var r driver.Result
	var err error
	go func() {
		r, err = s.execSync(args)
		close(sema)
	}()
	select {
	case <-sema:
		return r, err
	case <-ctx.Done():
		select {
		case <-sema: // no need to interrupt, operation completed in db
			return r, err
		default:
			// this is still racy and can be no-op if executed between sqlite3_* calls in execSync.
			C.sqlite3_interrupt(s.c.db)
			<-sema // wait for goroutine completed
			if isInterruptErr(err) {
				return nil, ctx.Err()
			}
			return r, err
		}
	}
}

func (s *SQLiteStmt) execSync(args []driver.NamedValue) (driver.Result, error) {
	if err := s.bind(args); err != nil {
		C._sqlite3_reset_clear(s.s)
		return nil, err
	}

	var rowid, changes C.longlong
	rv := C._sqlite3_step_row_internal(s.s, &rowid, &changes)
	if rv != C.SQLITE_ROW && rv != C.SQLITE_OK && rv != C.SQLITE_DONE {
		err := s.c.lastError()
		C._sqlite3_reset_clear(s.s)
		return nil, err
	}

	return &SQLiteResult{id: int64(rowid), changes: int64(changes)}, nil
}

// Readonly reports if this statement is considered readonly by SQLite.
//
// See: https://sqlite.org/c3ref/stmt_readonly.html
func (s *SQLiteStmt) Readonly() bool {
	return C.sqlite3_stmt_readonly(s.s) == 1
}

// Close the rows.
func (rc *SQLiteRows) Close() error {
	rc.closemu.Lock()
	defer rc.closemu.Unlock()
	s := rc.s
	if s == nil {
		if rc.colvals != nil {
			C.free(unsafe.Pointer(rc.colvals))
			rc.colvals = nil
		}
		return nil
	}
	rc.s = nil // remove reference to SQLiteStmt
	if rc.colvals != nil {
		C.free(unsafe.Pointer(rc.colvals))
		rc.colvals = nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	if rc.cls {
		s.mu.Unlock()
		return s.Close()
	}
	rv := C.sqlite3_reset(s.s)
	if rv != C.SQLITE_OK {
		s.mu.Unlock()
		return s.c.lastError()
	}
	s.mu.Unlock()
	return nil
}

func (s *SQLiteStmt) cacheMetadata() bool {
	return !s.cls || s.cacheKey != ""
}

func (s *SQLiteStmt) columnNamesLocked(n int) []string {
	if s.metadata == nil {
		s.metadata = &sqliteStmtMetadata{}
	}
	if len(s.metadata.cols) != n {
		s.metadata.cols = make([]string, n)
		for i := range s.metadata.cols {
			s.metadata.cols[i] = C.GoString(C.sqlite3_column_name(s.s, C.int(i)))
		}
	}
	return s.metadata.cols
}

func (s *SQLiteStmt) declTypesLocked(n int) []string {
	if s.metadata == nil {
		s.metadata = &sqliteStmtMetadata{}
	}
	if len(s.metadata.decltype) != n {
		s.metadata.decltype = make([]string, n)
		for i := range s.metadata.decltype {
			s.metadata.decltype[i] = strings.ToLower(C.GoString(C.sqlite3_column_decltype(s.s, C.int(i))))
		}
	}
	return s.metadata.decltype
}

// Columns return column names.
func (rc *SQLiteRows) Columns() []string {
	if rc.s == nil {
		return rc.cols
	}
	rc.s.mu.Lock()
	defer rc.s.mu.Unlock()
	if rc.s.s != nil && int(rc.nc) != len(rc.cols) {
		if rc.s.cacheMetadata() {
			rc.cols = rc.s.columnNamesLocked(int(rc.nc))
		} else {
			rc.cols = make([]string, rc.nc)
			for i := range rc.cols {
				rc.cols[i] = C.GoString(C.sqlite3_column_name(rc.s.s, C.int(i)))
			}
		}
	}
	return rc.cols
}

func (rc *SQLiteRows) declTypes() []string {
	if rc.s.s != nil && rc.decltype == nil {
		if rc.s.cacheMetadata() {
			rc.decltype = rc.s.declTypesLocked(int(rc.nc))
		} else {
			rc.decltype = make([]string, rc.nc)
			for i := range rc.decltype {
				rc.decltype[i] = strings.ToLower(C.GoString(C.sqlite3_column_decltype(rc.s.s, C.int(i))))
			}
		}
	}
	return rc.decltype
}

// DeclTypes return column types.
func (rc *SQLiteRows) DeclTypes() []string {
	if rc.s == nil {
		return rc.decltype
	}
	rc.s.mu.Lock()
	defer rc.s.mu.Unlock()
	return rc.declTypes()
}

// Next move cursor to next. Attempts to honor context timeout from QueryContext call.
func (rc *SQLiteRows) Next(dest []driver.Value) error {
	if rc.s == nil {
		return io.EOF
	}
	rc.s.mu.Lock()
	defer rc.s.mu.Unlock()

	if rc.s.closed {
		return io.EOF
	}

	if rc.ctx.Done() == nil {
		return rc.nextSyncLocked(dest)
	}
	sema := make(chan struct{})
	var err error
	go func() {
		err = rc.nextSyncLocked(dest)
		close(sema)
	}()
	select {
	case <-sema:
		return err
	case <-rc.ctx.Done():
		select {
		case <-sema: // no need to interrupt
		default:
			// this is still racy and can be no-op if executed between sqlite3_* calls in nextSyncLocked.
			C.sqlite3_interrupt(rc.s.c.db)
			<-sema // ensure goroutine completed
		}
		return rc.ctx.Err()
	}
}

// nextSyncLocked moves cursor to next; must be called with locked mutex.
func (rc *SQLiteRows) nextSyncLocked(dest []driver.Value) error {
	rv := C._sqlite3_step_internal(rc.s.s)
	if rv == C.SQLITE_DONE {
		return io.EOF
	}
	if rv != C.SQLITE_ROW {
		rv = C.sqlite3_reset(rc.s.s)
		if rv != C.SQLITE_OK {
			return rc.s.c.lastError()
		}
		return nil
	}

	rc.declTypes()
	if len(dest) == 0 {
		return nil
	}
	C._sqlite3_column_values(rc.s.s, C.int(len(dest)), rc.colvals)
	colvals := (*[(math.MaxInt32 - 1) / unsafe.Sizeof(C.sqlite3_go_col{})]C.sqlite3_go_col)(unsafe.Pointer(rc.colvals))[:len(dest):len(dest)]

	decltype := rc.decltype
	_ = decltype[len(dest)-1]
	for i := range dest {
		col := &colvals[i]
		switch col.typ {
		case C.SQLITE_INTEGER:
			val := int64(col.i64)
			switch decltype[i] {
			case columnTimestamp, columnDatetime, columnDate:
				var t time.Time
				// Assume a millisecond unix timestamp if it's 13 digits -- too
				// large to be a reasonable timestamp in seconds.
				if val > 1e12 || val < -1e12 {
					val *= int64(time.Millisecond) // convert ms to nsec
					t = time.Unix(0, val)
				} else {
					t = time.Unix(val, 0)
				}
				t = t.UTC()
				if rc.s.c.loc != nil {
					t = t.In(rc.s.c.loc)
				}
				dest[i] = t
			case "boolean":
				dest[i] = val > 0
			default:
				dest[i] = val
			}
		case C.SQLITE_FLOAT:
			dest[i] = float64(col.f64)
		case C.SQLITE_BLOB:
			p := col.ptr
			if p == nil {
				dest[i] = []byte{}
				continue
			}
			n := col.n
			dest[i] = C.GoBytes(p, n)
		case C.SQLITE_NULL:
			dest[i] = nil
		case C.SQLITE_TEXT:
			var err error
			var timeVal time.Time

			n := int(col.n)
			s := C.GoStringN((*C.char)(unsafe.Pointer(col.ptr)), C.int(n))

			switch decltype[i] {
			case columnTimestamp, columnDatetime, columnDate:
				var t time.Time
				s = strings.TrimSuffix(s, "Z")
				for _, format := range SQLiteTimestampFormats {
					if timeVal, err = time.ParseInLocation(format, s, time.UTC); err == nil {
						t = timeVal
						break
					}
				}
				if err != nil {
					// The column is a time value, so return the zero time on parse failure.
					t = time.Time{}
				}
				if rc.s.c.loc != nil {
					t = t.In(rc.s.c.loc)
				}
				dest[i] = t
			default:
				dest[i] = s
			}
		}
	}
	return nil
}
