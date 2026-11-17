//go:build linux || freebsd

package integration

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman top", func() {
	It("podman pod top without pod name or id", func() {
		result := podmanTest.Podman([]string{"pod", "top"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "you must provide the name or id of a running pod"))
	})

	It("podman pod top on bogus pod", func() {
		result := podmanTest.Podman([]string{"pod", "top", "1234"})
		result.WaitWithDefaultTimeout()
		expect := "no pod with name or ID 1234 found: no such pod"
		if !IsRemote() {
			expect = "unable to look up requested container: " + expect
		}
		Expect(result).Should(ExitWithError(125, expect))
	})

	It("podman pod top on non-running pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"top", podid})
		result.WaitWithDefaultTimeout()
		expect := fmt.Sprintf(`no container with name or ID "%s" found: no such container`, podid)
		if !IsRemote() {
			expect = "unable to look up requested container: " + expect
		}
		Expect(result).Should(ExitWithError(125, expect))
	})

	It("podman pod top on pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "-d", "--pod", podid, ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		if !IsRemote() {
			podid = "-l"
		}
		result := podmanTest.Podman([]string{"pod", "top", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman pod top with options", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "-d", "--pod", podid, ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "top", podid, "pid", "%C", "args"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(len(result.OutputToStringArray())).To(BeNumerically(">", 1))
	})

	It("podman pod top on pod invalid options", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "-d", "--pod", podid, ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// We need to pass -eo to force executing ps in the Alpine container.
		// Alpines stripped down ps(1) is accepting any kind of weird input in
		// contrast to others, such that a `ps invalid` will silently ignore
		// the wrong input and still print the -ef output instead.
		result := podmanTest.Podman([]string{"pod", "top", podid, "-eo", "invalid"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "Error: '-eo': unknown descriptor"))
	})

	It("podman pod top on pod with containers in same pid namespace", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "-d", "--pod", podid, ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "-d", "--pod", podid, "--pid", fmt.Sprintf("container:%s", cid), ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "top", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(HaveLen(3))
	})

	It("podman pod top on pod with containers in different namespace", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"run", "-d", "--pod", podid, ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-d", "--pod", podid, ALPINE, "top", "-d", "2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		for range 10 {
			GinkgoWriter.Println("Waiting for containers to be running .... ")
			if podmanTest.NumberOfContainersRunning() == 2 {
				break
			}
			time.Sleep(1 * time.Second)
		}
		result := podmanTest.Podman([]string{"pod", "top", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).To(HaveLen(3))
	})
})
