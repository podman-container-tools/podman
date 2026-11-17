//go:build !windows && !linux && !plan9

package linux

import "syscall"

func sysErrno(err error) Errno {
	se, ok := err.(syscall.Errno)
	if ok {
		// POSIX-defined errors seem to match up to error number 34
		// according to http://www.ioplex.com/~miallen/errcmpp.html.
		//
		// 9P2000.L expects Linux error codes, so after 34 we normalize.
		if se <= 34 {
			return Errno(se)
		}
		return 0
	}
	return 0
}
