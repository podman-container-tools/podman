package hvsock

// On Linux we have to deal with two different implementations. The
// "legacy" implementation never made it into the kernel, but several
// kernels, including the LinuxKit one carried patches for it for
// quite a while. The legacy version defined a new address family
// while the new version sits on top of the existing VMware/virtio
// socket implementation.
//
// We try to determine at init if we are on a kernel with the legacy
// implementation or the new version and set "legacyMode" accordingly.
//
// We can't just reuse the vsock implementation as we still need to
// emulated CloseRead()/CloseWrite() as not all Windows builds support
// it.

/*
#include <sys/socket.h>

struct sockaddr_hv {
	unsigned short shv_family;
	unsigned short reserved;
	unsigned char  shv_vm_id[16];
	unsigned char  shv_service_id[16];
};
int bind_sockaddr_hv(int fd, const struct sockaddr_hv *sa_hv) {
    return bind(fd, (const struct sockaddr*)sa_hv, sizeof(*sa_hv));
}
int connect_sockaddr_hv(int fd, const struct sockaddr_hv *sa_hv) {
    return connect(fd, (const struct sockaddr*)sa_hv, sizeof(*sa_hv));
}
int accept_hv(int fd, struct sockaddr_hv *sa_hv, socklen_t *sa_hv_len) {
    return accept(fd, (struct sockaddr *)sa_hv, sa_hv_len);
}
int getsockname_hv(int fd, struct sockaddr_hv *sa_hv, socklen_t *sa_hv_len) {
    return getsockname(fd, (struct sockaddr *)sa_hv, sa_hv_len);
}
*/
import "C"

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	hvsockAF  = 43 //SHV_PROTO_RAW
	hvsockRaw = 1  // SHV_PROTO_RAW
)

// Supported returns if hvsocks are supported on your platform
func Supported() bool {
	var sa C.struct_sockaddr_hv
	var sa_len C.socklen_t

	// Try opening  a hvsockAF socket. If it works we are on older, i.e. 4.9.x kernels.
	// 4.11 defines AF_SMC as 43 but it doesn't support protocol 1 so the
	// socket() call should fail.
	fd, err := syscall.Socket(hvsockAF, syscall.SOCK_STREAM, hvsockRaw)
	if err != nil {
		return false
	}

	// 4.16 defines SMCPROTO_SMC6 as 1 but its socket name size doesn't match
	// size of sockaddr_hv so corresponding check should fail.
	sa_len = C.sizeof_struct_sockaddr_hv
	ret, _ := C.getsockname_hv(C.int(fd), &sa, &sa_len)
	syscall.Close(fd)
	if ret < 0 || sa_len != C.sizeof_struct_sockaddr_hv {
		return false
	}

	return true
}

// Dial a Hyper-V socket address
func Dial(raddr Addr) (Conn, error) {
	fd, err := syscall.Socket(hvsockAF, syscall.SOCK_STREAM, hvsockRaw)
	if err != nil {
		return nil, err
	}

	sa := C.struct_sockaddr_hv{}
	sa.shv_family = hvsockAF
	sa.reserved = 0

	for i := 0; i < 16; i++ {
		sa.shv_vm_id[i] = C.uchar(raddr.VMID[i])
	}
	for i := 0; i < 16; i++ {
		sa.shv_service_id[i] = C.uchar(raddr.ServiceID[i])
	}

	// Retry connect in a loop if EINTR is encountered.
	for {
		if ret, errno := C.connect_sockaddr_hv(C.int(fd), &sa); ret != 0 {
			if errno == syscall.EINTR {
				continue
			}
			return nil, fmt.Errorf("connect(%s) failed with %d, errno=%d", raddr, ret, errno)
		}
		break
	}

	return newHVsockConn(uintptr(fd), &Addr{VMID: GUIDZero, ServiceID: GUIDZero}, &raddr), nil
}

// Listen returns a net.Listener which can accept connections on the given port
func Listen(addr Addr) (net.Listener, error) {
	fd, err := syscall.Socket(hvsockAF, syscall.SOCK_STREAM, hvsockRaw)
	if err != nil {
		return nil, err
	}

	sa := C.struct_sockaddr_hv{}
	sa.shv_family = hvsockAF
	sa.reserved = 0

	for i := 0; i < 16; i++ {
		sa.shv_vm_id[i] = C.uchar(addr.VMID[i])
	}
	for i := 0; i < 16; i++ {
		sa.shv_service_id[i] = C.uchar(addr.ServiceID[i])
	}

	if ret, errno := C.bind_sockaddr_hv(C.int(fd), &sa); ret != 0 {
		return nil, fmt.Errorf("listen(%s) failed with %d, errno=%d", addr, ret, errno)
	}

	err = syscall.Listen(fd, syscall.SOMAXCONN)
	if err != nil {
		return nil, fmt.Errorf("listen(%s) failed: %w", addr, err)
	}
	return &hvsockListener{fd, addr}, nil
}

