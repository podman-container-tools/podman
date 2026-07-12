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

func newLibrary(path string) (*library, error) {
	lib, err := purego.Dlopen(path, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, err
	}

	return &library{
		handle: lib,
		fnMap:  make(map[string]any),
	}, nil
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

func (lib *library) Close() {
	purego.Dlclose(lib.handle)
}

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

func (c *CoreFoundationLib) CFNumberCreate(allocator uintptr, theType int64, valuePtr uintptr) unsafe.Pointer {
	fn := getFunc[CFNumberCreateFunc](c.library, "CFNumberCreate")
	return fn(allocator, theType, valuePtr)
}

func (c *CoreFoundationLib) CFNumberGetValue(num uintptr, theType int64, valuePtr uintptr) bool {
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

func (l *IOKitLib) IOConnectCallStructMethod(connection, selector uint32, inputStruct, inputStructCnt, outputStruct uintptr, outputStructCnt *uintptr) int32 {
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

func (s *SystemLib) HostProcessorInfo(host uint32, flavor int32, outProcessorCount *uint32, outProcessorInfo uintptr,
	outProcessorInfoCnt *uint32,
) int32 {
	fn := getFunc[HostProcessorInfoFunc](s.library, "host_processor_info")
	return fn(host, flavor, outProcessorCount, outProcessorInfo, outProcessorInfoCnt)
}

func (s *SystemLib) HostStatistics(host uint32, flavor int32, hostInfoOut uintptr, hostInfoOutCnt *uint32) int32 {
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

func (s *SystemLib) MachTimeBaseInfo(info uintptr) int32 {
	fn := getFunc[MachTimeBaseInfoFunc](s.library, "mach_timebase_info")
	return fn(info)
}

func (s *SystemLib) VMDeallocate(targetTask uint32, vmAddress, vmSize uintptr) int32 {
	fn := getFunc[VMDeallocateFunc](s.library, "vm_deallocate")
	return fn(targetTask, vmAddress, vmSize)
}

func (s *SystemLib) ProcPidPath(pid int32, buffer uintptr, bufferSize uint32) int32 {
	fn := getFunc[ProcPidPathFunc](s.library, "proc_pidpath")
	return fn(pid, buffer, bufferSize)
}

func (s *SystemLib) ProcPidInfo(pid, flavor int32, arg uint64, buffer uintptr, bufferSize int32) int32 {
	fn := getFunc[ProcPidInfoFunc](s.library, "proc_pidinfo")
	return fn(pid, flavor, arg, buffer, bufferSize)
}

// status codes
const (
	KERN_SUCCESS = 0
)

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
	IOConnectCallStructMethodFunc         func(connection, selector uint32, inputStruct, inputStructCnt, outputStruct uintptr, outputStructCnt *uintptr) int32

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
type (
	CFGetTypeIDFunc        func(cf uintptr) int64
	CFNumberCreateFunc     func(allocator uintptr, theType int64, valuePtr uintptr) unsafe.Pointer
	CFNumberGetValueFunc   func(num uintptr, theType int64, valuePtr uintptr) bool
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

type (
	HostProcessorInfoFunc func(host uint32, flavor int32, outProcessorCount *uint32, outProcessorInfo uintptr,
		outProcessorInfoCnt *uint32) int32
	HostStatisticsFunc   func(host uint32, flavor int32, hostInfoOut uintptr, hostInfoOutCnt *uint32) int32
	MachHostSelfFunc     func() uint32
	MachTaskSelfFunc     func() uint32
	MachTimeBaseInfoFunc func(info uintptr) int32
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
	ProcPidPathFunc func(pid int32, buffer uintptr, bufferSize uint32) int32
	ProcPidInfoFunc func(pid, flavor int32, arg uint64, buffer uintptr, bufferSize int32) int32
)

const (
	SysctlSym      = "sysctl"
	ProcPidPathSym = "proc_pidpath"
	ProcPidInfoSym = "proc_pidinfo"
)

const (
	MAXPATHLEN               = 1024
	PROC_PIDLISTFDS          = 1
	PROC_PIDPATHINFO_MAXSIZE = 4 * MAXPATHLEN
	PROC_PIDTASKINFO         = 4
	PROC_PIDVNODEPATHINFO    = 9
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

func (s *SMC) CallStruct(selector uint32, inputStruct, inputStructCnt, outputStruct uintptr, outputStructCnt *uintptr) int32 {
	return s.lib.IOConnectCallStructMethod(s.conn, selector, inputStruct, inputStructCnt, outputStruct, outputStructCnt)
}

func (s *SMC) Close() error {
	if result := s.lib.IOServiceClose(s.conn); result != 0 {
		return errors.New("ERROR: IOServiceClose failed")
	}
	s.lib.Close()
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

func (s CStr) Addr() uintptr {
	return uintptr(unsafe.Pointer(s.Ptr()))
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
