//go:build windows

package hypervctl

import (
	"errors"
	"fmt"

	"github.com/containers/libhvee/pkg/wmiext"
)

// VM State errors
var (
	ErrMachineAlreadyRunning = errors.New("machine already running")
	ErrMachineNotRunning     = errors.New("machine not running")
	ErrMachineStateInvalid   = errors.New("machine in invalid state for action")
	ErrMachineStarting       = errors.New("machine is currently starting")
)

// VM Creation errors
var (
	ErrMachineAlreadyExists = errors.New("machine already exists")
)

type DestroySystemResult int32

// VM Destroy Exit Codes
const (
	VMDestroyCompletedwithNoError DestroySystemResult = 0
	VMDestroyNotSupported         DestroySystemResult = 1
	VMDestroyFailed               DestroySystemResult = 2
	VMDestroyTimeout              DestroySystemResult = 3
	VMDestroyInvalidParameter     DestroySystemResult = 4
	VMDestroyInvalidState         DestroySystemResult = 5
)

func (e DestroySystemResult) Reason() string {
	switch e {
	case VMDestroyNotSupported:
		return "not supported"
	case VMDestroyFailed:
		return "failed"
	case VMDestroyTimeout:
		return "timeout"
	case VMDestroyInvalidParameter:
		return "invalid parameter"
	case VMDestroyInvalidState:
		return "invalid state"
	}
	return "Unknown"
}

// Shutdown operation error codes
const (
	ErrShutdownFailed           = 32768
	ErrShutdownAccessDenied     = 32769
	ErrShutdownNotSupported     = 32770
	ErrShutdownStatusUnkown     = 32771
	ErrShutdownTimeout          = 32772
	ErrShutdownInvalidParameter = 32773
	ErrShutdownSystemInUse      = 32774
	ErrShutdownInvalidState     = 32775
	ErrShutdownIncorrectData    = 32776
	ErrShutdownNotAvailable     = 32777
	ErrShutdownOutOfMemory      = 32778
	ErrShutdownFileNotFound     = 32779
	ErrShutdownNotReady         = 32780
	ErrShutdownMachineLocked    = 32781
	ErrShutdownInProgress       = 32782
)

type shutdownCompError struct {
	errorCode int
	message   string
}

func (s *shutdownCompError) Error() string {
	return fmt.Sprintf("%s (%d)", s.message, s.errorCode)
}

func translateShutdownError(code int) error {
	var message string
	switch code {
	case ErrShutdownFailed:
		message = "shutdown failed"
	case ErrShutdownAccessDenied:
		message = "access was denied"
	case ErrShutdownNotSupported:
		message = "shutdown not supported by virtual machine"
	case ErrShutdownStatusUnkown:
		message = "virtual machine status is unknown"
	case ErrShutdownTimeout:
		message = "timeout starting shutdown"
	case ErrShutdownInvalidParameter:
		message = "invalid parameter"
	case ErrShutdownSystemInUse:
		message = "system in use"
	case ErrShutdownInvalidState:
		message = "virtual machine is in an invalid state for shutdown"
	case ErrShutdownIncorrectData:
		message = "incorrect data type"
	case ErrShutdownNotAvailable:
		message = "system is not available"
	case ErrShutdownOutOfMemory:
		message = "out of memory"
	case ErrShutdownFileNotFound:
		message = "file not found"
	case ErrShutdownMachineLocked:
		message = "machine is locked and cannot be shut down without the force option"
	case ErrShutdownInProgress:
		message = "shutdown is already in progress"
	default:
		message = "unknown error"
	}

	return &shutdownCompError{code, message}
}

// Modify resource errors
const (
	ErrModifyResourceNotSupported     = 1
	ErrModifyResourceFailed           = 2
	ErrModifyResourceTimeout          = 3
	ErrModifyResourceInvalidParameter = 4
	ErrModifyResourceInvalidState     = 5
	ErrModifyResourceIncompatParam    = 6
)

type modifyResourceError struct {
	errorCode int
	message   string
}

func (m *modifyResourceError) Error() string {
	return fmt.Sprintf("%s (%d)", m.message, m.errorCode)
}

func translateModifyError(code int) error {
	var message string
	switch code {
	case ErrModifyResourceNotSupported:
		message = "virtual machine does not support modification operations"
	case ErrModifyResourceFailed:
		message = "resource modification failed"
	case ErrModifyResourceTimeout:
		message = "timeout modifying resource"
	case ErrModifyResourceInvalidParameter:
		message = "a modify resource operation was passed an invalid parameter"
	case ErrModifyResourceInvalidState:
		message = "the requested modification could not be applied due to an invalid state"
	case ErrModifyResourceIncompatParam:
		message = "an incompatible parameter was passed to a modify resource operation"
	default:
		message = "unknown error"
	}

	return &modifyResourceError{code, message}
}

var (
	ErrHyperVNamespaceMissing = errors.New("HyperV namespace not found, is HyperV enabled?")
)

func translateCommonHyperVWmiError(wmiError error) error {
	if werr, ok := wmiError.(*wmiext.WmiError); ok {
		switch werr.Code() {
		case wmiext.WBEM_E_INVALID_NAMESPACE:
			return ErrHyperVNamespaceMissing
		}
	}

	return wmiError
}
