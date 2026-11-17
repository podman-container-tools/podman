//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pod stats", func() {
	It("podman pod stats should run with no pods", func() {
		session := podmanTest.Podman([]string{"pod", "stats", "--no-stream"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman pod stats with a bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "stats", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "unable to get list of pods: no pod with name or ID foobar found: no such pod"))
	})

	It("podman pod stats on a specific running pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", podid})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())
	})

	It("podman pod stats on a specific running pod with shortID", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", podid[:5]})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())
	})

	It("podman pod stats on a specific running pod with name", func() {
		_, ec, podid := podmanTest.CreatePod(map[string][]string{"--name": {"test"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", "test"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())
	})

	It("podman pod stats on running pods", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())
	})

	It("podman pod stats on all pods", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		stats := podmanTest.Podman([]string{"pod", "stats", "--no-stream", "-a"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())
	})

	It("podman pod stats with json output", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		stats := podmanTest.Podman([]string{"pod", "stats", "--format", "json", "--no-stream", "-a"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())
		Expect(stats.OutputToString()).To(BeValidJSON())
	})
	It("podman pod stats with GO template", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		stats := podmanTest.Podman([]string{"pod", "stats", "-a", "--no-reset", "--no-stream", "--format", "table {{.CID}} {{.Pod}} {{.Mem}} {{.MemUsage}} {{.CPU}} {{.NetIO}} {{.BlockIO}} {{.PIDS}} {{.Pod}}"})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(ExitCleanly())
	})

	It("podman pod stats with invalid GO template", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		stats := podmanTest.Podman([]string{"pod", "stats", "-a", "--no-reset", "--no-stream", "--format", "\"table {{.ID}} \""})
		stats.WaitWithDefaultTimeout()
		Expect(stats).To(ExitWithError(125, `template: stats:1:20: executing "stats" at <.ID>: can't evaluate field ID in type *types.PodStatsReport`))
	})

	It("podman pod stats on net=host post", func() {
		SkipIfRootless("--net=host not supported for rootless pods at present")
		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--net=host", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		ctrRun := podmanTest.Podman([]string{"run", "-d", "--pod", podName, ALPINE, "top"})
		ctrRun.WaitWithDefaultTimeout()
		Expect(ctrRun).Should(ExitCleanly())

		stats := podmanTest.Podman([]string{"pod", "stats", "--format", "json", "--no-stream", podName})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())
		Expect(stats.OutputToString()).To(BeValidJSON())
	})
})
