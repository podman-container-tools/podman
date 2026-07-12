//go:build windows

package wmiext

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

type IWbemLocatorVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	ConnectServer  uintptr
}

type Service struct {
	service *ole.IUnknown
	vTable  *IWbemServicesVtbl
}

type IWbemServicesVtbl struct {
	QueryInterface             uintptr
	AddRef                     uintptr
	Release                    uintptr
	OpenNamespace              uintptr
	CancelAsyncCall            uintptr
	QueryObjectSink            uintptr
	GetObject                  uintptr
	GetObjectAsync             uintptr
	PutClass                   uintptr
	PutClassAsync              uintptr
	DeleteClass                uintptr
	DeleteClassAsync           uintptr
	CreateClassEnum            uintptr
	CreateClassEnumAsync       uintptr
	PutInstance                uintptr
	PutInstanceAsync           uintptr
	DeleteInstance             uintptr
	DeleteInstanceAsync        uintptr
	CreateInstanceEnum         uintptr
	CreateInstanceEnumAsync    uintptr
	ExecQuery                  uintptr
	ExecQueryAsync             uintptr
	ExecNotificationQuery      uintptr
	ExecNotificationQueryAsync uintptr
	ExecMethod                 uintptr
	ExecMethodAsync            uintptr
}

func connectService(namespace string) (*Service, error) {

	if wmiWbemLocator == nil {
		return nil, errors.New("WMI failed initialization, service calls can not proceed")
	}

	var err error
	var res uintptr
	var strResource *uint16
	var strLocale *uint16
	var service *ole.IUnknown

	loc := fmt.Sprintf(`\\.\%s`, namespace)

	if strResource, err = syscall.UTF16PtrFromString(loc); err != nil {
		return nil, err
	}

	// Connect with en_US LCID since we do pattern matching against English key values
	if strLocale, err = syscall.UTF16PtrFromString("MS_409"); err != nil {
		return nil, err
	}

	myVTable := (*IWbemLocatorVtbl)(unsafe.Pointer(wmiWbemLocator.RawVTable))
	res, _, _ = syscall.SyscallN(
		myVTable.ConnectServer,                  // IWbemLocator::ConnectServer(
		uintptr(unsafe.Pointer(wmiWbemLocator)), // IWbemLocator ptr
		uintptr(unsafe.Pointer(strResource)),    // [in]  const BSTR    strNetworkResource,
		uintptr(0),                              // [in]  const BSTR    strUser,
		uintptr(0),                              // [in]  const BSTR    strPassword,
		uintptr(unsafe.Pointer(strLocale)),      // [in]  const BSTR    strLocale,
		uintptr(WBEM_FLAG_CONNECT_USE_MAX_WAIT), // [in]  long          lSecurityFlags,
		uintptr(0),                              // [in]  const BSTR    strAuthority,
		uintptr(0),                              // [in]  IWbemContext  *pCtx,
		uintptr(unsafe.Pointer(&service)))       // [out] IWbemServices **ppNamespace)

	if res != 0 {
		return nil, NewWmiError(res)
	}

	if err = CoSetProxyBlanket(service); err != nil {
		return nil, err
	}

	return newService(service), nil
}

func newService(service *ole.IUnknown) *Service {
	return &Service{
		service: service,
		vTable:  (*IWbemServicesVtbl)(unsafe.Pointer(service.RawVTable)),
	}
}

const (
	WBEM_FLAG_CONNECT_USE_MAX_WAIT = 0x80
)

func CoSetProxyBlanket(service *ole.IUnknown) (err error) {
	res, _, _ := procCoSetProxyBlanket.Call( // CoSetProxyBlanket(
		uintptr(unsafe.Pointer(service)),     // [in]      IUnknown                 *pProxy,
		uintptr(RPC_C_AUTHN_WINNT),           // [in]      DWORD                    dwAuthnSvc,
		uintptr(RPC_C_AUTHZ_NONE),            // [in]      DWORD                    dwAuthzSvc,
		uintptr(0),                           // [in, opt] OLECHAR                  *pServerPrincName,
		uintptr(RPC_C_AUTHN_LEVEL_CALL),      // [in]      DWORD                    dwAuthnLevel,
		uintptr(RPC_C_IMP_LEVEL_IMPERSONATE), // [in]      DWORD                    dwImpLevel,
		uintptr(0),                           // [in, opt] RPC_AUTH_IDENTITY_HANDLE pAuthInfo,
		uintptr(EOAC_NONE))                   // [in]      DWORD                    dwCapabilities)

	if res != 0 {
		return NewWmiError(res)
	}

	return nil
}

// NewLocalService creates a service and connect it to the local system at the specified namespace
func NewLocalService(namespace string) (s *Service, err error) {
	return connectService(namespace)
}

// Close frees all associated memory with this service
func (s *Service) Close() {
	if s != nil && s.service != nil {
		s.service.Release()
	}
}

