// SPDX-License-Identifier: BSD-3-Clause
//go:build darwin

package common

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Library represents a dynamic library loaded by purego.
type library struct {
	handle uintptr
	fnMap  map[string]any
	mu     sync.RWMutex
}

// library paths
const (
	IOKitLibPath          = "/System/Library/Frameworks/IOKit.framework/IOKit"
	CoreFoundationLibPath = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
	SystemLibPath         = "/usr/lib/libSystem.B.dylib"
)

// Library handles are opened once and shared for the process lifetime.
// Opening and closing them on every call causes SIGBUS/SIGSEGV crashes because
// the Go runtime (GC, timers) can interact with invalidated library handles
// after Dlclose. Sharing also keeps the resolved-symbol cache in fnMap warm,
// so a symbol lookup is not repeated on every call.
// See: https://github.com/shirou/gopsutil/issues/1832
var (
	libCacheMu sync.Mutex
	libCache   = make(map[string]*library)
)

func newLibrary(path string) (*library, error) {
	libCacheMu.Lock()
	defer libCacheMu.Unlock()

	if lib, ok := libCache[path]; ok {
		return lib, nil
	}

	handle, err := purego.Dlopen(path, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, err
	}

	lib := &library{
		handle: handle,
		fnMap:  make(map[string]any),
	}
	libCache[path] = lib

	return lib, nil
}

func (lib *library) Dlsym(symbol string) (uintptr, error) {
	return purego.Dlsym(lib.handle, symbol)
}

// getFunc resolves a function pointer from the library, caching it in fnMap.
// Thread-safe via double-checked locking to support shared library handles.
func getFunc[T any](lib *library, symbol string) T {
	// Fast path: read lock only
	lib.mu.RLock()
	if f, ok := lib.fnMap[symbol].(*dlFunc[T]); ok {
		lib.mu.RUnlock()
		return f.fn
	}
	lib.mu.RUnlock()

	// Slow path: write lock for first-time resolution
	lib.mu.Lock()
	defer lib.mu.Unlock()

	// Double-check after acquiring write lock
	if f, ok := lib.fnMap[symbol].(*dlFunc[T]); ok {
		return f.fn
	}

	dlfun := newDlfunc[T](symbol)
	dlfun.init(lib.handle)
	lib.fnMap[symbol] = dlfun
	return dlfun.fn
}

// Close is a no-op, kept so that existing defer-Close call sites keep working.
// Library handles are shared process-wide and are deliberately never unloaded;
// see newLibrary for why.
func (*library) Close() {}

type dlFunc[T any] struct {
	sym string
	fn  T
}

func (d *dlFunc[T]) init(handle uintptr) {
	purego.RegisterLibFunc(&d.fn, handle, d.sym)
}

func newDlfunc[T any](sym string) *dlFunc[T] {
	return &dlFunc[T]{sym: sym}
}

type CoreFoundationLib struct {
	*library
}

func NewCoreFoundationLib() (*CoreFoundationLib, error) {
	library, err := newLibrary(CoreFoundationLibPath)
	if err != nil {
		return nil, err
	}
	return &CoreFoundationLib{library}, nil
}

func (c *CoreFoundationLib) CFGetTypeID(cf uintptr) int64 {
	fn := getFunc[CFGetTypeIDFunc](c.library, "CFGetTypeID")
	return fn(cf)
}

func (c *CoreFoundationLib) CFNumberCreate(allocator uintptr, theType int64, valuePtr unsafe.Pointer) unsafe.Pointer {
	fn := getFunc[CFNumberCreateFunc](c.library, "CFNumberCreate")
	return fn(allocator, theType, valuePtr)
}

func (c *CoreFoundationLib) CFNumberGetValue(num uintptr, theType int64, valuePtr unsafe.Pointer) bool {
	fn := getFunc[CFNumberGetValueFunc](c.library, "CFNumberGetValue")
	return fn(num, theType, valuePtr)
}

func (c *CoreFoundationLib) CFDictionaryCreate(allocator uintptr, keys, values *unsafe.Pointer, numValues int64,
	keyCallBacks, valueCallBacks uintptr,
) unsafe.Pointer {
	fn := getFunc[CFDictionaryCreateFunc](c.library, "CFDictionaryCreate")
	return fn(allocator, keys, values, numValues, keyCallBacks, valueCallBacks)
}

