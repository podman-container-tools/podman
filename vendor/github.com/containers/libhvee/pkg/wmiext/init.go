//go:build windows

package wmiext

import (
	"github.com/go-ole/go-ole"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

var (
	ole32                    = windows.NewLazySystemDLL("ole32.dll")
	procCoSetProxyBlanket    = ole32.NewProc("CoSetProxyBlanket")
	procCoInitializeSecurity = ole32.NewProc("CoInitializeSecurity")

	modoleaut32               = windows.NewLazySystemDLL("oleaut32.dll")
	procSafeArrayCreateVector = modoleaut32.NewProc("SafeArrayCreateVector")
	procSafeArrayPutElement   = modoleaut32.NewProc("SafeArrayPutElement")
	procSafeArrayGetElement   = modoleaut32.NewProc("SafeArrayGetElement")
	procSafeArrayDestroy      = modoleaut32.NewProc("SafeArrayDestroy")

	clsidWbemObjectTextSrc = ole.NewGUID("{8d1c559d-84f0-4bb3-a7d5-56a7435a9ba6}")
	iidIWbemObjectTextSrc  = ole.NewGUID("{bfbf883a-cad7-11d3-a11b-00105a1f515a}")

	wmiWbemTxtLocator *ole.IUnknown
	wmiWbemLocator    *ole.IUnknown

	clsidWbemLocator = ole.NewGUID("4590f811-1d3a-11d0-891f-00aa004b2e24")
	iidIWbemLocator  = ole.NewGUID("dc12a687-737f-11cf-884d-00aa004b2e24")
)

const (
	// WMI Generic flags
	WBEM_FLAG_RETURN_WBEM_COMPLETE = 0x0
	WBEM_FLAG_RETURN_IMMEDIATELY   = 0x10
	WBEM_FLAG_FORWARD_ONLY         = 0x20

	// WMI Query flags
	WBEM_FLAG_SHALLOW = 1

	// Timeout flags
	WBEM_NO_WAIT  = 0
	WBEM_INFINITE = 0xFFFFFFFF

	// COM Auth Flags
	EOAC_NONE = 0

	// RPC Authentication
	RPC_C_AUTHN_WINNT = 10

	// RPC Authentication Level
	RPC_C_AUTHN_LEVEL_DEFAULT = 0
	RPC_C_AUTHN_LEVEL_CALL    = 3

	// RPC Authorization
	RPC_C_AUTHZ_NONE = 0

	// RPC Impersonation
	RPC_C_IMP_LEVEL_IMPERSONATE = 3
)

func init() {
	var err error

	err = ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		if oleCode, ok := err.(*ole.OleError); ok {
			code := oleCode.Code()
			// 1 = Already init
			if code != 0 && code != 1 {
				logrus.Errorf("Unable to initialize COM: %s", err.Error())
				return
			}
		}
	}

	initSecurity()

	wmiWbemLocator, err = ole.CreateInstance(clsidWbemLocator, iidIWbemLocator)
	if err != nil {
		logrus.Errorf("Could not initialize Wbem components, WMI operations will likely fail %s", err.Error())
	}

	// IID_IWbemObjectTextSrc Obtain the initial locator to WMI
	wmiWbemTxtLocator, err = ole.CreateInstance(clsidWbemObjectTextSrc, iidIWbemObjectTextSrc)
	if err != nil {
		logrus.Errorf("Could not initialize Wbem components, WMI operations will likely fail %s", err.Error())
	}
}

func initSecurity() {
	var svc int32 = -1

	res, _, _ := procCoInitializeSecurity.Call( // CoInitializeSecurity
		uintptr(0),                           // [in, optional] PSECURITY_DESCRIPTOR        pSecDesc,
		uintptr(svc),                         // [in]           LONG                        cAuthSvc,
		uintptr(0),                           // [in, optional] SOLE_AUTHENTICATION_SERVICE *asAuthSvc,
		uintptr(0),                           // [in, optional] void                        *pReserved1,
		uintptr(RPC_C_AUTHN_LEVEL_DEFAULT),   // [in]           DWORD                       dwAuthnLevel,
		uintptr(RPC_C_IMP_LEVEL_IMPERSONATE), // [in]           DWORD                       dwImpLevel,
		uintptr(0),                           // [in, optional] void                        *pAuthList,
		uintptr(EOAC_NONE),                   // [in]           DWORD                       dwCapabilities,
		uintptr(0))                           // [in, optional] void                        *pReserved3
	if int(res) < 0 {
		logrus.Errorf("Unable to initialize COM security: %s", NewWmiError(res).Error())
	}
}
