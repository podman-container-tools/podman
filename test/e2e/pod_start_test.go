//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pod start", func() {
	It("podman pod start bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "start", "123"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID 123 found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "123": no such pod`
		}
		Expect(session).Should(ExitWithError(125, expect))
	})

	It("podman pod start single empty pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"pod", "start", podid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, fmt.Sprintf("no containers in pod %s have no dependencies, cannot start pod: no such container", podid)))
	})

	It("podman pod start single pod by name", func() {
		name := "foobar99"
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}})
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", name, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "start", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring(name))
	})

	It("podman pod start multiple pods", func() {
		_, ec, podid1 := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec2, podid2 := podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec2).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "start", podid1, podid2})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
		Expect(session.OutputToString()).Should(ContainSubstring("foobar99"))
		Expect(session.OutputToString()).Should(ContainSubstring("foobar100"))
	})

	It("multiple pods in conflict", func() {
		podName := []string{"Pod_" + RandomString(10), "Pod_" + RandomString(10)}

		pod, _, podid1 := podmanTest.CreatePod(map[string][]string{
			"--infra":   {"true"},
			"--name":    {podName[0]},
			"--publish": {"127.0.0.1:8083:80"},
		})
		Expect(pod).To(ExitCleanly())

		session := podmanTest.Podman([]string{"create", "--pod", podName[0], ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		pod, _, podid2 := podmanTest.CreatePod(map[string][]string{
			"--infra":   {"true"},
			"--name":    {podName[1]},
			"--publish": {"127.0.0.1:8083:80"},
		})
		Expect(pod).To(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", podName[1], ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "start", podid1, podid2})
		session.WaitWithDefaultTimeout()
		// Different network backends emit different messages; check only the common part
		Expect(session).To(ExitWithError(125, "ddress already in use"))
	})

	It("podman pod start all pods", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, _ = podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "start", "--all"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(2))
	})

	It("podman pod start latest pod", func() {
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		_, ec, _ = podmanTest.CreatePod(map[string][]string{"--name": {"foobar100"}})
		Expect(ec).To(Equal(0))

		session = podmanTest.Podman([]string{"create", "--pod", "foobar100", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		podid := "--latest"
		if IsRemote() {
			podid = "foobar100"
		}
		session = podmanTest.Podman([]string{"pod", "start", podid})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1))
	})

	It("podman pod start multiple pods with bogus", func() {
		_, ec, podid := podmanTest.CreatePod(map[string][]string{"--name": {"foobar99"}})
		Expect(ec).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", "foobar99", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "start", podid, "doesnotexist"})
		session.WaitWithDefaultTimeout()
		expect := "no pod with name or ID doesnotexist found: no such pod"
		if IsRemote() {
			expect = `unable to find pod "doesnotexist": no such pod`
		}
		Expect(session).Should(ExitWithError(125, expect))
	})

	It("podman pod start single pod via --pod-id-file", func() {
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
	})

	It("podman pod start multiple pods via --pod-id-file", func() {
		podIDFiles := []string{}
		for _, i := range "0123456789" {
			cidFile := filepath.Join(tempdir, "cid"+string(i))
			podName := "rudolph" + string(i)
			// Create a pod with --pod-id-file.
			session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--pod-id-file", cidFile})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// Create container inside the pod.
			session = podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			// Append the id files along with the command.
			podIDFiles = append(podIDFiles, "--pod-id-file")
			podIDFiles = append(podIDFiles, cidFile)
		}

		cmd := []string{"pod", "start"}
		cmd = append(cmd, podIDFiles...)
		session := podmanTest.Podman(cmd)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(20)) // 10*(infra+top)
	})

	It("podman pod create --infra-conmon-pod create + start", func() {
		pidFile := filepath.Join(tempdir, "podID")

		podName := "rudolph"
		// Create a pod with --infra-conmon-pid.
		session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--infra-conmon-pidfile", pidFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "start", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainersRunning()).To(Equal(1)) // infra

		readFirstLine := func(path string) string {
			content, err := os.ReadFile(path)
			Expect(err).ToNot(HaveOccurred())
			return strings.Split(string(content), "\n")[0]
		}

		// Read the infra-conmon-pidfile and perform some sanity checks
		// on the pid.
		infraConmonPID := readFirstLine(pidFile)
		_, err = strconv.Atoi(infraConmonPID) // Make sure it's a proper integer
		Expect(err).ToNot(HaveOccurred())

		cmdline := readFirstLine(fmt.Sprintf("/proc/%s/cmdline", infraConmonPID))
		Expect(cmdline).To(ContainSubstring("/conmon"))
	})
})
