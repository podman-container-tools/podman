// SPDX-License-Identifier: BSD-3-Clause
//go:build darwin

package process

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/internal/common"
	"github.com/shirou/gopsutil/v4/net"
)

// copied from sys/sysctl.h
const (
	CTLKern          = 1  // "high kernel": proc, limits
	KernProc         = 14 // struct: process entries
	KernProcPID      = 1  // by process id
	KernProcProc     = 8  // only return procs
	KernProcAll      = 0  // everything
	KernProcPathname = 12 // path to executable
)

type _Ctype_struct___0 struct { //nolint:revive //FIXME
	Pad uint64
}

func pidsWithContext(_ context.Context) ([]int32, error) {
	var ret []int32

	kprocs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return ret, err
	}

	for i := range kprocs {
		proc := &kprocs[i]
		ret = append(ret, int32(proc.Proc.P_pid))
	}

	return ret, nil
}

func (p *Process) PpidWithContext(_ context.Context) (int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}

	return k.Eproc.Ppid, nil
}

func (p *Process) NameWithContext(ctx context.Context) (string, error) {
	k, err := p.getKProc()
	if err != nil {
		return "", err
	}

	name := common.ByteToString(k.Proc.P_comm[:])

	if len(name) >= 15 {
		cmdName, err := p.cmdNameWithContext(ctx)
		if err != nil {
			return "", err
		}
		if cmdName != "" {
			extendedName := filepath.Base(cmdName)
			if strings.HasPrefix(extendedName, p.name) {
				name = extendedName
			}
		}
	}

	return name, nil
}

func (p *Process) createTimeWithContext(_ context.Context) (int64, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}

	return k.Proc.P_starttime.Sec*1000 + int64(k.Proc.P_starttime.Usec)/1000, nil
}

func (p *Process) StatusWithContext(ctx context.Context) ([]string, error) {
	r, err := callPsWithContext(ctx, "state", p.Pid, false, false)
	if err != nil {
		return []string{""}, err
	}
	status := convertStatusChar(r[0][0][0:1])
	return []string{status}, err
}

func (p *Process) ForegroundWithContext(ctx context.Context) (bool, error) {
	// see https://github.com/shirou/gopsutil/issues/596#issuecomment-432707831 for implementation details
	pid := p.Pid
	out, err := invoke.CommandWithContext(ctx, "ps", "-o", "stat=", "-p", strconv.Itoa(int(pid)))
	if err != nil {
		return false, err
	}
	return strings.IndexByte(string(out), '+') != -1, nil
}

func (p *Process) UidsWithContext(_ context.Context) ([]uint32, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}

	// See: http://unix.superglobalmegacorp.com/Net2/newsrc/sys/ucred.h.html
	userEffectiveUID := uint32(k.Eproc.Ucred.Uid)

	return []uint32{userEffectiveUID}, nil
}

func (p *Process) GidsWithContext(_ context.Context) ([]uint32, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}

	gids := make([]uint32, 0, 3)
	gids = append(gids, uint32(k.Eproc.Pcred.P_rgid), uint32(k.Eproc.Ucred.Groups[0]), uint32(k.Eproc.Pcred.P_svgid))

	return gids, nil
}

func (*Process) GroupsWithContext(_ context.Context) ([]uint32, error) {
	return nil, common.ErrNotImplementedError
	// k, err := p.getKProc()
	// if err != nil {
	// 	return nil, err
	// }

	// groups := make([]int32, k.Eproc.Ucred.Ngroups)
	// for i := int16(0); i < k.Eproc.Ucred.Ngroups; i++ {
	// 	groups[i] = int32(k.Eproc.Ucred.Groups[i])
	// }

	// return groups, nil
}

func (*Process) TerminalWithContext(_ context.Context) (string, error) {
	return "", common.ErrNotImplementedError
	/*
		k, err := p.getKProc()
		if err != nil {
			return "", err
		}

		ttyNr := uint64(k.Eproc.Tdev)
		termmap, err := getTerminalMap()
		if err != nil {
			return "", err
		}

		return termmap[ttyNr], nil
	*/
}

func (p *Process) NiceWithContext(_ context.Context) (int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}
	return int32(k.Proc.P_nice), nil
}

