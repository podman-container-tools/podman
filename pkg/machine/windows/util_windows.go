package windows

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"unsafe"

	"github.com/sirupsen/logrus"
	"go.podman.io/storage/pkg/homedir"
	"golang.org/x/sys/windows"
)

const UACConfirmationPrompt = "Since you are not running as admin, a new window will open and " +
	"require you to approve administrator privileges.\n\n"

// IsInAdministratorsGroup checks if the current user is a member of the Administrators group,
// regardless of whether the current process is elevated. This can be used to determine
// if the user can elevate privileges via UAC.
func IsInAdministratorsGroup() bool {
	u, err := user.Current()
	if nil != err {
		return false
	}
	ids, err := u.GroupIds()
	if nil != err {
		return false
	}
	// S-1-5-32-544 is the SID for the Administrators group
	// see: https://learn.microsoft.com/en-us/windows-server/identity/ad-ds/manage/understand-security-identifiers
	return slices.Contains(ids, "S-1-5-32-544")
}

// HasAdminRights checks if the current process has administrator privileges.
func HasAdminRights() bool {
	var sid *windows.SID

	// See: https://coolaj86.com/articles/golang-and-windows-and-admins-oh-my/
	if err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid); err != nil {
		logrus.Warnf("SID allocation error: %s", err)
		return false
	}
	defer func() {
		_ = windows.FreeSid(sid)
	}()

	//  From MS docs:
	// "If TokenHandle is NULL, CheckTokenMembership uses the impersonation
	//  token of the calling thread. If the thread is not impersonating,
	//  the function duplicates the thread's primary token to create an
	//  impersonation token."
	token := windows.Token(0)

	member, err := token.IsMember(sid)
	if err != nil {
		logrus.Warnf("Token Membership Error: %s", err)
		return false
	}

	return member || token.IsElevated()
}

type SHELLEXECUTEINFO struct {
	cbSize         uint32
	fMask          uint32
	hwnd           syscall.Handle
	lpVerb         uintptr
	lpFile         uintptr
	lpParameters   uintptr
	lpDirectory    uintptr
	nShow          int
	hInstApp       syscall.Handle
	lpIDList       uintptr
	lpClass        uintptr
	hkeyClass      syscall.Handle
	dwHotKey       uint32
	hIconOrMonitor syscall.Handle
	hProcess       syscall.Handle
}

// Cleaner to refer to the official OS constant names, and consistent with syscall
// Ref: https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-shellexecuteinfow#members
const (
	SEE_MASK_NOCLOSEPROCESS = 0x40
	SE_ERR_ACCESSDENIED     = 0x05
)

type ExitCodeError struct {
	Code uint
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("process exited with code %d", e.Code)
}

// BuildCommandArgs builds command line arguments for re-execution, optionally adding --reexec flag
// for specified commands (init, rm)
func BuildCommandArgs(elevate bool) string {
	var args []string
	for _, arg := range os.Args[1:] {
		if arg != "--reexec" {
			args = append(args, syscall.EscapeArg(arg))
			if elevate && (arg == "init" || arg == "rm") {
				args = append(args, "--reexec")
			}
		}
	}
	return strings.Join(args, " ")
}

