//go:build darwin

package apple

import (
	"net"
	"net/http"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

// ServeIgnitionOverSock allows podman to open a small httpd instance on the vsock between the host
// and guest to inject the ignitionfile into fcos
func ServeIgnitionOverSock(ignitionSocket *define.VMFile, mc *vmconfigs.MachineConfig) error {
	ignitionFile, err := mc.IgnitionFile()
	if err != nil {
		return err
	}

	logrus.Debugf("reading ignition file: %s", ignitionFile.GetPath())
	ignFile, err := ignitionFile.Read()
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write(ignFile)
		if err != nil {
			logrus.Errorf("failed to serve ignition file: %v", err)
		}
	})
	listener, err := net.Listen("unix", ignitionSocket.GetPath())
	if err != nil {
		return err
	}
	logrus.Debugf("ignition socket device: %s", ignitionSocket.GetPath())
	defer func() {
		if err := listener.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	return http.Serve(listener, mux)
}
