//go:build windows

package wmiext

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/sirupsen/logrus"
)

const (
	WmiPathKey = "__PATH"
)

var (
	WindowsEpoch = time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)
)

type Instance struct {
	object  *ole.IUnknown
	vTable  *IWbemClassObjectVtbl
	service *Service
}

type IWbemClassObjectVtbl struct {
	QueryInterface          uintptr
	AddRef                  uintptr
	Release                 uintptr
	GetQualifierSet         uintptr
	Get                     uintptr
	Put                     uintptr
	Delete                  uintptr
	GetNames                uintptr
	BeginEnumeration        uintptr
	Next                    uintptr
	EndEnumeration          uintptr
	GetPropertyQualifierSet uintptr
	Clone                   uintptr
	GetObjectText           uintptr
	SpawnDerivedClass       uintptr
	SpawnInstance           uintptr
	CompareTo               uintptr
	GetPropertyOrigin       uintptr
	InheritsFrom            uintptr
	GetMethod               uintptr
	PutMethod               uintptr
	DeleteMethod            uintptr
	BeginMethodEnumeration  uintptr
	NextMethod              uintptr
	EndMethodEnumeration    uintptr
	GetMethodQualifierSet   uintptr
	GetMethodOrigin         uintptr
}

type CIMTYPE_ENUMERATION uint32

const (
	CIM_ILLEGAL    CIMTYPE_ENUMERATION = 0xFFF
	CIM_EMPTY      CIMTYPE_ENUMERATION = 0
	CIM_SINT8      CIMTYPE_ENUMERATION = 16
	CIM_UINT8      CIMTYPE_ENUMERATION = 17
	CIM_SINT16     CIMTYPE_ENUMERATION = 2
	CIM_UINT16     CIMTYPE_ENUMERATION = 18
	CIM_SINT32     CIMTYPE_ENUMERATION = 3
	CIM_UINT32     CIMTYPE_ENUMERATION = 19
	CIM_SINT64     CIMTYPE_ENUMERATION = 20
	CIM_UINT64     CIMTYPE_ENUMERATION = 21
	CIM_REAL32     CIMTYPE_ENUMERATION = 4
	CIM_REAL64     CIMTYPE_ENUMERATION = 5
	CIM_BOOLEAN    CIMTYPE_ENUMERATION = 11
	CIM_STRING     CIMTYPE_ENUMERATION = 8
	CIM_DATETIME   CIMTYPE_ENUMERATION = 101
	CIM_REFERENCE  CIMTYPE_ENUMERATION = 102
	CIM_CHAR16     CIMTYPE_ENUMERATION = 103
	CIM_OBJECT     CIMTYPE_ENUMERATION = 13
	CIM_FLAG_ARRAY CIMTYPE_ENUMERATION = 0x2000
)

type WBEM_FLAVOR_TYPE uint32

const (
	WBEM_FLAVOR_DONT_PROPAGATE                  WBEM_FLAVOR_TYPE = 0
	WBEM_FLAVOR_FLAG_PROPAGATE_TO_INSTANCE      WBEM_FLAVOR_TYPE = 0x1
	WBEM_FLAVOR_FLAG_PROPAGATE_TO_DERIVED_CLASS WBEM_FLAVOR_TYPE = 0x2
	WBEM_FLAVOR_MASK_PROPAGATION                WBEM_FLAVOR_TYPE = 0xf
	WBEM_FLAVOR_OVERRIDABLE                     WBEM_FLAVOR_TYPE = 0
	WBEM_FLAVOR_NOT_OVERRIDABLE                 WBEM_FLAVOR_TYPE = 0x10
	WBEM_FLAVOR_MASK_PERMISSIONS                WBEM_FLAVOR_TYPE = 0x10
	WBEM_FLAVOR_ORIGIN_LOCAL                    WBEM_FLAVOR_TYPE = 0
	WBEM_FLAVOR_ORIGIN_PROPAGATED               WBEM_FLAVOR_TYPE = 0x20
	WBEM_FLAVOR_ORIGIN_SYSTEM                   WBEM_FLAVOR_TYPE = 0x40
	WBEM_FLAVOR_MASK_ORIGIN                     WBEM_FLAVOR_TYPE = 0x60
	WBEM_FLAVOR_NOT_AMENDED                     WBEM_FLAVOR_TYPE = 0
	WBEM_FLAVOR_AMENDED                         WBEM_FLAVOR_TYPE = 0x80
	WBEM_FLAVOR_MASK_AMENDED                    WBEM_FLAVOR_TYPE = 0x80
)

