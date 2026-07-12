// Package hvsock provides a Go interface to Hyper-V sockets both on
// Windows and on Linux. The Linux bindings require patches for the
// 4.9.x kernel. If you are using a Linux kernel 4.14.x or newer you
// should use the vsock package instead as the Hyper-V socket support
// in these kernels have been merged with the virtio sockets
// implementation.
package hvsock

import (
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
)

var (
	// GUIDZero used by listeners to accept connections from all partitions
	GUIDZero, _ = GUIDFromString("00000000-0000-0000-0000-000000000000")
	// GUIDWildcard used by listeners to accept connections from all partitions
	GUIDWildcard, _ = GUIDFromString("00000000-0000-0000-0000-000000000000")
	// GUIDBroadcast undocumented
	GUIDBroadcast, _ = GUIDFromString("FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	// GUIDChildren used by listeners to accept connections from children
	GUIDChildren, _ = GUIDFromString("90db8b89-0d35-4f79-8ce9-49ea0ac8b7cd")
	// GUIDLoopback use to connect in loopback mode
	GUIDLoopback, _ = GUIDFromString("e0e16197-dd56-4a10-9195-5ee7a155a838")
	// GUIDParent use to connect to the parent partition
	GUIDParent, _ = GUIDFromString("a42e7cda-d03f-480c-9cc2-a4de20abb878")

	// GUIDs for LinuxVMs with the new Hyper-V socket implementation need to match this template
	guidTemplate, _ = GUIDFromString("00000000-facb-11e6-bd58-64006a7986d3")
)

const (
	// The Hyper-V socket implementation used in the 4.9.x kernels
	// seems to fail silently if messages are above 8k. The newer
	// implementation in the 4.14.x (and newer) kernels seems to
	// work fine with larger messages. This is constant is used as
	// a temporary workaround to limit the amount of data sent and
	// should be removed once support for 4.9.x kernels is
	// deprecated.
	maxMsgSize = 8 * 1024
)

// GUID is used by Hypper-V sockets for "addresses" and "ports"
type GUID [16]byte

// Convert a GUID into a string
func (g *GUID) String() string {
	/* XXX This assume little endian */
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		g[3], g[2], g[1], g[0],
		g[5], g[4],
		g[7], g[6],
		g[8], g[9],
		g[10], g[11], g[12], g[13], g[14], g[15])
}

// Port converts a Service GUID to a "port" usable by the vsock package.
// It can be used to convert hvsock code to vsock code. On 4.14.x
// kernels Service GUIDs for talking to Linux should have the form of
// xxxxxxxx-facb-11e6-bd58-64006a7986d3, where xxxxxxxx is the vsock port.
func (g *GUID) Port() (uint32, error) {
	// Check that the GUID is as expected
	if !reflect.DeepEqual(g[4:], guidTemplate[4:]) {
		return 0, fmt.Errorf("%s does not conform with the template", g)
	}
	return binary.LittleEndian.Uint32(g[0:4]), nil
}

// GUIDFromString parses a string and returns a GUID
func GUIDFromString(s string) (GUID, error) {
	var g GUID
	var err error
	_, err = fmt.Sscanf(s, "%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		&g[3], &g[2], &g[1], &g[0],
		&g[5], &g[4],
		&g[7], &g[6],
		&g[8], &g[9],
		&g[10], &g[11], &g[12], &g[13], &g[14], &g[15])
	return g, err
}

// Addr represents a Hyper-V socket address
type Addr struct {
	VMID      GUID
	ServiceID GUID
}

// Network returns the type of network for Hyper-V sockets
func (a Addr) Network() string {
	return "hvsock"
}

func (a Addr) String() string {
	vmid := a.VMID.String()
	svc := a.ServiceID.String()

	return vmid + ":" + svc
}

// Conn is a hvsock connection which supports half-close.
type Conn interface {
	net.Conn
	CloseRead() error
	CloseWrite() error
}

// Since there doesn't seem to be a standard min function
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
