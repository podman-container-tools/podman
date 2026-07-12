package winquit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/containers/winquit/pkg/winquit/win32"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

type receiversType struct {
	sync.Mutex

	result   bool
	channels map[any]baseChannelType
}

var (
	receivers *receiversType = &receiversType{
		channels: make(map[any]baseChannelType),
	}

	loopInit sync.Once
	loopTid  uint32
)

func (r *receiversType) add(channel baseChannelType) {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.channels[channel.getKey()]; ok {
		return
	}

	if r.result {
		go func() {
			channel.notifyBlocking()
		}()
		return
	}

	r.channels[channel.getKey()] = channel
}

func (r *receiversType) notifyAll() {
	r.Lock()
	defer r.Unlock()
	r.result = true
	for _, channel := range r.channels {
		channel.notifyNonBlocking()
		delete(r.channels, channel.getKey())
	}
	for _, channel := range r.channels {
		channel.notifyBlocking()
		delete(r.channels, channel)
	}
}

func initLoop() {
	loopInit.Do(func() {
		go messageLoop()
	})
}

func notifyOnQuit(done chan bool) {
	receivers.add(&boolChannelType{done})
	initLoop()
}

func simulateSigTermOnQuit(handler chan os.Signal) {
	receivers.add(&sigChannelType{handler})
	initLoop()
}

func getCurrentMessageLoopThreadId() uint32 {
	return loopTid
}

func messageLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	loopTid = windows.GetCurrentThreadId()
	registerDummyWindow()

	logrus.Debug("Entering loop for quit")
	for {
		ret, msg, err := win32.GetMessage(0, 0, 0)
		if err != nil {
			logrus.Debugf("Error receiving win32 message, %s", err.Error())
			continue
		}
		if ret == 0 {
			logrus.Debug("Received QUIT notification")
			receivers.notifyAll()

			return
		}
		logrus.Debugf("Unhandled message: %d", msg.Message)
		win32.TranslateMessage(msg)
		win32.DispatchMessage(msg)
	}
}

func getAppName() (string, error) {
	exeName, err := os.Executable()
	if err != nil {
		return "", err
	}
	suffix := filepath.Ext(exeName)
	return strings.TrimSuffix(filepath.Base(exeName), suffix), nil
}

func registerDummyWindow() error {
	var app syscall.Handle
	var err error

	app, err = win32.GetModuleHandle("")
	if err != nil {
		return err
	}

	appName, err := getAppName()
	if err != nil {
		return err
	}

	className := appName + "-rclass"
	winName := appName + "-root"

	_, err = win32.RegisterDummyWinClass(className, app)
	if err != nil {
		return err
	}

	_, err = win32.CreateDummyWindow(winName, className, app)
	if err != nil {
		return err
	}

	return nil
}
