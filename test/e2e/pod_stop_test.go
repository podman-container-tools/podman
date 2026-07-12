//go:build linux || freebsd

package integration

import (
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pod stop", func() {
	It("podman pod stop bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "stop", "123"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID 123 found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "123": no such pod`
		}
		Expect(session).Should(ExitWithError(125, expect))
	})

	It("podman pod stop --ignore bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "stop", "--ignore", "123"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman stop bogus pod and a running pod", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID bogus found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "bogus": no such pod`
		}
		Expect(session).Should(ExitWithError(125, expect))
	})

	It("podman stop --ignore bogus pod and a running pod", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("test1", podid1)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", "--ignore", "bogus", "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", "-t", "-1", "--ignore", "test1", "bogus"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman pod stop single empty pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "stop", podid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman pod stop single pod by name", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", "foobar99"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod stop multiple pods", func() {
		_, ec, podid1 := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec2, podid2 := podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec2).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", podid1, podid2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod stop all pods", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, _ = podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod stop latest pod", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, _ = podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec).To(Equal(0))

		session = podmanTest.RunTopContainerInPod("", "foobar100")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		podid := "--latest"
		if IsRemote() {
			podid = "foobar100"
		}
		session = podmanTest.Podman([]string{"pod", "stop", podid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman pod stop multiple pods with bogus", func() {
		_, ec, podid1 := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", "foobar99")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "stop", podid1, "doesnotexist"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID doesnotexist found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "doesnotexist": no such pod`
		}
		Expect(session).Should(ExitWithError(125, expect))
	})

	It("podman pod start/stop single pod via --pod-id-file", func() {
		podIDFile := filepath.Join(tempdir, "podID")

		podName := "rudolph"

		// Create a pod with --pod-id-file.
		session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--pod-id-file", podIDFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Create container inside the pod.
		session = podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "start", "--pod-id-file", podIDFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2)) // infra+top

		session = podmanTest.Podman([]string{"pod", "stop", "--pod-id-file", podIDFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod start/stop multiple pods via --pod-id-file", func() {
		podIDFiles := []string{}
		for _, i := range "0123456789" {
			podIDFile := filepath.Join(tempdir, "cid"+string(i))
			podName := "rudolph" + string(i)
			// Create a pod with --pod-id-file.
			session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--pod-id-file", podIDFile})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// Create container inside the pod.
			session = podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// Append the id files along with the command.
			podIDFiles = append(podIDFiles, "--pod-id-file")
			podIDFiles = append(podIDFiles, podIDFile)
		}

		cmd := []string{"pod", "start"}
		cmd = append(cmd, podIDFiles...)
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(20)) // 10*(infra+top)

		cmd = []string{"pod", "stop"}
		cmd = append(cmd, podIDFiles...)
		session = podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(0))
	})

	It("podman pod stop orders container stop", func() {
		podName := "testpod"
		infraName := "testpod-infra"
		podmanTest.PodmanExitCleanly("pod", "create", "--infra-name", infraName, podName)

		ctrName := "testctr"
		podmanTest.PodmanExitCleanly("run", "-d", "--name", ctrName, "--pod", podName, ALPINE, "top")

		podmanTest.PodmanExitCleanly("pod", "stop", podName)

		ctrStop := podmanTest.PodmanExitCleanly("inspect", "--format", "{{ .State.FinishedAt }}", ctrName)
		infraStop := podmanTest.PodmanExitCleanly("inspect", "--format", "{{ .State.FinishedAt }}", infraName)

		timeFormat := "2006-01-02 15:04:05.999999999 -0700 MST"

		ctrStopTime, err := time.Parse(timeFormat, ctrStop.OutputToString())
		Expect(err).ShouldNot(HaveOccurred())
		infraStopTime, err := time.Parse(timeFormat, infraStop.OutputToString())
		Expect(err).ShouldNot(HaveOccurred())

		Expect(infraStopTime).To(BeTemporally(">", ctrStopTime))
	})
})
