package child

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/rootless-containers/rootlesskit/v3/pkg/lowlevelmsgutil"
	"github.com/rootless-containers/rootlesskit/v3/pkg/port"
	"github.com/rootless-containers/rootlesskit/v3/pkg/port/builtin/msg"
	opaquepkg "github.com/rootless-containers/rootlesskit/v3/pkg/port/builtin/opaque"
)

// probeTimeout bounds every backend-probe subprocess so a stuck or slow
// nft/iptables invocation can never hang the init handshake indefinitely.
const probeTimeout = 3 * time.Second

func NewDriver(logWriter io.Writer) port.ChildDriver {
	return &childDriver{
		logWriter: logWriter,
	}
}

type childDriver struct {
	logWriter                  io.Writer
	sourceIPTransparent        bool
	sourceIPTransparentBackend string // "auto" (default), "nft", or "iptables"

	// backendResolved is closed once resolveBackendName finishes (which
	// runs in a goroutine started during RunChildDriver). Any reader
	// must wait on this channel before accessing resolvedBackend.
	backendResolved chan struct{}
	resolvedBackend string // "nft", "iptables", or "" (none available)

	// routingSetup guards the actual rule installation (ip route/rule,
	// sysctl, nft/iptables rules). It runs at most once, on the first
	// real transparent connection.
	routingSetup sync.Once
	routingReady bool
}

func (d *childDriver) RunChildDriver(opaque map[string]string, quit <-chan struct{}, detachedNetNSPath string) error {
	d.sourceIPTransparent = opaque[opaquepkg.SourceIPTransparent] == "true"
	d.sourceIPTransparentBackend = opaque[opaquepkg.SourceIPTransparentBackend]
	if d.sourceIPTransparentBackend == "" {
		d.sourceIPTransparentBackend = "auto"
	}
	socketPath := opaque[opaquepkg.SocketPath]
	if socketPath == "" {
		return errors.New("socket path not set")
	}
	childReadyPipePath := opaque[opaquepkg.ChildReadyPipePath]
	if childReadyPipePath == "" {
		return errors.New("child ready pipe path not set")
	}
	childReadyPipeW, err := os.OpenFile(childReadyPipePath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return err
	}
	ln, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: socketPath,
		Net:  "unix",
	})
	if err != nil {
		return err
	}
	// Start backend name resolution asynchronously so that it runs in
	// parallel with the parent's connection setup. The ready pipe is
	// closed immediately (just like on master), so the parent is not
	// blocked. handleConnectInit waits on backendResolved before
	// reading resolvedBackend -- by that point the probe has had the
	// entire parent-side setup time to complete concurrently.
	//
	// The probe has to run in the same netns as the rule installation
	// (see routine), i.e., in the detached netns if it is set.
	// Otherwise the probe fails with EPERM, as the current netns is not
	// owned by the current userns.
	d.backendResolved = make(chan struct{})
	if d.sourceIPTransparent {
		go func() {
			defer close(d.backendResolved)
			if detachedNetNSPath == "" {
				d.resolveBackendName()
				return
			}
			if err := ns.WithNetNSPath(detachedNetNSPath, func(_ ns.NetNS) error {
				d.resolveBackendName()
				return nil
			}); err != nil {
				fmt.Fprintf(d.logWriter, "source IP transparent: failed to enter the detached netns %q: %v\n", detachedNetNSPath, err)
			}
		}()
	} else {
		close(d.backendResolved)
	}
	// write nothing, just close
	if err = childReadyPipeW.Close(); err != nil {
		return err
	}
	stopAccept := make(chan struct{}, 1)
	go func() {
		<-quit
		stopAccept <- struct{}{}
		ln.Close()
	}()
	for {
		c, err := ln.AcceptUnix()
		if err != nil {
			select {
			case <-stopAccept:
				return nil
			default:
			}
			return err
		}
		go func() {
			if rerr := d.routine(c, detachedNetNSPath); rerr != nil {
				rep := msg.Reply{
					Error: rerr.Error(),
				}
				lowlevelmsgutil.MarshalToWriter(c, &rep)
			}
			c.Close()
		}()
	}
}

func (d *childDriver) routine(c *net.UnixConn, detachedNetNSPath string) error {
	var req msg.Request
	if _, err := lowlevelmsgutil.UnmarshalFromReader(c, &req); err != nil {
		return err
	}
	switch req.Type {
	case msg.RequestTypeInit:
		return d.handleConnectInit(c, &req)
	case msg.RequestTypeConnect:
		if detachedNetNSPath == "" {
			return d.handleConnectRequest(c, &req)
		} else {
			return ns.WithNetNSPath(detachedNetNSPath, func(_ ns.NetNS) error {
				return d.handleConnectRequest(c, &req)
			})
		}
	default:
		return fmt.Errorf("unknown request type %q", req.Type)
	}
}

