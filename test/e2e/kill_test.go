//go:build linux || freebsd

package integration

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman kill", func() {
	It("podman kill bogus container", func() {
		session := podmanTest.Podman([]string{"kill", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `no container with name or ID "foobar" found: no such container`))
	})

	It("podman container kill a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "kill", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman container kill a running container by short id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"container", "kill", cid[:5]})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal(cid[:5]))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman kill a running container by id", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"kill", cid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman kill a running container by id with TERM", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"kill", "-s", "9", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman kill a running container by name", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"kill", "-s", "9", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman kill a running container by id with a bogus signal", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		result := podmanTest.Podman([]string{"kill", "-s", "foobar", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "invalid signal: foobar"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman kill latest container", func() {
		session := podmanTest.RunTopContainer("test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		cid := "-l"
		if IsRemote() {
			cid = "test1"
		}
		result := podmanTest.Podman([]string{"kill", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman kill paused container", func() {
		ctrName := "testctr"
		session := podmanTest.RunTopContainer(ctrName)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		pause := podmanTest.Podman([]string{"pause", ctrName})
		pause.WaitWithDefaultTimeout()
		Expect(pause).Should(ExitCleanly())

		kill := podmanTest.Podman([]string{"kill", ctrName})
		kill.WaitWithDefaultTimeout()
		Expect(kill).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "-f", "{{.State.Status}}", ctrName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(Or(Equal("stopped"), Equal("exited")))
	})

	It("podman kill --cidfile", func() {
		cidFile := filepath.Join(tempdir, "cid")

		session := podmanTest.Podman([]string{"run", "-dt", "--cidfile", cidFile, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToStringArray()[0]

		kill := podmanTest.Podman([]string{"kill", "--cidfile", cidFile})
		kill.WaitWithDefaultTimeout()
		Expect(kill).Should(ExitCleanly())

		wait := podmanTest.Podman([]string{"wait", "--condition", "exited", cid})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(ExitCleanly())
	})

	It("podman kill multiple --cidfile", func() {
		cidFile1 := filepath.Join(tempdir, "cid1")
		cidFile2 := filepath.Join(tempdir, "cid2")

		session := podmanTest.Podman([]string{"run", "-dt", "--cidfile", cidFile1, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid1 := session.OutputToStringArray()[0]

		session2 := podmanTest.Podman([]string{"run", "-dt", "--cidfile", cidFile2, ALPINE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
		cid2 := session2.OutputToStringArray()[0]

		kill := podmanTest.Podman([]string{"kill", "--cidfile", cidFile1, "--cidfile", cidFile2})
		kill.WaitWithDefaultTimeout()
		Expect(kill).Should(ExitCleanly())

		wait := podmanTest.Podman([]string{"wait", "--condition", "exited", cid1})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(ExitCleanly())
		wait = podmanTest.Podman([]string{"wait", "--condition", "exited", cid2})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(ExitCleanly())
	})

	It("podman kill --all", func() {
		session := podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))

		session = podmanTest.RunTopContainer("")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))

		session = podmanTest.Podman([]string{"kill", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})
})