// ExecQuery executes a WQL query and returns an enumeration to iterate the result set.
// Queries are executed in a semi-synchronous fashion.
func (s *Service) ExecQuery(wqlQuery string) (*Enum, error) {
	var err error
	var pEnum *ole.IUnknown
	var strQuery *uint16
	var strQL *uint16

	if strQL, err = syscall.UTF16PtrFromString("WQL"); err != nil {
		return nil, err
	}

	if strQuery, err = syscall.UTF16PtrFromString(wqlQuery); err != nil {
		return nil, err
	}

	// Semisynchronous mode = return immed + forward (for perf)
	flags := WBEM_FLAG_FORWARD_ONLY | WBEM_FLAG_RETURN_IMMEDIATELY

	hres, _, _ := syscall.SyscallN(
		s.vTable.ExecQuery,                 // IWbemServices::ExecQuery(
		uintptr(unsafe.Pointer(s.service)), // IWbemServices ptr
		uintptr(unsafe.Pointer(strQL)),     // [in] const BSTR           strQueryLanguage,
		uintptr(unsafe.Pointer(strQuery)),  // [in] const BSTR           strQuery,
		uintptr(flags),                     // [in] long                 lFlags,
		uintptr(0),                         // [in] IWbemContext         *pCtx,
		uintptr(unsafe.Pointer(&pEnum)))    // [out] IEnumWbemClassObject **ppEnum)
	if hres != 0 {
		return nil, NewWmiError(hres)
	}

	if err = CoSetProxyBlanket(pEnum); err != nil {
		return nil, err
	}

	return newEnum(pEnum, s), nil
}

// GetObject obtains a single WMI class or instance given its path
func (s *Service) GetObject(objectPath string) (instance *Instance, err error) {
	var pObject *ole.IUnknown
	var strObjectPath *uint16

	if strObjectPath, err = syscall.UTF16PtrFromString(objectPath); err != nil {
		return
	}

	// Synchronous call
	flags := WBEM_FLAG_RETURN_WBEM_COMPLETE

	res, _, _ := syscall.SyscallN(
		s.vTable.GetObject,                     // IWbemServices::GetObject(
		uintptr(unsafe.Pointer(s.service)),     // IWbemServices ptr
		uintptr(unsafe.Pointer(strObjectPath)), // [in]  const BSTR       strObjectPath,
		uintptr(flags),                         // [in]  long             lFlags,
		uintptr(0),                             // [in]  IWbemContext     *pCtx,
		uintptr(unsafe.Pointer(&pObject)),      // [out] IWbemClassObject **ppObject,
		uintptr(0))                             // [out] IWbemCallResult  **ppCallResult)
	if int(res) < 0 {
		// returns WBEM_E_PROVIDER_NOT_FOUND when no entry found
		return nil, NewWmiError(res)
	}

	return newInstance(pObject, s), nil
}

// GetObjectAsObject gets an object by its path and set all fields of the passed in target to match the instance's
// properties. Conversion is performed as appropriate.
func (s *Service) GetObjectAsObject(objPath string, target interface{}) error {
	instance, err := s.GetObject(objPath)
	if err != nil {
		return err
	}
	defer instance.Close()

	return instance.GetAll(target)
}

// CreateInstanceEnum creates an enumerator that iterates all registered object instances for a given className.
func (s *Service) CreateInstanceEnum(className string) (*Enum, error) {
	var err error
	var pEnum *ole.IUnknown
	var strFilter *uint16

	if strFilter, err = syscall.UTF16PtrFromString(className); err != nil {
		return nil, err
	}

	// No subclasses in result set
	flags := WBEM_FLAG_SHALLOW

	res, _, _ := syscall.SyscallN(
		s.vTable.CreateInstanceEnum,        // IWbemServices::CreateInstanceEnum(
		uintptr(unsafe.Pointer(s.service)), // IWbemServices ptr
		uintptr(unsafe.Pointer(strFilter)), // [in]  const BSTR           strFilter,
		uintptr(flags),                     // [in]  long                 lFlags,
		uintptr(0),                         // [in]  IWbemContext         *pCtx,
		uintptr(unsafe.Pointer(&pEnum)))    // [out] IEnumWbemClassObject **ppEnum)
	if int(res) < 0 {
		return nil, NewWmiError(res)
	}

	if err = CoSetProxyBlanket(pEnum); err != nil {
		return nil, err
	}

	return newEnum(pEnum, s), nil
}