// resolveBackendName probes for nft/iptables availability, setting
// resolvedBackend. This is a cheap, read-only operation (LookPath + a
// smoke-test command) that does NOT install any firewall rules, routing
// entries, or sysctls.
//
// It is called once from RunChildDriver, before the child signals
// readiness to the parent, so that by the time the parent sends the init
// request the backend name is already available -- no external commands
// need to run in the init handshake path.
func (d *childDriver) resolveBackendName() {
	switch d.sourceIPTransparentBackend {
	case "nft":
		if probeNFT(d.logWriter) {
			d.resolvedBackend = "nft"
		}
	case "iptables":
		if probeIPTables(d.logWriter) {
			d.resolvedBackend = "iptables"
		}
	default: // "auto"
		if probeNFT(d.logWriter) {
			d.resolvedBackend = "nft"
		} else if probeIPTables(d.logWriter) {
			d.resolvedBackend = "iptables"
		}
	}
}

// ensureTransparentRoutingSetup installs the firewall rules, routing
// entries, and sysctls at most once (via routingSetup), setting
// routingReady. Called on the first real transparent connection.
//
// Every caller must go through this method rather than reading routingReady
// directly: init and connect requests are each handled in their own
// goroutine (see the accept loop in RunChildDriver), and it is
// sync.Once.Do itself -- not just "the setup already ran, earlier" -- that
// gives every caller a happens-before guarantee on those fields, regardless
// of which goroutine actually executes the closure.
func (d *childDriver) ensureTransparentRoutingSetup() {
	d.routingSetup.Do(func() {
		d.routingReady = d.setupTransparentRouting()
		if !d.routingReady {
			fmt.Fprintf(d.logWriter, "source IP transparent: falling back to non-transparent mode, client source IPs will not be preserved\n")
		}
	})
}

// handleConnectInit handles the "init" request.
func (d *childDriver) handleConnectInit(c *net.UnixConn, req *msg.Request) error {
	rep := msg.Reply{}
	if d.sourceIPTransparent {
		<-d.backendResolved
		rep.SourceIPTransparentBackend = d.resolvedBackend
	}
	_, err := lowlevelmsgutil.MarshalToWriter(c, &rep)
	return err
}

func (d *childDriver) handleConnectRequest(c *net.UnixConn, req *msg.Request) error {
	switch req.Proto {
	case "tcp":
	case "tcp4":
	case "tcp6":
	case "udp":
	case "udp4":
	case "udp6":
	default:
		return fmt.Errorf("unknown proto: %q", req.Proto)
	}
	// dialProto does not need "4", "6" suffix
	dialProto := strings.TrimSuffix(strings.TrimSuffix(req.Proto, "6"), "4")
	ip := req.IP
	if ip == "" {
		ip = "127.0.0.1"
		if req.ParentIP != "" {
			if req.ParentIP != req.HostGatewayIP && req.ParentIP != "0.0.0.0" {
				ip = req.ParentIP
			}
		}
	} else {
		p := net.ParseIP(ip)
		if p == nil {
			return fmt.Errorf("invalid IP: %q", ip)
		}
		ip = p.String()
	}
	targetAddr := net.JoinHostPort(ip, strconv.Itoa(req.Port))

	var targetConn net.Conn
	var err error
	// IP_TRANSPARENT source IP preservation is only supported for TCP.
	//
	// For UDP it cannot be made to work reliably: the in-netns server replies to
	// the real (non-local) client address, and unlike TCP there is no per-flow
	// accepted socket to carry the fwmark (no udp_fwmark_accept), so the reply's
	// route and source address are selected at send time via the main table. The
	// reply is therefore sent out the default route (e.g. the slirp4netns TAP)
	// and never reaches the transparent socket, breaking UDP forwarding entirely
	// for non-loopback clients (rootless-containers/rootlesskit#592). UDP falls
	// back to the non-transparent path below, which works for all clients but
	// does not preserve the client source IP.
	if d.sourceIPTransparent && req.SourceIP != "" && req.SourcePort != 0 && dialProto == "tcp" && !net.ParseIP(req.SourceIP).IsLoopback() {
		d.ensureTransparentRoutingSetup()
		if !d.routingReady {
			goto fallback
		}
		targetConn, err = transparentDial(dialProto, targetAddr, req.SourceIP, req.SourcePort)
		if err != nil {
			fmt.Fprintf(d.logWriter, "transparent dial failed, falling back: %v\n", err)
		}
	}
fallback:
	if targetConn == nil {
		var dialer net.Dialer
		targetConn, err = dialer.Dial(dialProto, targetAddr)
		if err != nil {
			return err
		}
	}
	defer targetConn.Close() // no effect on duplicated FD
	targetConnFiler, ok := targetConn.(filer)
	if !ok {
		return fmt.Errorf("unknown target connection: %+v", targetConn)
	}
	targetConnFile, err := targetConnFiler.File()
	if err != nil {
		return err
	}
	defer targetConnFile.Close()
	oob := unix.UnixRights(int(targetConnFile.Fd()))
	f, err := c.File()
	if err != nil {
		return err
	}
	defer f.Close()
	for {
		err = unix.Sendmsg(int(f.Fd()), []byte("dummy"), oob, nil, 0)
		if err != unix.EINTR {
			break
		}
	}
	return err
}

