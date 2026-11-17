package tunnel

import (
	"context"
	"os"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/signal"
)

// Image-related runtime using an ssh-tunnel to utilize Podman service
type ImageEngine struct {
	ClientCtx context.Context
	FarmNode
}

// Container-related runtime using an ssh-tunnel to utilize Podman service
type ContainerEngine struct {
	ClientCtx context.Context
}

// Container-related runtime using an ssh-tunnel to utilize Podman service
type SystemEngine struct {
	ClientCtx context.Context
}

type FarmNode struct {
	NodeName        string
	platforms       sync.Once
	platformsErr    error
	os              string
	arch            string
	variant         string
	nativePlatforms []string
}

func remoteProxySignals(ctrID string, killFunc func(string) error) {
	sigBuffer := make(chan os.Signal, signal.SignalBufferSize)
	signal.CatchAll(sigBuffer)

	logrus.Debugf("Enabling signal proxying")

	go func() {
		for s := range sigBuffer {
			syscallSignal := s.(syscall.Signal)
			signalName, err := signal.ParseSysSignalToName(syscallSignal)
			if err != nil {
				logrus.Infof("Ceasing signal %v forwarding to container %s as it has stopped: %s", s, ctrID, err)
			}
			if err := killFunc(signalName); err != nil {
				if err.Error() == define.ErrCtrStateInvalid.Error() {
					logrus.Debugf("Ceasing signal %q forwarding to container %s as it has stopped", signalName, ctrID)
				}
			}
		}
	}()
}