func (c *CoreFoundationLib) CFDictionaryAddValue(theDict, key, value uintptr) {
	fn := getFunc[CFDictionaryAddValueFunc](c.library, "CFDictionaryAddValue")
	fn(theDict, key, value)
}

func (c *CoreFoundationLib) CFDictionaryGetValue(theDict, key uintptr) unsafe.Pointer {
	fn := getFunc[CFDictionaryGetValueFunc](c.library, "CFDictionaryGetValue")
	return fn(theDict, key)
}

func (c *CoreFoundationLib) CFArrayGetCount(theArray uintptr) int64 {
	fn := getFunc[CFArrayGetCountFunc](c.library, "CFArrayGetCount")
	return fn(theArray)
}

func (c *CoreFoundationLib) CFArrayGetValueAtIndex(theArray uintptr, index int64) unsafe.Pointer {
	fn := getFunc[CFArrayGetValueAtIndexFunc](c.library, "CFArrayGetValueAtIndex")
	return fn(theArray, index)
}

func (c *CoreFoundationLib) CFStringCreateMutable(alloc uintptr, maxLength int64) unsafe.Pointer {
	fn := getFunc[CFStringCreateMutableFunc](c.library, "CFStringCreateMutable")
	return fn(alloc, maxLength)
}

func (c *CoreFoundationLib) CFStringGetLength(theString uintptr) int64 {
	fn := getFunc[CFStringGetLengthFunc](c.library, "CFStringGetLength")
	return fn(theString)
}

func (c *CoreFoundationLib) CFStringGetCString(theString uintptr, buffer CStr, bufferSize int64, encoding uint32) {
	fn := getFunc[CFStringGetCStringFunc](c.library, "CFStringGetCString")
	fn(theString, buffer, bufferSize, encoding)
}

func (c *CoreFoundationLib) CFStringCreateWithCString(alloc uintptr, cStr string, encoding uint32) unsafe.Pointer {
	fn := getFunc[CFStringCreateWithCStringFunc](c.library, "CFStringCreateWithCString")
	return fn(alloc, cStr, encoding)
}

func (c *CoreFoundationLib) CFDataGetLength(theData uintptr) int64 {
	fn := getFunc[CFDataGetLengthFunc](c.library, "CFDataGetLength")
	return fn(theData)
}

func (c *CoreFoundationLib) CFDataGetBytePtr(theData uintptr) unsafe.Pointer {
	fn := getFunc[CFDataGetBytePtrFunc](c.library, "CFDataGetBytePtr")
	return fn(theData)
}

func (c *CoreFoundationLib) CFRelease(cf uintptr) {
	fn := getFunc[CFReleaseFunc](c.library, "CFRelease")
	fn(cf)
}

type IOKitLib struct {
	*library
}

func NewIOKitLib() (*IOKitLib, error) {
	library, err := newLibrary(IOKitLibPath)
	if err != nil {
		return nil, err
	}
	return &IOKitLib{library}, nil
}

func (l *IOKitLib) IOServiceGetMatchingService(mainPort uint32, matching uintptr) uint32 {
	fn := getFunc[IOServiceGetMatchingServiceFunc](l.library, "IOServiceGetMatchingService")
	return fn(mainPort, matching)
}

func (l *IOKitLib) IOServiceGetMatchingServices(mainPort uint32, matching uintptr, existing *uint32) int32 {
	fn := getFunc[IOServiceGetMatchingServicesFunc](l.library, "IOServiceGetMatchingServices")
	return fn(mainPort, matching, existing)
}

func (l *IOKitLib) IOServiceMatching(name string) unsafe.Pointer {
	fn := getFunc[IOServiceMatchingFunc](l.library, "IOServiceMatching")
	return fn(name)
}

func (l *IOKitLib) IOServiceOpen(service, owningTask, connType uint32, connect *uint32) int32 {
	fn := getFunc[IOServiceOpenFunc](l.library, "IOServiceOpen")
	return fn(service, owningTask, connType, connect)
}

func (l *IOKitLib) IOServiceClose(connect uint32) int32 {
	fn := getFunc[IOServiceCloseFunc](l.library, "IOServiceClose")
	return fn(connect)
}

func (l *IOKitLib) IOIteratorNext(iterator uint32) uint32 {
	fn := getFunc[IOIteratorNextFunc](l.library, "IOIteratorNext")
	return fn(iterator)
}

func (l *IOKitLib) IORegistryEntryGetName(entry uint32, name CStr) int32 {
	fn := getFunc[IORegistryEntryGetNameFunc](l.library, "IORegistryEntryGetName")
	return fn(entry, name)
}