// setupTransparentRouting sets up policy routing so that response packets
// destined to transparent-bound source IPs are delivered locally.
// The firewall rules are implemented via nft, falling back to iptables if
// nft isn't available on the host (see setupTransparentRoutingNFT and
// setupTransparentRoutingIPTables).
//
// Transparent sockets (IP_TRANSPARENT) bind to non-local addresses (the real
// client IP). Response packets to these addresses must be routed locally instead
// of being sent out through the TAP device (slirp4netns).
//
// The transparent SYN goes through OUTPUT (where we tag it with CONNMARK) and
// then either:
//
//  1. Gets DNAT'd to the container (nerdctl/CNI): the SYN-ACK arrives via the
//     bridge in PREROUTING, where we restore connmark to fwmark.
//
//  2. Goes through loopback to a userspace proxy like docker-proxy: the SYN
//     enters PREROUTING on loopback with connmark, which sets fwmark. With
//     tcp_fwmark_accept=1, the accepted socket inherits the fwmark. The proxy's
//     SYN-ACK is then routed via the fwmark table (local delivery) instead of
//     the default route (TAP), allowing it to reach the transparent socket.
func (d *childDriver) setupTransparentRouting() bool {
	// Wait for the async backend probe (started in RunChildDriver) to
	// finish before reading resolvedBackend.
	<-d.backendResolved
	if d.resolvedBackend == "" {
		return false
	}

	// Common prep, independent of the firewall backend used below.
	prepCmds := [][]string{
		// Table 100: treat all addresses as local (for delivery to transparent sockets)
		{"ip", "route", "add", "local", "default", "dev", "lo", "table", "100"},
		{"ip", "-6", "route", "add", "local", "default", "dev", "lo", "table", "100"},
		// Route fwmark-100 packets via table 100
		{"ip", "rule", "add", "fwmark", "100", "lookup", "100", "priority", "100"},
		{"ip", "-6", "rule", "add", "fwmark", "100", "lookup", "100", "priority", "100"},
		// Inherit fwmark from SYN to accepted socket (needed for userspace proxies
		// like docker-proxy, so that SYN-ACK routing uses table 100)
		{"sysctl", "-w", "net.ipv4.tcp_fwmark_accept=1"},
	}
	for _, args := range prepCmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			fmt.Fprintf(d.logWriter, "source IP transparent routing setup: %v: %s\n", err, out)
		}
	}
	switch d.resolvedBackend {
	case "nft":
		d.installNFTRules()
	case "iptables":
		d.installIPTablesRules()
	}
	return true
}

// probeNFT checks whether nft is available and usable (read-only).
func probeNFT(logWriter io.Writer) bool {
	if _, err := exec.LookPath("nft"); err != nil {
		fmt.Fprintf(logWriter, "source IP transparent (nft): nft not found, disabling: %v\n", err)
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "nft", "list", "tables").CombinedOutput(); err != nil {
		fmt.Fprintf(logWriter, "source IP transparent (nft): nft not available, disabling: %v: %s\n", err, out)
		return false
	}
	return true
}

