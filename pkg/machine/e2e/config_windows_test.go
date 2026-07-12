package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containers/libhvee/pkg/hypervctl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/machine/define"
)

const podmanBinary = "../../../bin/windows/podman.exe"

var fakeImagePath string = ""

func initPlatform() {
	switch testProvider.VMType().String() {
	case define.HyperVVirt.String():
		vmm := hypervctl.NewVirtualMachineManager()
		name := fmt.Sprintf("podman-hyperv-%s.vhdx", randomString())
		fullFileName := filepath.Join(tmpDir, name)
		if err := vmm.CreateVhdxFile(fullFileName, 15*1024*1024); err != nil {
			Fail(fmt.Sprintf("Failed to create file %s %q", fullFileName, err))
		}
		fakeImagePath = fullFileName
		fmt.Println("Created fake disk image: " + fakeImagePath)
	case define.WSLVirt.String():
	default:
		Fail(fmt.Sprintf("unknown Windows provider: %q", testProvider.VMType().String()))
	}
}

func cleanupPlatform() {
	if err := os.Remove(fakeImagePath); err != nil {
		fmt.Printf("Failed to remove %s image: %q\n", fakeImagePath, err)
	}
}

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

// withFakeImage should be used in tests where the machine is
// initialized (or not) but never started.  It is intended
// to speed up CI by not processing our large machine files.
func (i *initMachine) withFakeImage(mb *machineTestBuilder) *initMachine {
	switch testProvider.VMType() {
	case define.HyperVVirt:
		i.image = fakeImagePath
	case define.WSLVirt:
		i.image = mb.imagePath
	default:
		Fail(fmt.Sprintf("unknown Windows provider: %q", testProvider.VMType().String()))
	}
	return i
}
