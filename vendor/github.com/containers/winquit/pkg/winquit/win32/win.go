//go:build windows
// +build windows

package win32

import (
	"fmt"
	"syscall"
	"unsafe"
)

type WNDCLASSEX struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	menuName      *uint16
	className     *uint16
	hIconSm       syscall.Handle
}

const (
	COLOR_WINDOW  = 0x05
	CW_USEDEFAULT = ^0x7fffffff
)

var (
	procEnumThreadWindows = user32.NewProc("EnumThreadWindows")
	procRegisterClassEx   = user32.NewProc("RegisterClassExW")
	procCreateWindowEx    = user32.NewProc("CreateWindowExW")
	procDefWinProc        = user32.NewProc("DefWindowProcW")

	callbackEnumThreadWindows = syscall.NewCallback(wndProcCloseWindow)
)

func DefWindowProc(hWnd syscall.Handle, msg uint32, wParam uintptr, lParam uintptr) int32 {

	ret, _, _ :=
		procDefWinProc.Call( // LRESULT DefWindowProcW()
			uintptr(hWnd), //          [in] HWND   hWnd,
			uintptr(msg),  //          [in] UINT   Msg,
			wParam,        //          [in] WPARAM wParam,
			lParam,        //          [in] LPARAM lParam
		)
	return int32(ret)
}

func GetModuleHandle(name string) (syscall.Handle, error) {
	var name16 *uint16
	var err error

	if len(name) > 0 {
		if name16, err = syscall.UTF16PtrFromString(name); err != nil {
			return 0, err
		}
	}

	ret, _, err :=
		procGetModuleHandle.Call( //     HMODULE GetModuleHandleW()
			uintptr(unsafe.Pointer(name16)), //  [in, optional] LPCWSTR lpModuleName
		)
	if ret == 0 {
		return 0, fmt.Errorf("could not obtain module handle: %w", err)
	}

	return syscall.Handle(ret), nil
}

func RegisterClassEx(class *WNDCLASSEX) (uint16, error) {

	ret, _, err :=
		procRegisterClassEx.Call( //      ATOM RegisterClassExW()
			uintptr(unsafe.Pointer(class)), // [in] const WNDCLASSEXW *unnamedParam1
		)
	if ret == 0 {
		return 0, fmt.Errorf("could not register window class: %w", err)
	}

	return uint16(ret), nil
}

func wndProc(hWnd syscall.Handle, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case WM_DESTROY:
		PostQuitMessage(0)
		return 0
	default:
		return uintptr(DefWindowProc(hWnd, msg, wParam, lParam))
	}
}

func CloseThreadWindows(threadId uint32) {
	_, _, _ =
		procEnumThreadWindows.Call( // // BOOL EnumThreadWindows()
			uintptr(threadId),         //      [in] DWORD       dwThreadId,
			callbackEnumThreadWindows, //      [in] WNDENUMPROC lpfn,
			0,                         //      [in] LPARAM      lParam
		)
}

func wndProcCloseWindow(hwnd uintptr, lparam uintptr) uintptr {
	SendMessage(syscall.Handle(hwnd), WM_CLOSE, 0, 0)

	return 1
}

func RegisterDummyWinClass(name string, appInstance syscall.Handle) (uint16, error) {
	var class16 *uint16
	var err error
	if class16, err = syscall.UTF16PtrFromString(name); err != nil {
		return 0, err
	}

	class := WNDCLASSEX{
		className:   class16,
		hInstance:   appInstance,
		lpfnWndProc: syscall.NewCallback(wndProc),
	}

	class.cbSize = uint32(unsafe.Sizeof(class))

	return RegisterClassEx(&class)
}

func CreateDummyWindow(name string, className string, appInstance syscall.Handle) (syscall.Handle, error) {
	var name16, class16 *uint16
	var err error

	cwDefault := CW_USEDEFAULT

	if name16, err = syscall.UTF16PtrFromString(name); err != nil {
		return 0, err
	}
	if class16, err = syscall.UTF16PtrFromString(className); err != nil {
		return 0, err
	}
	ret, _, err := procCreateWindowEx.Call( //HWND CreateWindowExW(
		0,                                //       [in]           DWORD     dwExStyle,
		uintptr(unsafe.Pointer(class16)), //       [in, optional] LPCWSTR   lpClassName,
		uintptr(unsafe.Pointer(name16)),  //       [in, optional] LPCWSTR   lpWindowName,
		0,                                //       [in]           DWORD     dwStyle,
		uintptr(cwDefault),               //       [in]           int       X,
		uintptr(cwDefault),               //       [in]           int       Y,
		0,                                //       [in]           int       nWidth,
		0,                                //       [in]           int       nHeight,
		0,                                //       [in, optional] HWND      hWndParent,
		0,                                //       [in, optional] HMENU     hMenu,
		uintptr(appInstance),             //       [in, optional] HINSTANCE hInstance,
		0,                                //       [in, optional] LPVOID    lpParam
	)

	if ret == 0 {
		return 0, fmt.Errorf("could not create window: %w", err)
	}

	return syscall.Handle(ret), nil
}
