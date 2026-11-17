//go:build !remote && (linux || freebsd) && cgo

package libpod

import (
	"errors"

	sqlite3 "github.com/mattn/go-sqlite3"
)

// isSQLiteConstraint reports whether err is a SQLite constraint violation
// (for example a UNIQUE or primary-key conflict). It inspects the typed driver
// error instead of the error message so it stays correct even if the message
// wording changes.
func isSQLiteConstraint(err error) bool {
	var sqliteErr sqlite3.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code == sqlite3.ErrConstraint
}
