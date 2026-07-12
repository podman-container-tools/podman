//go:build amd64 || arm64

package machine

import (
	"fmt"
	"os"
	"strings"

	"go.podman.io/podman/v6/pkg/machine/define"
)

// GetDevNullFiles returns pointers to Read-only and Write-only DevNull files
func GetDevNullFiles() (*os.File, *os.File, error) {
	dnr, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0o755)
	if err != nil {
		return nil, nil, err
	}

	dnw, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0o755)
	if err != nil {
		if e := dnr.Close(); e != nil {
			err = e
		}
		return nil, nil, err
	}

	return dnr, dnw, nil
}

// WaitAPIAndPrintInfo prints info about the machine and does a ping test on the
// API socket
func WaitAPIAndPrintInfo(forwardState APIForwardingState, name, helper, forwardSock string, noInfo, _ bool) {
	suffix := ""
	var fmtString string

	if name != define.DefaultMachineName {
		suffix = " " + name
	}

	if forwardState == NoForwarding {
		return
	}

	WaitAndPingAPI(forwardSock)

	if !noInfo {
		fmt.Printf("API forwarding listening on: %s\n", forwardSock)
		if forwardState == DockerGlobal {
			fmt.Printf("Docker API clients default to this address. You do not need to set DOCKER_HOST.\n\n")
		} else {
			stillString := "still "
			switch forwardState {
			case NotInstalled:

				fmtString = `
The system helper service is not installed; the default Docker API socket
address can't be used by podman. `

				if len(helper) < 1 {
					fmt.Print(fmtString)
				} else {
					fmtString += `If you would like to install it, run the following commands:

        sudo %s install
        podman machine stop%[2]s; podman machine start%[2]s

`
					fmt.Printf(fmtString, helper, suffix)
				}
			case MachineLocal:
				fmt.Printf("\nAnother process was listening on the default Docker API socket address.\n")
			case ClaimUnsupported:
				fallthrough
			default:
				stillString = ""
			}

			fmtString = `You can %sconnect Docker API clients by setting DOCKER_HOST using the
following command in your terminal session:

        %s

`
			prefix := ""
			if !strings.Contains(forwardSock, "://") {
				prefix = "unix://"
			}
			fmt.Printf(fmtString, stillString, GetEnvSetString("DOCKER_HOST", prefix+forwardSock))
		}
	}
}

func PrintRootlessWarning(name string) {
	suffix := ""
	if name != define.DefaultMachineName {
		suffix = " " + name
	}

	fmtString := `
This machine is currently configured in rootless mode. If your containers
require root permissions (e.g. ports < 1024), or if you run into compatibility
issues with non-podman clients, you can switch using the following command:

	podman machine set --rootful%s

`
	fmt.Printf(fmtString, suffix)
}
