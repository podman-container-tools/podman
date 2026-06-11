//go:build !remote && (linux || freebsd) && !cgo

package libpod

// isSQLiteConstraint reports whether err is a SQLite constraint violation.
//
// The github.com/mattn/go-sqlite3 driver and its typed errors are only
// available with cgo, and the driver itself requires cgo to function, so this
// build can never actually talk to SQLite at runtime. This stub exists solely
// so the package still compiles for CGO-free static analysis (for example the
// FreeBSD lint run performed by "make validatepr").
func isSQLiteConstraint(_ error) bool {
	return false
}