func (*Process) IOCountersWithContext(_ context.Context) (*IOCountersStat, error) {
	return nil, common.ErrNotImplementedError
}

func (p *Process) ChildrenWithContext(ctx context.Context) ([]*Process, error) {
	procs, err := ProcessesWithContext(ctx)
	if err != nil {
		return nil, nil
	}
	ret := make([]*Process, 0, len(procs))
	for _, proc := range procs {
		ppid, err := proc.PpidWithContext(ctx)
		if err != nil {
			continue
		}
		if ppid == p.Pid {
			ret = append(ret, proc)
		}
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].Pid < ret[j].Pid })
	return ret, nil
}

func (p *Process) ConnectionsWithContext(ctx context.Context) ([]net.ConnectionStat, error) {
	return net.ConnectionsPidWithContext(ctx, "all", p.Pid)
}

func (p *Process) ConnectionsMaxWithContext(ctx context.Context, maxConn int) ([]net.ConnectionStat, error) {
	return net.ConnectionsPidMaxWithContext(ctx, "all", p.Pid, maxConn)
}

func ProcessesWithContext(ctx context.Context) ([]*Process, error) {
	out := []*Process{}

	pids, err := PidsWithContext(ctx)
	if err != nil {
		return out, err
	}

	for _, pid := range pids {
		p, err := NewProcessWithContext(ctx, pid)
		if err != nil {
			continue
		}
		out = append(out, p)
	}

	return out, nil
}

// Returns a proc as defined here:
// http://unix.superglobalmegacorp.com/Net2/newsrc/sys/kinfo_proc.h.html
func (p *Process) getKProc() (*unix.KinfoProc, error) {
	return unix.SysctlKinfoProc("kern.proc.pid", int(p.Pid))
}

// call ps command.
// Return value deletes Header line(you must not input wrong arg).
// And split by Space. Caller have responsibility to manage.
// If passed arg pid is 0, get information from all process.
func callPsWithContext(ctx context.Context, arg string, pid int32, threadOption, nameOption bool) ([][]string, error) {
	var cmd []string
	switch {
	case pid == 0: // will get from all processes.
		cmd = []string{"-ax", "-o", arg}
	case threadOption:
		cmd = []string{"-x", "-o", arg, "-M", "-p", strconv.Itoa(int(pid))}
	default:
		cmd = []string{"-x", "-o", arg, "-p", strconv.Itoa(int(pid))}
	}
	if nameOption {
		cmd = append(cmd, "-c")
	}
	out, err := invoke.CommandWithContext(ctx, "ps", cmd...)
	if err != nil {
		return [][]string{}, err
	}
	lines := strings.Split(string(out), "\n")

	var ret [][]string
	for _, l := range lines[1:] {
		var lr []string
		if nameOption {
			lr = append(lr, l)
		} else {
			for _, r := range strings.Split(l, " ") {
				if r == "" {
					continue
				}
				lr = append(lr, strings.TrimSpace(r))
			}
		}
		if len(lr) != 0 {
			ret = append(ret, lr)
		}
	}

	return ret, nil
}

type dlFuncs struct {
	lib *common.SystemLib
}

func loadProcFuncs() (*dlFuncs, error) {
	lib, err := common.NewSystemLib()
	if err != nil {
		return nil, err
	}
	return &dlFuncs{lib}, err
}

func (f *dlFuncs) getTimeScaleToNanoSeconds() float64 {
	var timeBaseInfo common.MachTimeBaseInfo

	f.lib.MachTimeBaseInfo(uintptr(unsafe.Pointer(&timeBaseInfo)))

	return float64(timeBaseInfo.Numer) / float64(timeBaseInfo.Denom)
}

func (f *dlFuncs) Close() {
	f.lib.Close()
}

func (p *Process) ExeWithContext(_ context.Context) (string, error) {
	funcs, err := loadProcFuncs()
	if err != nil {
		return "", err
	}
	defer funcs.Close()

	buf := common.NewCStr(common.PROC_PIDPATHINFO_MAXSIZE)
	ret := funcs.lib.ProcPidPath(p.Pid, buf.Addr(), common.PROC_PIDPATHINFO_MAXSIZE)

	if ret <= 0 {
		return "", fmt.Errorf("unknown error: proc_pidpath returned %d", ret)
	}

	return buf.GoString(), nil
}