func newInstance(object *ole.IUnknown, service *Service) *Instance {
	instance := &Instance{
		object:  object,
		vTable:  (*IWbemClassObjectVtbl)(unsafe.Pointer(object.RawVTable)),
		service: service,
	}

	return instance
}

// Close cleans up all memory associated with this instance.
func (i *Instance) Close() {
	if i != nil && i.object != nil {
		i.object.Release()
	}
}

// GetClassName Gets the WMI class name for this WMI object instance
func (i *Instance) GetClassName() (className string, err error) {
	return i.GetAsString(`__CLASS`)
}

// Path gets the WMI object path of this instance
func (i *Instance) Path() (string, error) {
	ref, _, _, err := i.GetAsAny(WmiPathKey)
	return ref.(string), err
}

// IsReferenceProperty returns whether the property is of type CIM_REFERENCE, a string which points to
// an object path of another instance.
func (i *Instance) IsReferenceProperty(name string) (bool, error) {
	_, cimType, _, err := i.GetAsAny(name)
	return cimType == CIM_REFERENCE, err
}

// SpawnInstance create a new WMI object instance that is zero-initialized. The returned instance
// will not respect expected default values, which must be populated by other means.
func (i *Instance) SpawnInstance() (instance *Instance, err error) {
	var res uintptr
	var newUnknown *ole.IUnknown

	res, _, _ = syscall.SyscallN(
		i.vTable.SpawnInstance,               // IWbemClassObject::SpawnInstance(
		uintptr(unsafe.Pointer(i.object)),    // IWbemClassObject ptr
		uintptr(0),                           // [in]  long             lFlags,
		uintptr(unsafe.Pointer(&newUnknown))) // [out] IWbemClassObject **ppNewInstance)
	if res != 0 {
		return nil, NewWmiError(res)
	}

	return newInstance(newUnknown, i.service), nil
}

// CloneInstance create a new cloned copy of this WMI instance.
func (i *Instance) CloneInstance() (*Instance, error) {
	classObj := i.object
	vTable := (*IWbemClassObjectVtbl)(unsafe.Pointer(classObj.RawVTable))
	var cloned *ole.IUnknown

	ret, _, _ := syscall.SyscallN(
		vTable.Clone,                      // IWbemClassObject::Clone(
		uintptr(unsafe.Pointer(classObj)), // IWbemClassObject ptr
		uintptr(unsafe.Pointer(&cloned)))  // [out] IWbemClassObject **ppCopy)
	if ret != 0 {
		return nil, NewWmiError(ret)
	}

	return newInstance(cloned, i.service), nil
}

// PutAll sets all fields of this instance to the passed src parameter's fields, converting accordingly.
// The src parameter must be a pointer to a struct, otherwise an error will be returned.
func (i *Instance) PutAll(src interface{}) error {
	val := reflect.ValueOf(src)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return errors.New("not a struct or pointer to struct")
	}

	props, err := i.GetAllProperties()
	if err != nil {
		return err
	}

	return i.instancePutAllTraverse(val, props)
}

