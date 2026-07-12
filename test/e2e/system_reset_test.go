//go:build linux || freebsd

package integration

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

// system reset must run serial: https://github.com/containers/podman/issues/17903
var _ = Describe("podman system reset", Serial, func() {
	It("podman system reset", func() {
		SkipIfRemote("system reset not supported on podman --remote")
		// system reset will not remove additional store images, so need to grab length
		useCustomNetworkDir(podmanTest, tempdir)

		session := podmanTest.Podman([]string{"rmi", "--force", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"images", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		l := len(session.OutputToStringArray())

		podmanTest.AddImageToRWStore(ALPINE)
		session = podmanTest.Podman([]string{"volume", "create", "data"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "-v", "data:/data", ALPINE, "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"network", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"system", "reset", "-f"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		Expect(session.ErrorToString()).To(Not(ContainSubstring("Failed to add pause process")))
		Expect(session.ErrorToString()).To(Not(ContainSubstring("/usr/share/containers/storage.conf")))

		session = podmanTest.Podman([]string{"images", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(l))

		session = podmanTest.Podman([]string{"volume", "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(1))

		session = podmanTest.Podman([]string{"container", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(BeEmpty())

		session = podmanTest.Podman([]string{"network", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// default network should exists
		Expect(session.OutputToStringArray()).To(HaveLen(1))

		// TODO: machine tests currently don't run outside of the machine test pkg
		// no machines are created here to cleanup
		// machine commands are rootless only
		if isRootless() {
			session = podmanTest.Podman([]string{"machine", "list", "-q"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToStringArray()).To(BeEmpty())
		}
	})

	It("system reset completely removes container", func() {
		SkipIfRemote("system reset not supported on podman --remote")
		useCustomNetworkDir(podmanTest, tempdir)

		rmi := podmanTest.Podman([]string{"rmi", "--force", "--all"})
		rmi.WaitWithDefaultTimeout()
		Expect(rmi).Should(ExitCleanly())
		podmanTest.AddImageToRWStore(ALPINE)

		// The name ensures that we have a Libpod resource that we'll
		// hit if we recreate the container after a reset and it still
		// exists. The port does the same for a system-level resource.
		ctrName := "testctr"
		port1 := GetPort()
		port2 := GetPort()
		session := podmanTest.Podman([]string{"run", "--name", ctrName, "-p", fmt.Sprintf("%d:%d", port1, port2), "-d", ALPINE, "sleep", "inf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// run system reset on a container that is running
		// set a timeout of 9 seconds, which tests that reset is using the timeout
		// of zero and forceable killing containers with no wait.
		// #21874
		reset := podmanTest.Podman([]string{"system", "reset", "--force"})
		reset.WaitWithTimeout(9)
		Expect(reset).Should(ExitCleanly())

		session2 := podmanTest.Podman([]string{"run", "--name", ctrName, "-p", fmt.Sprintf("%d:%d", port1, port2), "-d", ALPINE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
	})
})
