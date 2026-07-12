//go:build linux || freebsd

package integration

import (
	"os"
	"strconv"
	"strings"

	"github.com/moby/sys/capability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

// helper function for confirming that container capabilities are equal
// to those of the host, but only to the extent of caps we (podman)
// know about at compile time. That is: the kernel may have more caps
// available than we are aware of, leading to host=FFF... and ctr=3FF...
// because the latter is all we request. Accept that.
func containerCapMatchesHost(ctrCap string, hostCap string) {
	if isRootless() {
		return
	}
	ctrCapN, err := strconv.ParseUint(ctrCap, 16, 64)
	Expect(err).NotTo(HaveOccurred(), "Error parsing %q as hex", ctrCap)

	hostCapN, err := strconv.ParseUint(hostCap, 16, 64)
	Expect(err).NotTo(HaveOccurred(), "Error parsing %q as hex", hostCap)

	// host caps can never be zero (except rootless).
	// and host caps must always be a superset (inclusive) of container
	Expect(hostCapN).To(BeNumerically(">", 0), "host cap %q should be nonzero", hostCap)
	Expect(hostCapN).To(BeNumerically(">=", ctrCapN), "host cap %q should never be less than container cap %q", hostCap, ctrCap)
	hostCapMasked := hostCapN & (1<<len(capability.ListKnown()) - 1)
	Expect(ctrCapN).To(Equal(hostCapMasked), "container cap %q is not a subset of host cap %q", ctrCap, hostCap)
}

var _ = Describe("Podman privileged container tests", func() {
	It("podman privileged make sure sys is mounted rw", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", BB, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("sysfs (rw,"))
	})

	It("podman privileged CapEff", func() {
		hostCap := SystemExec("awk", []string{"/^CapEff/ { print $2 }", "/proc/self/status"})
		Expect(hostCap).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"run", "--privileged", BB, "awk", "/^CapEff/ { print $2 }", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		containerCapMatchesHost(session.OutputToString(), hostCap.OutputToString())
	})

	It("podman cap-add CapEff", func() {
		// Get caps of current process
		hostCap := SystemExec("awk", []string{"/^CapEff/ { print $2 }", "/proc/self/status"})
		Expect(hostCap).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"run", "--cap-add", "all", BB, "awk", "/^CapEff/ { print $2 }", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		containerCapMatchesHost(session.OutputToString(), hostCap.OutputToString())
	})

	It("podman cap-add CapEff with --user", func() {
		// Get caps of current process
		hostCap := SystemExec("awk", []string{"/^CapEff/ { print $2 }", "/proc/self/status"})
		Expect(hostCap).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"run", "--user=bin", "--cap-add", "all", BB, "awk", "/^CapEff/ { print $2 }", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		containerCapMatchesHost(session.OutputToString(), hostCap.OutputToString())
	})

	It("podman cap-drop CapEff", func() {
		session := podmanTest.Podman([]string{"run", "--cap-drop", "all", BB, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		capEff := strings.Split(session.OutputToString(), " ")
		Expect("0000000000000000").To(Equal(capEff[1]))
	})

	It("podman privileged should disable seccomp by default", func() {
		hostSeccomp := SystemExec("grep", []string{"-Ei", "^Seccomp:\\s+0$", "/proc/self/status"})
		Expect(hostSeccomp).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"run", "--privileged", ALPINE, "grep", "-Ei", "^Seccomp:\\s+0$", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman non-privileged should have very few devices", func() {
		session := podmanTest.Podman([]string{"run", "-t", BB, "ls", "-l", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(17))
	})

	It("podman privileged should inherit host devices", func() {
		session := podmanTest.Podman([]string{"run", "--privileged", ALPINE, "ls", "-l", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(len(session.OutputToStringArray())).To(BeNumerically(">", 20))
	})

	It("podman privileged should restart after host devices change", func() {
		containerName := "privileged-restart-test"
		SkipIfRootless("Cannot create devices in /dev in rootless mode")
		// path must be unique to this test, not used anywhere else
		devdir := "/dev/devdirprivrestart"
		Expect(os.MkdirAll(devdir, os.ModePerm)).To(Succeed())
		defer os.RemoveAll(devdir)

		mknod := SystemExec("mknod", []string{devdir + "/null", "c", "1", "3"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"run", "--name=" + containerName, "--privileged", FEDORA_MINIMAL, "ls", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		deviceFiles := session.OutputToStringArray()

		os.RemoveAll(devdir)
		session = podmanTest.Podman([]string{"start", "--attach", containerName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		deviceFilesAfterRemoval := session.OutputToStringArray()
		Expect(deviceFiles).To(Not(Equal(deviceFilesAfterRemoval)))
	})

	It("run no-new-privileges test", func() {
		cap := SystemExec("grep", []string{"NoNewPrivs", "/proc/self/status"})
		if cap.ExitCode() != 0 {
			Skip("Can't determine NoNewPrivs")
		}

		fields := strings.Fields(cap.OutputToString())
		if fields[1] != "0" {
			Skip("NoNewPrivs set")
		}

		session := podmanTest.Podman([]string{"run", BB, "grep", "NoNewPrivs", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		privs := strings.Fields(session.OutputToString())
		Expect(privs[1]).To(Equal("0"), "NoNewPrivs should be 0 without security-opt")

		session = podmanTest.Podman([]string{"run", "--security-opt", "no-new-privileges", BB, "grep", "NoNewPrivs", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		noprivs := strings.Fields(session.OutputToString())
		Expect(noprivs[1]).To(Equal("1"), "NoNewPrivs should be 1 with security-opt")
	})
})