// installNFTRules installs the nftables rules for transparent routing.
// The "inet" family covers both IPv4 and IPv6 in a single ruleset.
// Must only be called after probeNFT has returned true.
func (d *childDriver) installNFTRules() {
	const nftTable = "rootlesskit_transparent"
	cmds := [][]string{
		// Create a single nftables table in the "inet" family, which covers both
		// IPv4 and IPv6, replacing the separate iptables/ip6tables tables used below.
		{"nft", "add", "table", "inet", nftTable},
		// Hook into OUTPUT and PREROUTING at the "mangle" priority, matching where
		// the equivalent rules live in the iptables mangle table.
		{"nft", "add", "chain", "inet", nftTable, "output",
			"{", "type", "filter", "hook", "output", "priority", "mangle", ";", "}"},
		{"nft", "add", "chain", "inet", nftTable, "prerouting",
			"{", "type", "filter", "hook", "prerouting", "priority", "mangle", ";", "}"},
		// In OUTPUT: tag transparent connections (non-local source) with a connection
		// mark. Equivalent to iptables: -m addrtype ! --src-type LOCAL -j CONNMARK --set-mark 100
		{"nft", "add", "rule", "inet", nftTable, "output",
			"meta", "l4proto", "tcp", "fib", "saddr", "type", "!=", "local", "ct", "mark", "set", "100"},
		// In PREROUTING: restore the connmark to the packet mark for routing.
		// Equivalent to iptables: -m connmark --mark 100 -j MARK --set-mark 100
		{"nft", "add", "rule", "inet", nftTable, "prerouting",
			"meta", "l4proto", "tcp", "ct", "mark", "100", "meta", "mark", "set", "100"},
	}
	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			fmt.Fprintf(d.logWriter, "source IP transparent (nft) routing setup: %v: %s\n", err, out)
		}
	}
}

// probeIPTables checks whether iptables is available and usable (read-only).
func probeIPTables(logWriter io.Writer) bool {
	if _, err := exec.LookPath("iptables"); err != nil {
		fmt.Fprintf(logWriter, "source IP transparent (iptables): iptables not found, disabling: %v\n", err)
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "iptables", "-t", "mangle", "-L", "-n").CombinedOutput(); err != nil {
		fmt.Fprintf(logWriter, "source IP transparent (iptables): mangle table not available, disabling: %v: %s\n", err, out)
		return false
	}
	return true
}

// installIPTablesRules installs iptables/ip6tables rules for transparent
// routing, kept as a fallback for hosts where nft is unavailable.
// Must only be called after probeIPTables has returned true.
func (d *childDriver) installIPTablesRules() {
	cmds := [][]string{
		// In OUTPUT: tag transparent connections (non-local source) with CONNMARK
		{"iptables", "-t", "mangle", "-A", "OUTPUT", "-p", "tcp", "-m", "addrtype", "!", "--src-type", "LOCAL", "-j", "CONNMARK", "--set-mark", "100"},
		{"ip6tables", "-t", "mangle", "-A", "OUTPUT", "-p", "tcp", "-m", "addrtype", "!", "--src-type", "LOCAL", "-j", "CONNMARK", "--set-mark", "100"},
		// In PREROUTING: restore connmark to fwmark for routing
		{"iptables", "-t", "mangle", "-A", "PREROUTING", "-p", "tcp", "-m", "connmark", "--mark", "100", "-j", "MARK", "--set-mark", "100"},
		{"ip6tables", "-t", "mangle", "-A", "PREROUTING", "-p", "tcp", "-m", "connmark", "--mark", "100", "-j", "MARK", "--set-mark", "100"},
	}
	for _, args := range cmds {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			fmt.Fprintf(d.logWriter, "source IP transparent (iptables) routing setup: %v: %s\n", err, out)
		}
	}
}

// transparentDial dials targetAddr using IP_TRANSPARENT, binding to the given
// source IP and port so the backend service sees the real client address.
// Only TCP is supported; see the comment in handleConnectRequest.
func transparentDial(dialProto, targetAddr, sourceIP string, sourcePort int) (net.Conn, error) {
	dialer := net.Dialer{
		Timeout:   time.Second,
		LocalAddr: &net.TCPAddr{IP: net.ParseIP(sourceIP), Port: sourcePort},
		Control: func(network, address string, c syscall.RawConn) error {
			var sockErr error
			if err := c.Control(func(fd uintptr) {
				if strings.Contains(network, "6") {
					sockErr = unix.SetsockoptInt(int(fd), unix.SOL_IPV6, unix.IPV6_TRANSPARENT, 1)
				} else {
					sockErr = unix.SetsockoptInt(int(fd), unix.SOL_IP, unix.IP_TRANSPARENT, 1)
				}
			}); err != nil {
				return err
			}
			return sockErr
		},
	}
	return dialer.Dial(dialProto, targetAddr)
}

// filer is implemented by *net.TCPConn and *net.UDPConn
type filer interface {
	File() (f *os.File, err error)
}