//
// Hyper-v sockets Listener implementation
//

type hvsockListener struct {
	fd    int
	local Addr
}

// Accept accepts an incoming call and returns the new connection.
func (v *hvsockListener) Accept() (net.Conn, error) {
	var acceptSA C.struct_sockaddr_hv
	var acceptSALen C.socklen_t

	acceptSALen = C.sizeof_struct_sockaddr_hv
	fd, err := C.accept_hv(C.int(v.fd), &acceptSA, &acceptSALen)
	if err != nil {
		return nil, fmt.Errorf("accept(%s) failed: %w", v.local, err)
	}

	remote := &Addr{VMID: guidFromC(acceptSA.shv_vm_id), ServiceID: guidFromC(acceptSA.shv_service_id)}
	return newHVsockConn(uintptr(fd), &v.local, remote), nil
}

// Close closes the listening connection
func (v *hvsockListener) Close() error {
	// Note this won't cause the Accept to unblock.
	return unix.Close(v.fd)
}

// Addr returns the address the Listener is listening on
func (v *hvsockListener) Addr() net.Addr {
	return v.local
}

//
// Hyper-V socket connection implementation
//

// hvsockConn represents a connection over a Hyper-V socket
type hvsockConn struct {
	hvsock *os.File
	fd     uintptr
	local  *Addr
	remote *Addr
}

func newHVsockConn(fd uintptr, local, remote *Addr) *hvsockConn {
	hvsock := os.NewFile(fd, fmt.Sprintf("hvsock:%d", fd))
	return &hvsockConn{hvsock: hvsock, fd: fd, local: local, remote: remote}
}

// LocalAddr returns the local address of a connection
func (v *hvsockConn) LocalAddr() net.Addr {
	return v.local
}

// RemoteAddr returns the remote address of a connection
func (v *hvsockConn) RemoteAddr() net.Addr {
	return v.remote
}

// Close closes the connection
func (v *hvsockConn) Close() error {
	return v.hvsock.Close()
}

// CloseRead shuts down the reading side of a hvsock connection
func (v *hvsockConn) CloseRead() error {
	return syscall.Shutdown(int(v.fd), syscall.SHUT_RD)
}

// CloseWrite shuts down the writing side of a hvsock connection
func (v *hvsockConn) CloseWrite() error {
	return syscall.Shutdown(int(v.fd), syscall.SHUT_WR)
}

// Read reads data from the connection
func (v *hvsockConn) Read(buf []byte) (int, error) {
	return v.hvsock.Read(buf)
}

// Write writes data over the connection
// TODO(rn): replace with a straight call to v.hvsock.Write() once 4.9.x support is deprecated
func (v *hvsockConn) Write(buf []byte) (int, error) {
	written := 0
	toWrite := len(buf)
	for toWrite > 0 {
		thisBatch := min(toWrite, maxMsgSize)
		n, err := v.hvsock.Write(buf[written : written+thisBatch])
		if err != nil {
			return written, err
		}
		if n != thisBatch {
			return written, fmt.Errorf("short write %d != %d", n, thisBatch)
		}
		toWrite -= n
		written += n
	}

	return written, nil
}

// SetDeadline sets the read and write deadlines associated with the connection
func (v *hvsockConn) SetDeadline(t time.Time) error {
	return nil // FIXME
}

// SetReadDeadline sets the deadline for future Read calls.
func (v *hvsockConn) SetReadDeadline(t time.Time) error {
	return nil // FIXME
}

// SetWriteDeadline sets the deadline for future Write calls
func (v *hvsockConn) SetWriteDeadline(t time.Time) error {
	return nil // FIXME
}

// File duplicates the underlying socket descriptor and returns it.
func (v *hvsockConn) File() (*os.File, error) {
	// This is equivalent to dup(2) but creates the new fd with CLOEXEC already set.
	r0, _, e1 := syscall.Syscall(syscall.SYS_FCNTL, uintptr(v.hvsock.Fd()), syscall.F_DUPFD_CLOEXEC, 0)
	if e1 != 0 {
		return nil, os.NewSyscallError("fcntl", e1)
	}
	return os.NewFile(r0, v.hvsock.Name()), nil
}

func guidFromC(cg [16]C.uchar) GUID {
	var g GUID
	for i := 0; i < 16; i++ {
		g[i] = byte(cg[i])
	}
	return g
}
