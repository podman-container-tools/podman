package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"

	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
)

const podmanBinary = "../../../bin/windows/podman.exe"

// pgrep emulates the pgrep linux command
func pgrep(n string) (string, error) {
	// add filter to find the process and do no display a header
	args := []string{"/fi", fmt.Sprintf("IMAGENAME eq %s", n), "/nh"}
	out, err := exec.Command("tasklist.exe", args...).Output()
	if err != nil {
		return "", err
	}
	strOut := string(out)
	// in pgrep, if no running process is found, it exits 1 and the output is zilch
	if strings.Contains(strOut, "INFO: No tasks are running which match the specified search") {
		return "", fmt.Errorf("no task found")
	}
	return strOut, nil
}

func getOtherProvider() string {
	if isVmtype(define.WSLVirt) {
		return "hyperv"
	} else if isVmtype(define.HyperVVirt) {
		return "wsl"
	}
	return ""
}

// apiProxyPID returns the PID of the host proxy serving the machine API:
// win-sshproxy on WSL (from its tid file), gvproxy otherwise.
func apiProxyPID(name, runtimeDir string) int {
	if testProvider.VMType() != define.WSLVirt {
		return gvproxyPID(runtimeDir)
	}
	stateDir, err := machine.GetWinProxyStateDir(name, define.WSLVirt)
	Expect(err).ToNot(HaveOccurred())
	contents, err := os.ReadFile(filepath.Join(stateDir, "win-sshproxy.tid"))
	Expect(err).ToNot(HaveOccurred())
	var pid, tid int
	_, err = fmt.Sscanf(string(contents), "%d:%d", &pid, &tid)
	Expect(err).ToNot(HaveOccurred())
	return pid
}

// processExists reports whether pid is running.
func processExists(pid int) bool {
	active, _ := machine.GetProcessState(pid)
	return active
}

func runWslCommand(cmdArgs []string) *machineSession {
	binary := "wsl"
	GinkgoWriter.Println(binary + " " + strings.Join(cmdArgs, " "))
	c := exec.Command(binary, cmdArgs...)
	session, err := Start(c, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("Unable to start session: %q", err))
	}
	ms := machineSession{session}
	ms.waitWithTimeout(defaultTimeout)
	return &ms
}
