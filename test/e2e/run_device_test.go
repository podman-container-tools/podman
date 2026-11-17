//go:build linux || freebsd

package integration

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

func createContainersConfFileWithDevices(pTest *PodmanTestIntegration, devices string, cdiSpecDirs []string) {
	configPath := filepath.Join(pTest.TempDir, "containers.conf")
	containersConf := fmt.Appendf(nil, "[containers]\ndevices = [%s]\n", devices)
	if len(cdiSpecDirs) > 0 {
		quoted := make([]string, len(cdiSpecDirs))
		for i, dir := range cdiSpecDirs {
			quoted[i] = fmt.Sprintf("%q", dir)
		}
		containersConf = append(containersConf, fmt.Appendf(nil, "[engine]\ncdi_spec_dirs = [%s]\n", strings.Join(quoted, ", "))...)
	}
	err := os.WriteFile(configPath, containersConf, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	// Set custom containers.conf file
	os.Setenv("CONTAINERS_CONF", configPath)
	if IsRemote() {
		pTest.RestartRemoteService()
	}
}

var _ = Describe("Podman run device", func() {
	It("podman run bad device test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--device", "/dev/baddevice", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "stat /dev/baddevice: no such file or directory"))
	})

	It("podman run device test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "/dev/kmsg", ALPINE, "test", "-c", "/dev/kmsg"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		if !isRootless() {
			// Kernel 6.9.0 (2024-03) requires SYSLOG
			// Do not read any data from the device (thus the -n 0), because there is a rare race condition that happens
			// when another writes to the ring buffer when the device is already opened and that causes the following test
			// to fail: https://github.com/containers/podman/issues/23882.
			session = podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "/dev/kmsg", "--cap-add", "SYS_ADMIN,SYSLOG", ALPINE, "head", "-n", "0", "/dev/kmsg"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
		}
	})

	It("podman run device rename test", func() {
		// TODO: Confirm absence of /dev/kmsg in container
		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "/dev/kmsg:/dev/kmsg1", ALPINE, "test", "-c", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run device permission test", func() {
		// TODO: Confirm write-permission failure
		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "/dev/kmsg:r", ALPINE, "test", "-r", "/dev/kmsg"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run device rename and permission test", func() {
		// TODO: Confirm write-permission failure
		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "/dev/kmsg:/dev/kmsg1:r", ALPINE, "test", "-r", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})
	It("podman run device rename and bad permission test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "/dev/kmsg:/dev/kmsg1:rd", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "invalid device mode \"rd\" in device \"/dev/kmsg:/dev/kmsg1:rd\""))
	})

	It("podman run device with empty mode test", func() {
		session := podmanTest.Podman([]string{"run", "-q", "--device", "/dev/fuse::", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "empty device mode in device specification: /dev/fuse::"))
	})

	It("podman run device host device and container device parameter are directories", func() {
		SkipIfRootless("Cannot create devices in /dev in rootless mode")
		// path must be unique to this test, not used anywhere else
		devdir := "/dev/devdirrundevice"
		Expect(os.MkdirAll(devdir, os.ModePerm)).To(Succeed())
		defer os.RemoveAll(devdir)

		mknod := SystemExec("mknod", []string{devdir + "/null", "c", "1", "3"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"run", "-q", "--device", devdir + ":/dev/bar", ALPINE, "stat", "-c%t:%T", "/dev/bar/null"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("1:3"))
	})

	It("podman run device host device with --privileged", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", ALPINE, "test", "-c", "/dev/kmsg"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// verify --privileged is required
		session2 := podmanTest.Podman([]string{"run", ALPINE, "test", "-c", "/dev/kmsg"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitWithError(1, ""))
		Expect(session2.OutputToString()).To(BeEmpty())
	})

	It("podman run CDI device test", func() {
		SkipIfRootless("Rootless will not be able to create files/folders in /etc")
		cdiDir := "/etc/cdi"
		if _, err := os.Stat(cdiDir); errors.Is(err, fs.ErrNotExist) {
			Expect(os.MkdirAll(cdiDir, os.ModePerm)).To(Succeed())
		}
		defer os.RemoveAll(cdiDir)

		cmd := exec.Command("cp", "cdi/device.json", cdiDir)
		err = cmd.Run()
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", "--device", "vendor.com/device=myKmsg", ALPINE, "test", "-c", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		createContainersConfFileWithDevices(podmanTest, "\"vendor.com/device=myKmsg\"", []string{})
		session = podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", ALPINE, "test", "-c", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run CDI device test with --cdi-spec-dir", func() {
		SkipIfRemote("The --cdi-spec-dir only works locally.")
		cdiDir := podmanTest.TempDir + "/cdi"
		Expect(os.MkdirAll(cdiDir, os.ModePerm)).To(Succeed())

		cmd := exec.Command("cp", "cdi/device.json", cdiDir)
		err = cmd.Run()
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--cdi-spec-dir", cdiDir, "-q", "--security-opt", "label=disable", "--device", "vendor.com/device=myKmsg", ALPINE, "test", "-c", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		createContainersConfFileWithDevices(podmanTest, "\"vendor.com/device=myKmsg\"", []string{cdiDir})
		session = podmanTest.Podman([]string{"run", "-q", "--security-opt", "label=disable", ALPINE, "test", "-c", "/dev/kmsg1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run cannot access non default devices", func() {
		// Unlikely to happen but do not read any data from the device (thus the -n 0), because there is a rare
		// race condition that happens so the test would fail for the rare race condition instead of a failure
		// reported by open(). More details: https://github.com/containers/podman/issues/23882.
		session := podmanTest.Podman([]string{"run", "-v", "/dev:/dev-host", ALPINE, "head", "-n0", "/dev-host/kmsg"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithErrorRegex(1, "head: /dev-host/kmsg: (Operation not permitted|Permission denied)"))
	})
})