func (l *IOKitLib) IORegistryEntryGetParentEntry(entry uint32, plane string, parent *uint32) int32 {
	fn := getFunc[IORegistryEntryGetParentEntryFunc](l.library, "IORegistryEntryGetParentEntry")
	return fn(entry, plane, parent)
}

func (l *IOKitLib) IORegistryEntryCreateCFProperty(entry uint32, key, allocator uintptr, options uint32) unsafe.Pointer {
	fn := getFunc[IORegistryEntryCreateCFPropertyFunc](l.library, "IORegistryEntryCreateCFProperty")
	return fn(entry, key, allocator, options)
}

func (l *IOKitLib) IORegistryEntryCreateCFProperties(entry uint32, properties unsafe.Pointer, allocator uintptr, options uint32) int32 {
	fn := getFunc[IORegistryEntryCreateCFPropertiesFunc](l.library, "IORegistryEntryCreateCFProperties")
	return fn(entry, properties, allocator, options)
}

func (l *IOKitLib) IOObjectConformsTo(object uint32, className string) bool {
	fn := getFunc[IOObjectConformsToFunc](l.library, "IOObjectConformsTo")
	return fn(object, className)
}

func (l *IOKitLib) IOObjectRelease(object uint32) int32 {
	fn := getFunc[IOObjectReleaseFunc](l.library, "IOObjectRelease")
	return fn(object)
}

func (l *IOKitLib) IOConnectCallStructMethod(connection, selector uint32, inputStruct unsafe.Pointer, inputStructCnt uintptr,
	outputStruct unsafe.Pointer, outputStructCnt *uintptr,
) int32 {
	fn := getFunc[IOConnectCallStructMethodFunc](l.library, "IOConnectCallStructMethod")
	return fn(connection, selector, inputStruct, inputStructCnt, outputStruct, outputStructCnt)
}

func (l *IOKitLib) IOHIDEventSystemClientCreate(allocator uintptr) unsafe.Pointer {
	fn := getFunc[IOHIDEventSystemClientCreateFunc](l.library, "IOHIDEventSystemClientCreate")
	return fn(allocator)
}

func (l *IOKitLib) IOHIDEventSystemClientSetMatching(client, match uintptr) int32 {
	fn := getFunc[IOHIDEventSystemClientSetMatchingFunc](l.library, "IOHIDEventSystemClientSetMatching")
	return fn(client, match)
}

func (l *IOKitLib) IOHIDServiceClientCopyEvent(service uintptr, eventType int64, options int32, timeout int64) unsafe.Pointer {
	fn := getFunc[IOHIDServiceClientCopyEventFunc](l.library, "IOHIDServiceClientCopyEvent")
	return fn(service, eventType, options, timeout)
}

func (l *IOKitLib) IOHIDServiceClientCopyProperty(service, property uintptr) unsafe.Pointer {
	fn := getFunc[IOHIDServiceClientCopyPropertyFunc](l.library, "IOHIDServiceClientCopyProperty")
	return fn(service, property)
}

func (l *IOKitLib) IOHIDEventGetFloatValue(event uintptr, field int32) float64 {
	fn := getFunc[IOHIDEventGetFloatValueFunc](l.library, "IOHIDEventGetFloatValue")
	return fn(event, field)
}

func (l *IOKitLib) IOHIDEventSystemClientCopyServices(client uintptr) unsafe.Pointer {
	fn := getFunc[IOHIDEventSystemClientCopyServicesFunc](l.library, "IOHIDEventSystemClientCopyServices")
	return fn(client)
}

type SystemLib struct {
	*library
}

func NewSystemLib() (*SystemLib, error) {
	library, err := newLibrary(SystemLibPath)
	if err != nil {
		return nil, err
	}
	return &SystemLib{library}, nil
}

func (s *SystemLib) HostProcessorInfo(host uint32, flavor int32, outProcessorCount *uint32, outProcessorInfo unsafe.Pointer,
	outProcessorInfoCnt *uint32,
) int32 {
	fn := getFunc[HostProcessorInfoFunc](s.library, "host_processor_info")
	return fn(host, flavor, outProcessorCount, outProcessorInfo, outProcessorInfoCnt)
}

func (s *SystemLib) HostStatistics(host uint32, flavor int32, hostInfoOut unsafe.Pointer, hostInfoOutCnt *uint32) int32 {
	fn := getFunc[HostStatisticsFunc](s.library, "host_statistics")
	return fn(host, flavor, hostInfoOut, hostInfoOutCnt)
}

