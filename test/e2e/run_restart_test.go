//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman run restart containers", func() {
	It("Podman start after successful run", func() {
		session := podmanTest.Podman([]string{"run", "--name", "test", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"wait", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session2 := podmanTest.Podman([]string{"start", "--attach", "test"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
	})

	It("Podman start after signal kill", func() {
		_ = podmanTest.RunTopContainer("test1")

		killSession := podmanTest.Podman([]string{"kill", "-s", "9", "test1"})
		killSession.WaitWithDefaultTimeout()
		Expect(killSession).Should(ExitCleanly())

		session2 := podmanTest.Podman([]string{"start", "test1"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
	})
})
