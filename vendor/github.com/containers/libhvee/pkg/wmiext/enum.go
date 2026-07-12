//go:build windows

package wmiext

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

type IEnumWbemClassObjectVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	Reset          uintptr
	Next           uintptr
	NextAsync      uintptr
	Clone          uintptr
	Skip           uintptr
}

type Enum struct {
	enum    *ole.IUnknown
	vTable  *IEnumWbemClassObjectVtbl
	service *Service
}

func (e *Enum) Close() {
	if e != nil && e.enum != nil {
		e.enum.Release()
	}
}

func newEnum(enumerator *ole.IUnknown, service *Service) *Enum {
	return &Enum{
		enum:    enumerator,
		vTable:  (*IEnumWbemClassObjectVtbl)(unsafe.Pointer(enumerator.RawVTable)),
		service: service,
	}
}

// NextObject obtains the next instance in an enumeration and sets all fields
// of the struct pointer passed through the target parameter. Otherwise, if
// the target parameter is not a struct pointer type, an error will be
// returned.
func NextObject(enum *Enum, target interface{}) (bool, error) {
	var err error

	var instance *Instance
	if instance, err = enum.Next(); err != nil {
		return false, err
	}

	if instance == nil {
		return true, nil
	}

	defer instance.Close()

	return false, instance.GetAll(target)
}

// Next returns the next object instance in this iteration
func (e *Enum) Next() (instance *Instance, err error) {
	var res uintptr
	var apObjects *ole.IUnknown
	var uReturned uint32

	res, _, _ = syscall.SyscallN(
		e.vTable.Next,                       // IEnumWbemClassObject::Next()
		uintptr(unsafe.Pointer(e.enum)),     // IEnumWbemClassObject   ptr
		uintptr(WBEM_INFINITE),              // [in]  long             lTimeout,
		uintptr(1),                          // [in]  ULONG            uCount,
		uintptr(unsafe.Pointer(&apObjects)), // [out] IWbemClassObject **apObjects,
		uintptr(unsafe.Pointer(&uReturned))) // [out] ULONG            *puReturned)
	if int(res) < 0 {
		return nil, NewWmiError(res)
	}

	if uReturned < 1 {
		switch res {
		case WBEM_S_NO_ERROR, WBEM_S_FALSE:
			// No more elements
			return nil, nil
		default:
			return nil, fmt.Errorf("failure advancing enumeration (%d)", res)
		}
	}

	return newInstance(apObjects, e.service), nil
}