func (s *SystemLib) MachHostSelf() uint32 {
	fn := getFunc[MachHostSelfFunc](s.library, "mach_host_self")
	return fn()
}

func (s *SystemLib) MachTaskSelf() uint32 {
	fn := getFunc[MachTaskSelfFunc](s.library, "mach_task_self")
	return fn()
}

func (s *SystemLib) MachTimeBaseInfo(info unsafe.Pointer) int32 {
	fn := getFunc[MachTimeBaseInfoFunc](s.library, "mach_timebase_info")
	return fn(info)
}

func (s *SystemLib) VMDeallocate(targetTask uint32, vmAddress, vmSize uintptr) int32 {
	fn := getFunc[VMDeallocateFunc](s.library, "vm_deallocate")
	return fn(targetTask, vmAddress, vmSize)
}

func (s *SystemLib) ProcPidPath(pid int32, buffer unsafe.Pointer, bufferSize uint32) int32 {
	fn := getFunc[ProcPidPathFunc](s.library, "proc_pidpath")
	return fn(pid, buffer, bufferSize)
}

func (s *SystemLib) ProcPidInfo(pid, flavor int32, arg uint64, buffer unsafe.Pointer, bufferSize int32) int32 {
	fn := getFunc[ProcPidInfoFunc](s.library, "proc_pidinfo")
	return fn(pid, flavor, arg, buffer, bufferSize)
}

func (s *SystemLib) ProcPidRusage(pid, flavor int32, buffer unsafe.Pointer) int32 {
	fn := getFunc[ProcPidRusageFunc](s.library, "proc_pid_rusage")
	return fn(pid, flavor, buffer)
}

// ErrnoLocation returns a pointer to the calling OS thread's errno, as given by
// libc's __error(). The pointer is thread-local, so callers must hold
// runtime.LockOSThread() across this call, the libc call being checked, and the
// dereference of the returned pointer.
//
// Resolve the pointer *before* the libc call whose errno is to be inspected.
// Resolving a symbol performs a dlsym and allocates, and either may overwrite
// errno; doing it afterwards would race with the value being read.
func (s *SystemLib) ErrnoLocation() *int32 {
	fn := getFunc[ErrnoFunc](s.library, "__error")
	return fn()
}

// status codes
const (
	KERN_SUCCESS = 0
)

// Arguments that point at Go memory are declared as unsafe.Pointer, a Go
// pointer type, or a slice -- never as uintptr. purego maps uintptr to
// uintptr_t, i.e. a plain integer: it neither keeps the pointee alive nor makes
// it escape, so a Go local stays on the goroutine stack and the address goes
// stale the moment the stack grows. Every such call then reads or writes the old
// stack and silently sees or produces zeroes. The other kinds are passed through
// reflect.Value.Pointer() and stay reachable for the duration of the call.
//
// uintptr remains correct for handles that are not Go memory: CoreFoundation and
// IOKit object references, mach ports, addresses of dylib data symbols obtained
// through Dlsym, and kernel-allocated vm addresses.

// IOKit types and constants.
type (
	IOServiceGetMatchingServiceFunc       func(mainPort uint32, matching uintptr) uint32
	IOServiceGetMatchingServicesFunc      func(mainPort uint32, matching uintptr, existing *uint32) int32
	IOServiceMatchingFunc                 func(name string) unsafe.Pointer
	IOServiceOpenFunc                     func(service, owningTask, connType uint32, connect *uint32) int32
	IOServiceCloseFunc                    func(connect uint32) int32
	IOIteratorNextFunc                    func(iterator uint32) uint32
	IORegistryEntryGetNameFunc            func(entry uint32, name CStr) int32
	IORegistryEntryGetParentEntryFunc     func(entry uint32, plane string, parent *uint32) int32
	IORegistryEntryCreateCFPropertyFunc   func(entry uint32, key, allocator uintptr, options uint32) unsafe.Pointer
	IORegistryEntryCreateCFPropertiesFunc func(entry uint32, properties unsafe.Pointer, allocator uintptr, options uint32) int32
	IOObjectConformsToFunc                func(object uint32, className string) bool
	IOObjectReleaseFunc                   func(object uint32) int32
	IOConnectCallStructMethodFunc         func(connection, selector uint32, inputStruct unsafe.Pointer, inputStructCnt uintptr,
		outputStruct unsafe.Pointer, outputStructCnt *uintptr) int32

	IOHIDEventSystemClientCreateFunc      func(allocator uintptr) unsafe.Pointer
	IOHIDEventSystemClientSetMatchingFunc func(client, match uintptr) int32
	IOHIDServiceClientCopyEventFunc       func(service uintptr, eventType int64,
		options int32, timeout int64) unsafe.Pointer
	IOHIDServiceClientCopyPropertyFunc     func(service, property uintptr) unsafe.Pointer
	IOHIDEventGetFloatValueFunc            func(event uintptr, field int32) float64
	IOHIDEventSystemClientCopyServicesFunc func(client uintptr) unsafe.Pointer
)