// ExecMethod executes a method using the specified class and parameter payload instance. The parameter payload
// instance can be constructed using Instance.GetMethodParameters(). This is an advanced method, it is
// recommended to use BeginInvoke() instead, where possible.
func (s *Service) ExecMethod(className string, methodName string, inParams *Instance) (*Instance, error) {
	var err error
	var outParams *ole.IUnknown
	var strObjectPath *uint16
	var strMethodName *uint16

	if strObjectPath, err = syscall.UTF16PtrFromString(className); err != nil {
		return nil, err
	}

	if strMethodName, err = syscall.UTF16PtrFromString(methodName); err != nil {
		return nil, err
	}

	res, _, _ := syscall.SyscallN(
		s.vTable.ExecMethod,                      // IWbemServices::ExecMethod(
		uintptr(unsafe.Pointer(s.service)),       // IWbemServices ptr
		uintptr(unsafe.Pointer(strObjectPath)),   // [in]  const BSTR       strObjectPath,
		uintptr(unsafe.Pointer(strMethodName)),   // [in]  const BSTR       strMethodName,
		uintptr(0),                               // [in]  long             lFlags,
		uintptr(0),                               // [in]  IWbemContext     *pCtx,
		uintptr(unsafe.Pointer(inParams.object)), // [in]  IWbemClassObject *pInParams,
		uintptr(unsafe.Pointer(&outParams)),      // [out] IWbemClassObject **ppOutParams,
		uintptr(0))                               // [out] IWbemCallResult  **ppCallResult)
	if int(res) < 0 {
		return nil, NewWmiError(res)
	}

	return newInstance(outParams, s), nil
}

// FindFirstInstance find and returns the first WMI Instance in the result set for a WSL query.
func (s *Service) FindFirstInstance(wql string) (*Instance, error) {
	var enum *Enum
	var err error
	if enum, err = s.ExecQuery(wql); err != nil {
		return nil, err
	}
	defer enum.Close()

	instance, err := enum.Next()
	if err != nil {
		return nil, err
	}

	if instance == nil {
		return nil, ErrNoResults
	}

	return instance, nil
}

// FindFirstRelatedInstance finds and returns a related associator of the specified WMI object path of the
// expected className type.
func (s *Service) FindFirstRelatedInstance(objPath string, className string) (*Instance, error) {
	wql := fmt.Sprintf("ASSOCIATORS OF {%s} WHERE ResultClass = %s", objPath, className)
	return s.FindFirstInstance(wql)
}

// FindFirstRelatedInstanceThrough finds and returns a related associator of the specified WMI object path of the
// expected className type, and only through the expected association type.
func (s *Service) FindFirstRelatedInstanceThrough(objPath string, resultClass string, assocClass string) (*Instance, error) {
	wql := fmt.Sprintf("ASSOCIATORS OF {%s} WHERE AssocClass = %s ResultClass = %s ", objPath, assocClass, resultClass)
	return s.FindFirstInstance(wql)
}

// FindFirstRelatedObject finds and returns a related associator of the specified WMI object path of the
// expected className type, and populates the passed in struct with its fields
func (s *Service) FindFirstRelatedObject(objPath string, className string, target interface{}) error {
	wql := fmt.Sprintf("ASSOCIATORS OF {%s} WHERE ResultClass = %s", objPath, className)
	return s.FindFirstObject(wql, target)
}

// FindFirstObject finds and returns the first WMI Instance in the result set for a WSL query, and
// populates the specified struct pointer passed in through the target parameter.
func (s *Service) FindFirstObject(wql string, target interface{}) error {
	var enum *Enum
	var err error
	if enum, err = s.ExecQuery(wql); err != nil {
		return err
	}
	defer enum.Close()

	done, err := NextObject(enum, target)
	if err != nil {
		return err
	}

	if done {
		return errors.New("no results found")
	}

	return nil
}

// GetSingletonInstance gets the first WMI instance of the specified object class type. This is a
// shortcut method for uses where only one instance is expected.
func (s *Service) GetSingletonInstance(className string) (*Instance, error) {
	var (
		enum     *Enum
		instance *Instance
		err      error
	)

	if enum, err = s.CreateInstanceEnum(className); err != nil {
		return nil, err
	}
	defer enum.Close()

	if instance, err = enum.Next(); err != nil {
		return nil, err
	}

	return instance, nil
}

// CreateInstance creates a new WMI object class instance of the specified className, and sets
// all properties according to the passed in struct pointer through the src
// parameter, converting appropriately.
func (s *Service) CreateInstance(className string, src interface{}) (*Instance, error) {
	instance, err := s.SpawnInstance(className)
	if err != nil {
		return nil, err
	}

	return instance, instance.PutAll(src)
}

// SpawnInstance creates a new zeroed WMI instance. This instance will not contain expected values.
// Those must be retrieved and set separately, or CreateInstance() can be used instead.
func (s *Service) SpawnInstance(className string) (*Instance, error) {
	var class *Instance
	var err error
	if class, err = s.GetObject(className); err != nil {
		return nil, err
	}
	defer class.Close()

	return class.SpawnInstance()
}

// RefetchObject re-fetches the object and returns a new instance. The original instance will not
// automatically Close(). Callers of this method will need to manually close the
// original.
func (s *Service) RefetchObject(instance *Instance) (*Instance, error) {
	path, err := instance.Path()
	if err != nil {
		return instance, err
	}
	return s.GetObject(path)
}

// GetClassInstance gets the WMI class instance associated with the specified object instance.
// This method is used to perform schema queries.
func (s *Service) GetClassInstance(obj *Instance) (*Instance, error) {
	name, err := obj.GetClassName()
	if err != nil {
		return nil, err
	}
	return s.GetObject(name)
}
