//go:build debug
// +build debug

package sftp

import "log"

func debug(fmt string, args ...any) {
	log.Printf(fmt, args...)
}