// CwdWithContext retrieves the Current Working Directory for the given process.
// It uses the proc_pidinfo from libproc and will only work for processes the
// EUID can access.  Otherwise "operation not permitted" will be returned as the
// error.
// Note: This might also work for other *BSD OSs.
func (p *Process) CwdWithContext(_ context.Context) (string, error) {
	funcs, err := loadProcFuncs()
	if err != nil {
		return "", err
	}
	defer funcs.Close()

	// Lock OS thread to ensure the errno does not change
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var vpi vnodePathInfo
	const vpiSize = int32(unsafe.Sizeof(vpi))
	ret := funcs.lib.ProcPidInfo(p.Pid, common.PROC_PIDVNODEPATHINFO, 0, uintptr(unsafe.Pointer(&vpi)), vpiSize)
	errno, _ := funcs.lib.Dlsym("errno")
	err = *(**unix.Errno)(unsafe.Pointer(&errno))
	if errors.Is(err, unix.EPERM) {
		return "", ErrorNotPermitted
	}

	if ret <= 0 {
		return "", fmt.Errorf("unknown error: proc_pidinfo returned %d", ret)
	}

	if ret != vpiSize {
		return "", fmt.Errorf("too few bytes; expected %d, got %d", vpiSize, ret)
	}
	return common.GoString((*byte)(unsafe.Pointer(&vpi.Cdir.Path[0]))), nil
}

func procArgs(pid int32) ([]byte, int, error) {
	procargs, err := unix.SysctlRaw("kern.procargs2", int(pid))
	if err != nil {
		return nil, 0, err
	}

	// The first 4 bytes indicate the number of arguments.
	nargs := procargs[:4]
	return procargs, int(binary.LittleEndian.Uint32(nargs)), nil
}

func (p *Process) CmdlineSliceWithContext(_ context.Context) ([]string, error) {
	return p.cmdlineSlice()
}

func (p *Process) cmdlineSlice() ([]string, error) {
	pargs, nargs, err := procArgs(p.Pid)
	if err != nil {
		return nil, err
	}
	// procArgs reads nargs as a 4-byte uint32; skip exactly those 4 bytes
	// (matches Apple's ps using sizeof(nargs)). The previous code used
	// unsafe.Sizeof(int(0)) which is 8 on 64-bit and would discard the
	// first 4 bytes of exec_path — harmless only because chunks[0] is
	// dropped, but logically wrong.
	return parseCmdline(pargs[4:], nargs), nil
}

// parseCmdline extracts argv from the kern.procargs2 buffer with the leading
// nargs int already stripped. Layout:
//
//	exec_path \0 [padding \0...] argv[0] \0 ... argv[nargs-1] \0 envp[0] \0 ...
//
// Empty argv elements within the nargs count are preserved — skipping them
// would advance past argv[nargs-1] into envp, leaking environment values
// (potentially secrets) into Cmdline output.
//
// Known limitation: a process whose argv[0] is itself an empty string is
// indistinguishable from padding by this parser, since XNU does not expose
// the exec/argv alignment boundary. Such a process will still see one envp
// entry leak. Fixing it requires libgetargv-style alignment math against
// the XNU exec layout, which is out of scope for this change.
func parseCmdline(args []byte, nargs int) []string {
	chunks := bytes.Split(args, []byte{0})
	if len(chunks) <= 1 {
		return nil
	}
	// Skip exec_path (chunks[0]) and any padding NULs before argv[0].
	i := 1
	for ; i < len(chunks) && len(chunks[i]) == 0; i++ {
	}
	if nargs > len(chunks)-i {
		nargs = len(chunks) - i
	}
	if nargs < 0 {
		nargs = 0
	}
	argSlice := make([]string, 0, nargs)
	for ; nargs > 0; nargs-- {
		argSlice = append(argSlice, string(chunks[i]))
		i++
	}
	return argSlice
}