func (i *Instance) instancePutAllTraverse(val reflect.Value, propMap map[string]interface{}) error {
	for j := 0; j < val.NumField(); j++ {
		fieldVal := val.Field(j)
		fieldType := val.Type().Field(j)

		if fieldType.Type.Kind() == reflect.Struct && fieldType.Anonymous {
			if err := i.instancePutAllTraverse(fieldVal, propMap); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(fieldType.Name, "S__") {
			continue
		}

		if !fieldType.IsExported() {
			continue
		}

		if _, exists := propMap[fieldType.Name]; !exists {
			continue
		}

		if fieldVal.Kind() == reflect.String && fieldVal.Len() == 0 {
			continue
		}

		if err := i.Put(fieldType.Name, fieldVal.Interface()); err != nil {
			return err
		}
	}

	return nil
}

// Put sets the specified property to the passed Golang value, converting appropriately.
func (i *Instance) Put(name string, value interface{}) (err error) {
	var variant ole.VARIANT

	switch cast := value.(type) {
	case ole.VARIANT:
		variant = cast
	case *ole.VARIANT:
		variant = *cast
	default:
		variant, err = NewAutomationVariant(value)
		if err != nil {
			return err
		}
	}

	var wszName *uint16
	if wszName, err = syscall.UTF16PtrFromString(name); err != nil {
		return
	}

	classObj := i.object
	vTable := (*IWbemClassObjectVtbl)(unsafe.Pointer(classObj.RawVTable))
	res, _, _ := syscall.SyscallN(
		vTable.Put,                        // IWbemClassObject::Put(
		uintptr(unsafe.Pointer(classObj)), // IWbemClassObject ptr
		uintptr(unsafe.Pointer(wszName)),  // [in] LPCWSTR wszName,
		uintptr(0),                        // [in] long    lFlags,
		uintptr(unsafe.Pointer(&variant)), // [in] VARIANT *pVal,
		uintptr(0))                        // [in] CIMTYPE Type)
	if res != 0 {
		return NewWmiError(res)
	}

	_ = variant.Clear()
	return
}

// GetCimText returns the CIM XML representation of this instance. Some WMI methods use a string
// parameter to represent a full complex object, and this method is used to generate
// the expected format.
func (i *Instance) GetCimText() string {
	type wmiWbemTxtSrcVtable struct {
		QueryInterface uintptr
		AddRef         uintptr
		Release        uintptr
		GetTxt         uintptr
	}
	const CIM_XML_FORMAT = 1

	classObj := i.object

	vTable := (*wmiWbemTxtSrcVtable)(unsafe.Pointer(wmiWbemTxtLocator.RawVTable))
	var retString *uint16
	res, _, _ := syscall.SyscallN(
		vTable.GetTxt,                           // IWbemObjectTextSrc::GetText()
		uintptr(unsafe.Pointer(wmiWbemLocator)), // IWbemObjectTextSrc ptr
		uintptr(0),                              // [in]  long             lFlags
		uintptr(unsafe.Pointer(classObj)),       // [in]  IWbemClassObject *pObj
		uintptr(CIM_XML_FORMAT),                 // [in]  ULONG            uObjTextFormat,
		uintptr(0),                              // [in]  IWbemContext     *pCtx,
		uintptr(unsafe.Pointer(&retString)))     // [out] BSTR             *strText)
	if res != 0 {
		return ""
	}
	itemStr := ole.BstrToString(retString)
	return itemStr
}

// GetAll gets all fields that map to a target struct and populates all struct fields according to
// the expected type information. The target parameter should be a pointer to a struct, and
// will return an error otherwise.
func (i *Instance) GetAll(target interface{}) error {
	elem := reflect.ValueOf(target)
	if elem.Kind() != reflect.Ptr || elem.IsNil() {
		return errors.New("invalid destination type for mapping a WMI instance to an object")
	}

	// deref pointer
	elem = elem.Elem()
	var err error

	if err = i.BeginEnumeration(); err != nil {
		return err
	}

	properties := make(map[string]*ole.VARIANT)

	for {
		var name string
		var value *ole.VARIANT
		var done bool

		if done, name, value, _, _, err = i.NextAsVariant(); err != nil {
			return err
		}

		if done {
			break
		}

		if value != nil {
			properties[name] = value
		}
	}

	defer func() {
		for _, v := range properties {
			_ = v.Clear()
		}
	}()

	_ = i.EndEnumeration()

	return i.instanceGetAllPopulate(elem, elem.Type(), properties)
}

// GetAsAny gets a property and converts it to a Golang type that matches the internal
// variant automation type passed back from WMI. For usage with predictable static
// type mapping, use GetAsString(), GetAsUint(), or GetAll() instead of this method.
func (i *Instance) GetAsAny(name string) (interface{}, CIMTYPE_ENUMERATION, WBEM_FLAVOR_TYPE, error) {
	variant, cimType, flavor, err := i.GetAsVariant(name)
	if err != nil {
		return nil, cimType, flavor, err
	}

	defer func() {
		if err := variant.Clear(); err != nil {
			logrus.Error(err)
		}
	}()

	// Since there is no type information only perform the stock conversion
	result := convertToGenericValue(variant)

	return result, cimType, flavor, err
}

// GetAsString gets a property value as a string value, converting if necessary
func (i *Instance) GetAsString(name string) (value string, err error) {
	variant, _, _, err := i.GetAsVariant(name)
	if err != nil || variant == nil {
		return "", err
	}
	defer func() {
		if err := variant.Clear(); err != nil {
			logrus.Error(err)
		}
	}()

	// TODO: replace with something better
	return fmt.Sprintf("%v", convertToGenericValue(variant)), nil
}

// GetAsUint gets a property value as a uint value, if conversion is possible. Otherwise,
// returns an error.
func (i *Instance) GetAsUint(name string) (uint, error) {
	val, _, _, err := i.GetAsAny(name)
	if err != nil {
		return 0, err
	}

	switch ret := val.(type) {
	case int:
		return uint(ret), nil
	case int8:
		return uint(ret), nil
	case int16:
		return uint(ret), nil
	case int32:
		return uint(ret), nil
	case int64:
		return uint(ret), nil
	case uint:
		return ret, nil
	case uint8:
		return uint(ret), nil
	case uint16:
		return uint(ret), nil
	case uint32:
		return uint(ret), nil
	case uint64:
		return uint(ret), nil
	case string:
		parse, err := strconv.ParseUint(ret, 10, 64)
		return uint(parse), err
	default:
		return 0, fmt.Errorf("type conversion from %T on param %s not supported", val, name)
	}
}

// GetAsVariant obtains a specified property value, if it exists.
func (i *Instance) GetAsVariant(name string) (*ole.VARIANT, CIMTYPE_ENUMERATION, WBEM_FLAVOR_TYPE, error) {
	var variant ole.VARIANT
	var err error
	var wszName *uint16
	var cimType CIMTYPE_ENUMERATION
	var flavor WBEM_FLAVOR_TYPE

	if wszName, err = syscall.UTF16PtrFromString(name); err != nil {
		return nil, 0, 0, err
	}

	classObj := i.object
	vTable := (*IWbemClassObjectVtbl)(unsafe.Pointer(classObj.RawVTable))

	res, _, _ := syscall.SyscallN(
		vTable.Get,                        // IWbemClassObject::Get(
		uintptr(unsafe.Pointer(classObj)), // IWbemClassObject ptr
		uintptr(unsafe.Pointer(wszName)),  // [in]            LPCWSTR wszName,
		uintptr(0),                        // [in]            long    lFlags,
		uintptr(unsafe.Pointer(&variant)), // [out]           VARIANT *pVal,
		uintptr(unsafe.Pointer(&cimType)), // [out, optional] CIMTYPE *pType,
		uintptr(unsafe.Pointer(&flavor)))  // [out, optional] long    *plFlavor)
	if res != 0 {
		return nil, 0, 0, NewWmiError(res)
	}

	return &variant, cimType, flavor, nil
}

// Next retrieves the next property as a Golang type when iterating the properties using an enumerator
// created by BeginEnumeration(). The returned value's type represents the internal automation type
// used by WMI. It is usually preferred to use GetAsXXX(), GetAll(), or GetAll Properties() over this
// method.
func (i *Instance) Next() (done bool, name string, value interface{}, cimType CIMTYPE_ENUMERATION, flavor WBEM_FLAVOR_TYPE, err error) {
	var variant *ole.VARIANT
	done, name, variant, cimType, flavor, err = i.NextAsVariant()

	if err == nil && !done {
		defer func() {
			if err := variant.Clear(); err != nil {
				logrus.Error(err)
			}
		}()
		value = convertToGenericValue(variant)
	}

	return
}

// NextAsVariant retrieves the next property as a VARIANT type when iterating the properties using an enumerator
// created by BeginEnumeration(). The returned value's type represents the internal automation type
// used by WMI. It is usually preferred to use GetAsXXX(), GetAll(), or GetAllProperties() over this
// method. Callers are responsible for clearing the VARIANT, otherwise associated memory will leak.
func (i *Instance) NextAsVariant() (bool, string, *ole.VARIANT, CIMTYPE_ENUMERATION, WBEM_FLAVOR_TYPE, error) {
	var res uintptr
	var strName *uint16
	var variant ole.VARIANT
	var cimType CIMTYPE_ENUMERATION
	var flavor WBEM_FLAVOR_TYPE

	res, _, _ = syscall.SyscallN(
		i.vTable.Next,                     // IWbemClassObject::Next(
		uintptr(unsafe.Pointer(i.object)), // IWbemClassObject ptr
		uintptr(0),                        // [in]            long    lFlags,
		uintptr(unsafe.Pointer(&strName)), // [out]           BSTR    *strName,
		uintptr(unsafe.Pointer(&variant)), // [out]           VARIANT *pVal,
		uintptr(unsafe.Pointer(&cimType)), // [out, optional] CIMTYPE *pType,
		uintptr(unsafe.Pointer(&flavor)))  // [out, optional] long    *plFlavor
	if int(res) < 0 {
		return false, "", nil, cimType, flavor, NewWmiError(res)
	}

	if res == WBEM_S_NO_MORE_DATA {
		return true, "", nil, cimType, flavor, nil
	}

	defer ole.SysFreeString((*int16)(unsafe.Pointer(strName))) //nolint:errcheck
	name := ole.BstrToString(strName)

	return false, name, &variant, cimType, flavor, nil
}

// GetAllProperties gets all properties on this instance. The returned map is keyed by the field name and the value
// is a Golang type which matches the WMI internal implementation. For static type conversions,
// it's recommended to use either GetAll(), which uses struct fields for type information, or
// the GetAsXXX() methods.
func (i *Instance) GetAllProperties() (map[string]interface{}, error) {
	var err error
	properties := make(map[string]interface{})

	if err = i.BeginEnumeration(); err != nil {
		return nil, err
	}

	defer func() {
		if err := i.EndEnumeration(); err != nil {
			logrus.Error(err)
		}
	}()

	for {
		var name string
		var value interface{}
		var done bool

		if done, name, value, _, _, err = i.Next(); err != nil || done {
			return properties, err
		}

		properties[name] = value
	}
}

// GetMethodParameters returns a WMI class object which represents the [in] method parameters for a method invocation.
// This is an advanced method, used for dynamic introspection or manual method invocation. In most
// cases it is recommended to use BeginInvoke() instead, which constructs the parameter payload
// automatically.
func (i *Instance) GetMethodParameters(method string) (*Instance, error) {
	var err error
	var res uintptr
	var inSignature *ole.IUnknown

	var wszName *uint16
	if wszName, err = syscall.UTF16PtrFromString(method); err != nil {
		return nil, err
	}

	res, _, _ = syscall.SyscallN(
		i.vTable.GetMethod,                    // IWbemClassObject::GetMethod(
		uintptr(unsafe.Pointer(i.object)),     // IWbemClassObject ptr
		uintptr(unsafe.Pointer(wszName)),      // [in]  LPCWSTR          wszName
		uintptr(0),                            // [in]  long             lFlags,
		uintptr(unsafe.Pointer(&inSignature)), // [out] IWbemClassObject **ppInSignature,
		uintptr(0))                            // [out] IWbemClassObject **ppOutSignature)
	if res != 0 {
		return nil, NewWmiError(res)
	}

	return newInstance(inSignature, i.service), nil
}

func (i *Instance) instanceGetAllPopulate(elem reflect.Value, elemType reflect.Type, properties map[string]*ole.VARIANT) error {
	var err error

	for j := 0; j < elemType.NumField(); j++ {
		fieldType := elemType.Field(j)
		fieldVal := elem.Field(j)

		if !fieldType.IsExported() {
			continue
		}

		if fieldType.Type.Kind() == reflect.Struct && fieldType.Anonymous {
			if err := i.instanceGetAllPopulate(fieldVal, fieldType.Type, properties); err != nil {
				return err
			}
			continue
		}

		fieldName := fieldType.Name

		if strings.HasPrefix(fieldName, "S__") {
			fieldName = fieldName[1:]
		}
		if variant, ok := properties[fieldName]; ok {
			var val interface{}
			if val, err = convertToGoType(variant, fieldVal, fieldType.Type); err != nil {
				return err
			}

			if val != nil {
				fieldVal.Set(reflect.ValueOf(val))
			}
		}
	}

	return nil
}

// BeginEnumeration begins iterating the property list on this instance. This is an advanced method.
// In most cases, the GetAsXXX() methods, GetAll(), and GetAllProperties() methods should be
// preferred.
func (i *Instance) BeginEnumeration() error {
	classObj := i.object
	vTable := (*IWbemClassObjectVtbl)(unsafe.Pointer(classObj.RawVTable))

	result, _, _ := syscall.SyscallN(
		vTable.BeginEnumeration,           // IWbemClassObject::BeginEnumeration(
		uintptr(unsafe.Pointer(classObj)), // IWbemClassObject ptr,
		uintptr(0))                        // [in] long lEnumFlags) // 0 = defaults
	if result != 0 {
		return NewWmiError(result)
	}

	return nil
}

// EndEnumeration completes iterating a property list on this instance. This is an advanced method.
// In most cases, the GetAsXXX() methods, GetAll(), and GetAllProperties() methods
// should be preferred.
func (i *Instance) EndEnumeration() error {
	res, _, _ := syscall.SyscallN(
		i.vTable.EndEnumeration,           // IWbemClassObject::EndEnumeration(
		uintptr(unsafe.Pointer(i.object))) // IWbemClassObject ptr)
	if res != 0 {
		return NewWmiError(res)
	}

	return nil
}

// BeginInvoke invokes a method on this Instance. Returns a MethodExecutor builder object
// that is used to construct the input parameters (via calls to In()), perform the
// invocation (using calls to Execute()), retrieve output parameters (via calls to
// Out()), and finally the method return value (using a call to End())
func (i *Instance) BeginInvoke(method string) *MethodExecutor {
	objPath, err := i.Path()
	if err != nil {
		return &MethodExecutor{err: err}
	}

	var class, inParam *Instance
	if class, err = i.service.GetClassInstance(i); err == nil {
		inParam, err = class.GetMethodParameters(method)
		class.Close()
	}

	return &MethodExecutor{method: method, path: objPath, service: i.service, inParam: inParam, err: err}
}