const (
	KIOMainPortDefault = 0

	KIOHIDEventTypeTemperature = 15

	KNilOptions = 0
)

const (
	KIOMediaWholeKey = "Media"
	KIOServicePlane  = "IOService"
)

// CoreFoundation types and constants.
//
// valuePtr on CFNumberCreate and CFNumberGetValue points at a caller-owned Go
// value that CoreFoundation reads from or writes into, hence unsafe.Pointer; see
// the note above the IOKit function types.
type (
	CFGetTypeIDFunc        func(cf uintptr) int64
	CFNumberCreateFunc     func(allocator uintptr, theType int64, valuePtr unsafe.Pointer) unsafe.Pointer
	CFNumberGetValueFunc   func(num uintptr, theType int64, valuePtr unsafe.Pointer) bool
	CFDictionaryCreateFunc func(allocator uintptr, keys, values *unsafe.Pointer, numValues int64,
		keyCallBacks, valueCallBacks uintptr) unsafe.Pointer
	CFDictionaryAddValueFunc      func(theDict, key, value uintptr)
	CFDictionaryGetValueFunc      func(theDict, key uintptr) unsafe.Pointer
	CFArrayGetCountFunc           func(theArray uintptr) int64
	CFArrayGetValueAtIndexFunc    func(theArray uintptr, index int64) unsafe.Pointer
	CFStringCreateMutableFunc     func(alloc uintptr, maxLength int64) unsafe.Pointer
	CFStringGetLengthFunc         func(theString uintptr) int64
	CFStringGetCStringFunc        func(theString uintptr, buffer CStr, bufferSize int64, encoding uint32)
	CFStringCreateWithCStringFunc func(alloc uintptr, cStr string, encoding uint32) unsafe.Pointer
	CFDataGetLengthFunc           func(theData uintptr) int64
	CFDataGetBytePtrFunc          func(theData uintptr) unsafe.Pointer
	CFReleaseFunc                 func(cf uintptr)
)

const (
	KCFStringEncodingUTF8 = 0x08000100
	KCFNumberSInt64Type   = 4
	KCFNumberIntType      = 9
	KCFAllocatorDefault   = 0
	KCFNotFound           = -1
)

// libSystem types and constants.
type MachTimeBaseInfo struct {
	Numer uint32
	Denom uint32
}

// Buffers the kernel writes into are declared as unsafe.Pointer; see the note
// above the IOKit function types. vmAddress on VMDeallocateFunc stays a uintptr
// because it names kernel-allocated memory rather than Go memory.
type (
	HostProcessorInfoFunc func(host uint32, flavor int32, outProcessorCount *uint32, outProcessorInfo unsafe.Pointer,
		outProcessorInfoCnt *uint32) int32
	HostStatisticsFunc   func(host uint32, flavor int32, hostInfoOut unsafe.Pointer, hostInfoOutCnt *uint32) int32
	MachHostSelfFunc     func() uint32
	MachTaskSelfFunc     func() uint32
	MachTimeBaseInfoFunc func(info unsafe.Pointer) int32
	VMDeallocateFunc     func(targetTask uint32, vmAddress, vmSize uintptr) int32
)

const (
	HostProcessorInfoSym = "host_processor_info"
	HostStatisticsSym    = "host_statistics"
	MachHostSelfSym      = "mach_host_self"
	MachTaskSelfSym      = "mach_task_self"
	MachTimeBaseInfoSym  = "mach_timebase_info"
	VMDeallocateSym      = "vm_deallocate"
)

const (
	HOST_VM_INFO       = 2
	HOST_CPU_LOAD_INFO = 3

	HOST_VM_INFO_COUNT = 0xf
)

