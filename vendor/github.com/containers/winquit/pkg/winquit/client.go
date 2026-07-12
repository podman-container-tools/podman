package winquit

import (
	"time"
)

// RequestQuit sends a Windows quit notification to the specified process id.
// Since communication is performed over the Win32 GUI messaging facilities,
// console applications may not respond, as they require special handling to do
// so. Additionally incorrectly written or buggy GUI applications may not listen
// or respond appropriately to the event.
//
// All applications, console or GUI, which use the notification mechanisms
// provided by this package (NotifyOnQuit, SimulateSigTermOnQuit) will react
// appropriately to the event sent by RequestQuit.
//
// Callers must have appropriate security permissions, otherwise an error will
// be returned. See the notes in the package documentation for more details.
func RequestQuit(pid int) error {
	return requestQuit(pid)
}

// QuitProcess first sends a Windows quit notification to the specified process id,
// and waits, up the amount of time passed in the waitNicely argument, for it to
// exit. If the process does not exit in time, it is forcefully terminated.
//
// Callers must have appropriate security permissions, otherwise an error will
// be returned. See the notes in the package documentation for more details.
func QuitProcess(pid int, waitNicely time.Duration) error {
	return quitProcess(pid, waitNicely)
}
