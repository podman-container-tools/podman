//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pod pause", func() {
	pausedState := "Paused"

	It("podman pod pause bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "pause", "foobar"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID foobar found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "foobar": no such pod`
		}
		Expect(session).To(ExitWithError(125, expect))
	})

	It("podman unpause bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "unpause", "foobar"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID foobar found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "foobar": no such pod`
		}
		Expect(session).To(ExitWithError(125, expect))
	})

	It("podman pod pause a created pod by id", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "pause", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("pause a running pod by id", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "pause", podid})
		result.WaitWithDefaultTimeout()

		Expect(result).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"pod", "unpause", podid})
		result.WaitWithDefaultTimeout()
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("unpause a paused pod by id", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "pause", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(Equal(pausedState))

		result = podmanTest.Podman([]string{"pod", "unpause", podid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("unpause a paused pod by name", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"test1"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "test1")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "pause", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
		Expect(podmanTest.GetContainerStatus()).To(Equal(pausedState))

		result = podmanTest.Podman([]string{"pod", "unpause", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("unpause --all", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))
		_, ec, podid2 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.RunTopContainerInPod("", podid2)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "pause", podid1})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
		Expect(podmanTest.GetContainerStatus()).To(ContainSubstring(pausedState))

		result = podmanTest.Podman([]string{"pod", "unpause", "--all"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).Should(HaveLen(1))
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})
})
