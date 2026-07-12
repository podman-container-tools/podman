//go:build !windows
// +build !windows

package winquit

import (
	"os"
)

func notifyOnQuit(done chan bool) {
}

func simulateSigTermOnQuit(handler chan os.Signal) {
}

func getCurrentMessageLoopThreadId() uint32 {
	return 0
}
