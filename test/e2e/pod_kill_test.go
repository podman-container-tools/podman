//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pod kill", func() {
	It("podman pod kill bogus", func() {
		session := podmanTest.Podman([]string{"pod", "kill", "foobar"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID foobar found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "foobar": no such pod`
		}
		Expect(session).To(ExitWithError(125, expect))
	})

	It("podman pod kill a pod by id", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "kill", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod kill a pod by id with TERM", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "kill", "-s", "9", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod kill a pod by name", func() {
		_, ec, podid := podmanTest.CreatePod(map[string][]string{"--name": {"test1"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "kill", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod kill a pod by id with a bogus signal", func() {
		_, ec, podid := podmanTest.CreatePod(map[string][]string{"--name": {"test1"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "kill", "-s", "bogus", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, "invalid signal: bogus"))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman pod kill latest pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, podid2 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid2)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("", podid2)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		if !IsRemote() {
			podid2 = "-l"
		}
		result := podmanTest.Podman([]string{"pod", "kill", podid2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman pod kill all", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, podid2 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", podid2)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "kill", "-a"})
		result.WaitWithDefaultTimeout()
		GinkgoWriter.Println(result.OutputToString(), result.ErrorToString())
		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})
})
