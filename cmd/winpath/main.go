//go:build windows

package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type operation int

const (
	HWND_BROADCAST   = 0xFFFF
	WM_SETTINGCHANGE = 0x001A
	SMTO_ABORTIFHUNG = 0x0002
	ERR_BAD_ARGS     = 0x000A
	OPERATION_FAILED = 0x06AC

	Environment           = "Environment"
	Add         operation = iota
	Remove
	Open
	NotSpecified
)

func main() {
	op := NotSpecified
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "add":
			op = Add
		case "remove":
			op = Remove
		case "open":
			op = Open
		}
	}

	// Stay silent since ran from an installer
	if op == NotSpecified {
		alert("Usage: " + filepath.Base(os.Args[0]) + " [add|remove]\n\nThis utility adds or removes the podman directory to the Windows Path.")
		os.Exit(ERR_BAD_ARGS)
	}

	// Hidden operation as a workaround for the installer
	if op == Open && len(os.Args) > 2 {
		if err := winOpenFile(os.Args[2]); err != nil {
			os.Exit(OPERATION_FAILED)
		}
		os.Exit(0)
	}

	if err := modify(op); err != nil {
		os.Exit(OPERATION_FAILED)
	}
}

func modify(op operation) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	target := filepath.Dir(exe)

	if op == Remove {
		return removePathFromRegistry(target)
	}

	return addPathToRegistry(target)
}

// Appends a directory to the Windows Path stored in the registry
func addPathToRegistry(dir string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, Environment, registry.WRITE|registry.READ)
	if err != nil {
		return err
	}

	defer k.Close()

	existing, typ, err := k.GetStringValue("Path")
	if err != nil {
		return err
	}

	// Is this directory already on the windows path?
	for element := range strings.SplitSeq(existing, ";") {
		if strings.EqualFold(element, dir) {
			// Path already added
			return nil
		}
	}

	// If the existing path is empty we don't want to start with a delimiter
	if len(existing) > 0 {
		existing += ";"
	}

	existing += dir

	// It's important to preserve the registry key type so that it will be interpreted correctly
	// EXPAND = evaluate variables in the expression, e.g. %PATH% should be expanded to the system path
	// STRING = treat the contents as a string literal
	if typ == registry.EXPAND_SZ {
		err = k.SetExpandStringValue("Path", existing)
	} else {
		err = k.SetStringValue("Path", existing)
	}

	if err == nil {
		broadcastEnvironmentChange()
	}

	return err
}

// Removes all occurrences of a directory path from the Windows path stored in the registry
func removePathFromRegistry(path string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, Environment, registry.READ|registry.WRITE)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Nothing to clean up, the Environment registry key does not exist.
			return nil
		}
		return err
	}

	defer k.Close()

	existing, typ, err := k.GetStringValue("Path")
	if err != nil {
		return err
	}

	var elements []string
	for element := range strings.SplitSeq(existing, ";") {
		if strings.EqualFold(element, path) {
			continue
		}
		elements = append(elements, element)
	}

	newPath := strings.Join(elements, ";")
	// Preserve value type (see corresponding comment above)
	if typ == registry.EXPAND_SZ {
		err = k.SetExpandStringValue("Path", newPath)
	} else {
		err = k.SetStringValue("Path", newPath)
	}

	if err == nil {
		broadcastEnvironmentChange()
	}

	return err
}

// Sends a notification message to all top level windows informing them the environmental settings have changed.
// Applications such as the Windows command prompt and powershell will know to stop caching stale values on
// subsequent restarts. Since applications block the sender when receiving a message, we set a 3 second timeout
func broadcastEnvironmentChange() {
	env, _ := syscall.UTF16PtrFromString(Environment)
	user32 := syscall.NewLazyDLL("user32")
	proc := user32.NewProc("SendMessageTimeoutW")
	millis := 3000
	//nolint:dogsled
	_, _, _ = proc.Call(HWND_BROADCAST, WM_SETTINGCHANGE, 0, uintptr(unsafe.Pointer(env)), SMTO_ABORTIFHUNG, uintptr(millis), 0)
}

// Creates an "error" style pop-up window
func alert(caption string) int {
	// Error box style
	format := 0x10

	user32 := syscall.NewLazyDLL("user32.dll")
	captionPtr, _ := syscall.UTF16PtrFromString(caption)
	titlePtr, _ := syscall.UTF16PtrFromString("winpath")
	ret, _, _ := user32.NewProc("MessageBoxW").Call(
		uintptr(0),
		uintptr(unsafe.Pointer(captionPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(format))

	return int(ret)
}

func winOpenFile(file string) error {
	verb, _ := syscall.UTF16PtrFromString("open")
	fileW, _ := syscall.UTF16PtrFromString(file)
	return windows.ShellExecute(0, verb, fileW, nil, nil, windows.SW_NORMAL)
}
