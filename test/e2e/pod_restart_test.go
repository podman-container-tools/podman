//go:build linux || freebsd

package integration

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pod restart", func() {
	It("podman pod restart bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "restart", "123"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID 123 found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "123": no such pod`
		}
		Expect(session).Should(ExitWithError(125, expect))
	})

	It("podman pod restart single empty pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "restart", podid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, fmt.Sprintf("no containers in pod %s have no dependencies, cannot start pod: no such container", podid)))
	})

	It("podman pod restart single pod by name", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		startTime.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"pod", "restart", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToString()).To(Not(Equal(startTime.OutputToString())))
	})

	It("podman pod restart multiple pods", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, _ = podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("test2", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("test3", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("test4", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2", "test3", "test4"})
		startTime.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"pod", "restart", "foobar99", "foobar100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2", "test3", "test4"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Not(Equal(startTime.OutputToStringArray()[0])))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
		Expect(restartTime.OutputToStringArray()[2]).To(Not(Equal(startTime.OutputToStringArray()[2])))
		Expect(restartTime.OutputToStringArray()[3]).To(Not(Equal(startTime.OutputToStringArray()[3])))
	})

	It("podman pod restart all pods", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, _ = podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("test2", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		session = podmanTest.Podman([]string{"pod", "restart", "-a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Not(Equal(startTime.OutputToStringArray()[0])))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("podman pod restart latest pod", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, _ = podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("test2", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		startTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		startTime.WaitWithDefaultTimeout()

		podid := "-l"
		if IsRemote() {
			podid = "foobar100"
		}
		session = podmanTest.Podman([]string{"pod", "restart", podid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		restartTime := podmanTest.Podman([]string{"inspect", "--format='{{.State.StartedAt}}'", "test1", "test2"})
		restartTime.WaitWithDefaultTimeout()
		Expect(restartTime.OutputToStringArray()[0]).To(Equal(startTime.OutputToStringArray()[0]))
		Expect(restartTime.OutputToStringArray()[1]).To(Not(Equal(startTime.OutputToStringArray()[1])))
	})

	It("podman pod restart multiple pods with bogus", func() {
		_, ec, podid1 := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "restart", podid1, "doesnotexist"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID doesnotexist found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "doesnotexist": no such pod`
		}
		Expect(session).Should(ExitWithError(125, expect))
	})
})