// RelaunchElevatedWait launches the current executable with elevated privileges and waits for it to complete
func RelaunchElevatedWait() error {
	e, _ := os.Executable()
	d, _ := os.Getwd()
	exe, _ := syscall.UTF16PtrFromString(e)
	cwd, _ := syscall.UTF16PtrFromString(d)
	arg, _ := syscall.UTF16PtrFromString(BuildCommandArgs(true))
	verb, _ := syscall.UTF16PtrFromString("runas")

	shell32 := syscall.NewLazyDLL("shell32.dll")

	info := &SHELLEXECUTEINFO{
		fMask:        SEE_MASK_NOCLOSEPROCESS,
		hwnd:         0,
		lpVerb:       uintptr(unsafe.Pointer(verb)),
		lpFile:       uintptr(unsafe.Pointer(exe)),
		lpParameters: uintptr(unsafe.Pointer(arg)),
		lpDirectory:  uintptr(unsafe.Pointer(cwd)),
		nShow:        syscall.SW_SHOWNORMAL,
	}
	info.cbSize = uint32(unsafe.Sizeof(*info))
	procShellExecuteEx := shell32.NewProc("ShellExecuteExW")
	if ret, _, _ := procShellExecuteEx.Call(uintptr(unsafe.Pointer(info))); ret == 0 { // 0 = False
		err := syscall.GetLastError()
		if info.hInstApp == SE_ERR_ACCESSDENIED {
			return wrapMaybe(err, "request to elevate privileges was denied")
		}
		return wrapMaybef(err, "could not launch process, ShellEX Error = %d", info.hInstApp)
	}

	handle := info.hProcess
	defer func() {
		_ = syscall.CloseHandle(handle)
	}()

	w, err := syscall.WaitForSingleObject(handle, syscall.INFINITE)
	switch w {
	case syscall.WAIT_OBJECT_0:
		break
	case syscall.WAIT_FAILED:
		return fmt.Errorf("could not wait for process, failed: %w", err)
	default:
		return fmt.Errorf("could not wait for process, unknown error. event: %X, err: %v", w, err)
	}
	var code uint32
	if err := syscall.GetExitCodeProcess(handle, &code); err != nil {
		return err
	}
	if code != 0 {
		return &ExitCodeError{uint(code)}
	}

	return nil
}

// MessageBox shows a Windows message box and returns the user's choice
func MessageBox(caption, title string, fail bool) int {
	var format uint32
	if fail {
		format = windows.MB_ICONERROR
	} else {
		format = windows.MB_OKCANCEL | windows.MB_ICONINFORMATION
	}

	captionPtr, _ := syscall.UTF16PtrFromString(caption)
	titlePtr, _ := syscall.UTF16PtrFromString(title)

	ret, _ := windows.MessageBox(0, captionPtr, titlePtr, format)

	return int(ret)
}

func wrapMaybe(err error, message string) error {
	if err != nil {
		return fmt.Errorf("%v: %w", message, err)
	}

	return errors.New(message)
}

func wrapMaybef(err error, format string, args ...any) error {
	if err != nil {
		return fmt.Errorf(format+": %w", append(args, err)...)
	}

	return fmt.Errorf(format, args...)
}

// IsReExecuting checks if the current process was re-executed with --reexec flag
func IsReExecuting() bool {
	return slices.Contains(os.Args, "--reexec")
}

func CreateOrTruncateElevatedOutputFile() error {
	name, err := getElevatedOutputFileName()
	if err != nil {
		return err
	}

	_, err = os.Create(name)
	return err
}

// getElevatedOutputFileName returns the path to the elevated output log file
func getElevatedOutputFileName() (string, error) {
	dir, err := homedir.GetDataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "podman-elevated-output.log"), nil
}

func getElevatedOutputFileRead() (*os.File, error) {
	return getElevatedOutputFile(os.O_RDONLY)
}

func GetElevatedOutputFileWrite() (*os.File, error) {
	return getElevatedOutputFile(os.O_WRONLY | os.O_CREATE | os.O_APPEND)
}

func getElevatedOutputFile(mode int) (*os.File, error) {
	name, err := getElevatedOutputFileName()
	if err != nil {
		return nil, err
	}

	dir, err := homedir.GetDataHome()
	if err != nil {
		return nil, err
	}

	if err = os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	return os.OpenFile(name, mode, 0o644)
}

func DumpOutputFile() {
	file, err := getElevatedOutputFileRead()
	if err != nil {
		logrus.Debug("could not find elevated child output file")
		return
	}
	defer file.Close()
	_, _ = io.Copy(os.Stdout, file)
}

// LogErrorToFile logs an error message to the elevated output log file if --reexec is detected
func LogErrorToFile(err error) {
	if err == nil {
		return
	}
	file, fileErr := GetElevatedOutputFileWrite()
	if fileErr != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "Error: %v\n", err)
}
