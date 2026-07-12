//go:build windows
// +build windows

package win32

import (
	"syscall"
	"unsafe"
)

type MSG struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

const (
	WM_QUIT    = 0x12
	WM_DESTROY = 0x02
	WM_CLOSE   = 0x10
)

var (
	postQuitMessage      = user32.NewProc("PostQuitMessage")
	procGetMessage       = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessage  = user32.NewProc("DispatchMessageW")
	procSendMessage      = user32.NewProc("SendMessageW")
)

func TranslateMessage(msg *MSG) bool {
	ret, _, _ :=
		procTranslateMessage.Call( //   BOOL TranslateMessage()
			uintptr(unsafe.Pointer(msg)), // [in] const MSG *lpMsg
		)

	return ret != 0

}

func DispatchMessage(msg *MSG) uintptr {
	ret, _, _ :=
		procDispatchMessage.Call( //         LRESULT DispatchMessage()
			uintptr(unsafe.Pointer(msg)), //          [in] const MSG *lpMsg
		)

	return ret
}

func SendMessage(handle syscall.Handle, message uint, wparm uintptr, lparam uintptr) uintptr {
	ret, _, _ :=
		procSendMessage.Call( // LRESULT SendMessage()
			uintptr(handle),  //         [in] HWND   hWnd
			uintptr(message), //         [in] UINT   Msg
			wparm,            //         [in] WPARAM wParam
			lparam,           //         [in] LPARAM lParam
		)

	return ret
}

func PostQuitMessage(code int) {
	_, _, _ =
		postQuitMessage.Call( // void PostQuitMessage()
			uintptr(code), //         [in] int nExitCode
		)
}

func GetMessage(handle syscall.Handle, int, max int) (int32, *MSG, error) {
	var msg MSG
	ret, _, err :=
		procGetMessage.Call( //            // BOOL GetMessage()
			uintptr(unsafe.Pointer(&msg)), //      [out]          LPMSG lpMsg,
			uintptr(handle),               //      [in, optional] HWND  hWnd,
			0,                             //      [in]           UINT  wMsgFilterMin,
			0,                             //      [in]           UINT  wMsgFilterMax
		)

	if int32(ret) == -1 {
		return 0, nil, err
	}

	return int32(ret), &msg, nil
}