// cmdNameWithContext returns the command name (including spaces) without any arguments
func (p *Process) cmdNameWithContext(_ context.Context) (string, error) {
	r, err := p.cmdlineSlice()
	if err != nil {
		return "", err
	}

	if len(r) == 0 {
		return "", nil
	}

	return r[0], err
}

func (p *Process) CmdlineWithContext(ctx context.Context) (string, error) {
	r, err := p.CmdlineSliceWithContext(ctx)
	if err != nil {
		return "", err
	}
	return strings.Join(r, " "), err
}

func (p *Process) NumThreadsWithContext(_ context.Context) (int32, error) {
	funcs, err := loadProcFuncs()
	if err != nil {
		return 0, err
	}
	defer funcs.Close()

	var ti ProcTaskInfo
	funcs.lib.ProcPidInfo(p.Pid, common.PROC_PIDTASKINFO, 0, uintptr(unsafe.Pointer(&ti)), int32(unsafe.Sizeof(ti)))

	return int32(ti.Threadnum), nil
}

func (p *Process) TimesWithContext(_ context.Context) (*cpu.TimesStat, error) {
	funcs, err := loadProcFuncs()
	if err != nil {
		return nil, err
	}
	defer funcs.Close()

	var ti ProcTaskInfo
	funcs.lib.ProcPidInfo(p.Pid, common.PROC_PIDTASKINFO, 0, uintptr(unsafe.Pointer(&ti)), int32(unsafe.Sizeof(ti)))

	timescaleToNanoSeconds := funcs.getTimeScaleToNanoSeconds()
	ret := &cpu.TimesStat{
		CPU:    "cpu",
		User:   float64(ti.Total_user) * timescaleToNanoSeconds / 1e9,
		System: float64(ti.Total_system) * timescaleToNanoSeconds / 1e9,
	}
	return ret, nil
}

func (p *Process) MemoryInfoWithContext(_ context.Context) (*MemoryInfoStat, error) {
	funcs, err := loadProcFuncs()
	if err != nil {
		return nil, err
	}
	defer funcs.Close()

	var ti ProcTaskInfo
	funcs.lib.ProcPidInfo(p.Pid, common.PROC_PIDTASKINFO, 0, uintptr(unsafe.Pointer(&ti)), int32(unsafe.Sizeof(ti)))

	ret := &MemoryInfoStat{
		RSS: uint64(ti.Resident_size),
		VMS: uint64(ti.Virtual_size),
	}
	return ret, nil
}

// procFDInfo represents a file descriptor entry from sys/proc_info.h
type procFDInfo struct {
	ProcFd     int32
	ProcFdtype uint32
}

// NumFDsWithContext returns the number of file descriptors used by the process.
// It uses proc_pidinfo with PROC_PIDLISTFDS to query the kernel for the count
// of open file descriptors. The method makes a single syscall and calculates
// the count from the buffer size returned by the kernel.
func (p *Process) NumFDsWithContext(_ context.Context) (int32, error) {
	funcs, err := loadProcFuncs()
	if err != nil {
		return 0, err
	}
	defer funcs.Close()

	// First call: get required buffer size
	bufferSize := funcs.lib.ProcPidInfo(
		p.Pid,
		common.PROC_PIDLISTFDS,
		0,
		0, // NULL buffer
		0, // 0 size
	)
	if bufferSize <= 0 {
		return 0, fmt.Errorf("unknown error: proc_pidinfo returned %d", bufferSize)
	}

	// Allocate buffer of the required size
	const sizeofProcFDInfo = int32(unsafe.Sizeof(procFDInfo{}))
	numEntries := bufferSize / sizeofProcFDInfo
	buf := make([]procFDInfo, numEntries)

	// Second call: get actual data
	ret := funcs.lib.ProcPidInfo(
		p.Pid,
		common.PROC_PIDLISTFDS,
		0,
		uintptr(unsafe.Pointer(&buf[0])), // Real buffer
		bufferSize,                       // Size from first call
	)
	if ret <= 0 {
		return 0, fmt.Errorf("unknown error: proc_pidinfo returned %d", ret)
	}

	// Calculate actual number of FDs returned
	numFDs := ret / sizeofProcFDInfo
	return numFDs, nil
}
