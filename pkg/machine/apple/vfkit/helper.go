//go:build darwin

package vfkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/crc-org/vfkit/pkg/config"
	rest "github.com/crc-org/vfkit/pkg/rest/define"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/define"
	"golang.org/x/sys/unix"
)

const (
	inspect = "/vm/inspect"
	state   = "/vm/state"
	version = "/version"
)

func (vf *Helper) get(endpoint string, payload io.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, endpoint, payload)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (vf *Helper) post(endpoint string, payload io.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// getRawState asks vfkit for virtual machine state unmodified (see state())
func (vf *Helper) getRawState() (define.Status, error) {
	var response rest.VMState
	endPoint := vf.Endpoint + state
	serverResponse, err := vf.get(endPoint, nil)
	if err != nil {
		if errors.Is(err, unix.ECONNREFUSED) {
			logrus.Debugf("connection refused: %s", endPoint)
		}
		return "", err
	}
	err = json.NewDecoder(serverResponse.Body).Decode(&response)
	if err != nil {
		return "", err
	}
	if err := serverResponse.Body.Close(); err != nil {
		logrus.Error(err)
	}
	return ToMachineStatus(response.State)
}

// state asks vfkit for the virtual machine state. in case the vfkit
// service is not responding, we assume the service is not running
// and return a stopped status
func (vf *Helper) State() (define.Status, error) {
	vmState, err := vf.getRawState()
	if err == nil {
		return vmState, nil
	}
	if errors.Is(err, unix.ECONNREFUSED) {
		return define.Stopped, nil
	}
	return "", err
}

func (vf *Helper) stateChange(newState rest.StateChange) error {
	b, err := json.Marshal(rest.VMState{State: string(newState)})
	if err != nil {
		return err
	}
	payload := bytes.NewReader(b)
	serverResponse, err := vf.post(vf.Endpoint+state, payload)
	if err == nil {
		_ = serverResponse.Body.Close()
	}
	return err
}

func (vf *Helper) Stop(force, wait bool) error {
	state := rest.Stop
	if force {
		state = rest.HardStop
	}
	if err := vf.stateChange(state); err != nil {
		return err
	}
	if !wait {
		return nil
	}
	waitDuration := time.Millisecond * 500
	// Wait up to 90s then hard force off
	for range 180 {
		_, err := vf.getRawState()
		if err != nil {
			//nolint:nilerr // error means vfkit is gone so machine is stopped
			return nil
		}
		time.Sleep(waitDuration)
	}
	logrus.Warn("Failed to gracefully stop machine, performing hard stop")
	// we waited long enough do a hard stop
	return vf.stateChange(rest.HardStop)
}

// Helper describes the use of vfkit: cmdline and endpoint
type Helper struct {
	LogLevel       logrus.Level
	Endpoint       string
	BinaryPath     *define.VMFile
	VirtualMachine *config.VirtualMachine
	Rosetta        bool
}
