package system

import "os"

// syscallConnControl calls fn with the file descriptor of fd,
// simplifying the boilerplate of f.SyscallConn().Control().
func syscallConnControl[T any](fd *os.File, fn func(uintptr) (T, error)) (T, error) {
	var zeroRes T
	conn, err := fd.SyscallConn()
	if err != nil {
		return zeroRes, err
	}
	var res T
	var resErr error
	if err := conn.Control(func(fd uintptr) {
		res, resErr = fn(fd)
	}); err != nil {
		return zeroRes, err
	}
	if resErr != nil {
		return zeroRes, resErr
	}
	return res, nil
}