type (
	ProcPidPathFunc   func(pid int32, buffer unsafe.Pointer, bufferSize uint32) int32
	ProcPidInfoFunc   func(pid, flavor int32, arg uint64, buffer unsafe.Pointer, bufferSize int32) int32
	ProcPidRusageFunc func(pid, flavor int32, buffer unsafe.Pointer) int32
	ErrnoFunc         func() *int32
)

const (
	SysctlSym        = "sysctl"
	ProcPidPathSym   = "proc_pidpath"
	ProcPidInfoSym   = "proc_pidinfo"
	ProcPidRusageSym = "proc_pid_rusage"
	ErrnoSym         = "__error"
)

const (
	MAXPATHLEN               = 1024
	PROC_PIDLISTFDS          = 1
	PROC_PIDPATHINFO_MAXSIZE = 4 * MAXPATHLEN
	PROC_PIDTASKINFO         = 4
	PROC_PIDVNODEPATHINFO    = 9

	// RUSAGE_INFO_V2 is the proc_pid_rusage flavor that includes disk I/O bytes.
	// See sys/resource.h rusage_info_v2.
	RUSAGE_INFO_V2 = 2
)

// SMC represents a SMC instance.
type SMC struct {
	lib  *IOKitLib
	conn uint32
}

const ioServiceSMC = "AppleSMC"

const (
	KSMCUserClientOpen  = 0
	KSMCUserClientClose = 1
	KSMCHandleYPCEvent  = 2
	KSMCReadKey         = 5
	KSMCWriteKey        = 6
	KSMCGetKeyCount     = 7
	KSMCGetKeyFromIndex = 8
	KSMCGetKeyInfo      = 9
)

const (
	KSMCSuccess     = 0
	KSMCError       = 1
	KSMCKeyNotFound = 132
)

func NewSMC() (*SMC, error) {
	iokit, err := NewIOKitLib()
	if err != nil {
		return nil, err
	}

	service := iokit.IOServiceGetMatchingService(0, uintptr(iokit.IOServiceMatching(ioServiceSMC)))
	if service == 0 {
		return nil, fmt.Errorf("ERROR: %s NOT FOUND", ioServiceSMC)
	}

	var conn uint32
	machTaskSelf := getFunc[MachTaskSelfFunc](iokit.library, "mach_task_self")
	if result := iokit.IOServiceOpen(service, machTaskSelf(), 0, &conn); result != 0 {
		return nil, errors.New("ERROR: IOServiceOpen failed")
	}

	iokit.IOObjectRelease(service)
	return &SMC{
		lib:  iokit,
		conn: conn,
	}, nil
}

func (s *SMC) CallStruct(selector uint32, inputStruct unsafe.Pointer, inputStructCnt uintptr,
	outputStruct unsafe.Pointer, outputStructCnt *uintptr,
) int32 {
	return s.lib.IOConnectCallStructMethod(s.conn, selector, inputStruct, inputStructCnt, outputStruct, outputStructCnt)
}

// Close releases the SMC connection. The IOKit handle itself is shared
// process-wide and is deliberately left open; see newLibrary.
func (s *SMC) Close() error {
	if result := s.lib.IOServiceClose(s.conn); result != 0 {
		return errors.New("ERROR: IOServiceClose failed")
	}
	return nil
}

type CStr []byte

func NewCStr(length int64) CStr {
	return make(CStr, length)
}

func (s CStr) Length() int64 {
	return int64(len(s))
}

func (s CStr) Ptr() *byte {
	if len(s) < 1 {
		return nil
	}

	return &s[0]
}

func (s CStr) GoString() string {
	if s == nil {
		return ""
	}

	var length int
	for _, char := range s {
		if char == '\x00' {
			break
		}
		length++
	}
	return string(s[:length])
}

// https://github.com/ebitengine/purego/blob/main/internal/strings/strings.go#L26
func GoString(cStr *byte) string {
	if cStr == nil {
		return ""
	}
	var length int
	for *(*byte)(unsafe.Add(unsafe.Pointer(cStr), uintptr(length))) != '\x00' {
		length++
	}
	return string(unsafe.Slice(cStr, length))
}

// https://github.com/apple-oss-distributions/CF/blob/dc54c6bb1c1e5e0b9486c1d26dd5bef110b20bf3/CFString.c#L463
func GetCFStringBufLengthForUTF8(length int64) int64 {
	if length > (math.MaxInt64 / 3) {
		return KCFNotFound
	}
	return length*3 + 1 // includes null terminator
}
