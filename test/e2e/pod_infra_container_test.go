//go:build linux || freebsd

package integration

import (
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pod create", func() {
	It("podman create infra container", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		check := podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring(podID))
		Expect(check.OutputToStringArray()).To(HaveLen(1))

		check = podmanTest.Podman([]string{"ps", "-qa", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman start infra container", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"ps", "-qa", "--no-trunc", "--filter", "status=running"})
		check.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(check.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman start infra container different image", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--infra-image", BB})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		// If we use the default entry point, we should exit with no error
		Expect(session).Should(ExitCleanly())
	})

	It("podman infra container namespaces", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("", podID)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"ps", "-a", "--no-trunc", "--ns", "--format", "{{.Namespaces.IPC}} {{.Namespaces.NET}}"})
		check.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(check.OutputToStringArray()).To(HaveLen(2))
		Expect(check.OutputToStringArray()[0]).To(Equal(check.OutputToStringArray()[1]))

		check = podmanTest.Podman([]string{"ps", "-a", "--no-trunc", "--ns", "--format", "{{.IPC}} {{.NET}}"})
		check.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(check.OutputToStringArray()).To(HaveLen(2))
		Expect(check.OutputToStringArray()[0]).To(Equal(check.OutputToStringArray()[1]))
	})

	It("podman pod correctly sets up NetNS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "net"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-d", "--pod", podID, NGINX_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", podID, FEDORA_MINIMAL, "curl", "-s", "--retry", "2", "--retry-connrefused", "-f", "localhost:80"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", FEDORA_MINIMAL, "curl", "-f", "localhost"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(7, "Failed to connect to localhost port 80 "))

		session = podmanTest.Podman([]string{"pod", "create", "--network", "host"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--name", "hostCtr", "--pod", session.OutputToString(), ALPINE, "readlink", "/proc/self/ns/net"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		ns := SystemExec("readlink", []string{"/proc/self/ns/net"})
		ns.WaitWithDefaultTimeout()
		Expect(ns).Should(ExitCleanly())
		netns := ns.OutputToString()
		Expect(netns).ToNot(BeEmpty())

		Expect(session.OutputToString()).To(Equal(netns))

		// Sanity Check for podman inspect
		session = podmanTest.Podman([]string{"inspect", "--format", "'{{.NetworkSettings.SandboxKey}}'", "hostCtr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("''")) // no network path... host
	})

	It("podman pod correctly sets up IPCNS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "ipc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", podID, FEDORA_MINIMAL, "/bin/sh", "-c", "'touch /dev/shm/hi'"})
		session.WaitWithDefaultTimeout()
		if session.ExitCode() != 0 {
			Skip("ShmDir not initialized, skipping...")
		}
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", podID, FEDORA_MINIMAL, "/bin/sh", "-c", "'ls /dev/shm'"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("hi"))
	})

	It("podman pod correctly sets up PIDNS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "pid", "--name", "test-pod"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("test-ctr", podID)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"top", "test-ctr", "pid"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		PIDs := check.OutputToStringArray()
		Expect(PIDs).To(HaveLen(3))

		ctrPID, _ := strconv.Atoi(PIDs[1])
		infraPID, _ := strconv.Atoi(PIDs[2])
		Expect(ctrPID).To(BeNumerically("<", infraPID))
	})

	It("podman pod doesn't share PIDNS if requested to not", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "net", "--name", "test-pod"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("test-ctr", podID)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"top", "test-ctr", "pid"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		ctrTop := check.OutputToStringArray()

		check = podmanTest.Podman([]string{"top", podID[:12] + "-infra", "pid"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		infraTop := check.OutputToStringArray()

		ctrPID, _ := strconv.Atoi(ctrTop[1])
		infraPID, _ := strconv.Atoi(infraTop[1])
		Expect(ctrPID).To(Equal(infraPID))
	})

	It("podman pod container can override pod net NS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "net"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "-d", "--pod", podID, NGINX_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", podID, "--network", "bridge", NGINX_IMAGE, "curl", "-f", "localhost"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(7, "Failed to connect to localhost port 80 "))
	})

	It("podman pod container can override pod pid NS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "pid"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", podID, "--pid", "host", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.Namespaces.PIDNS}}"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		outputArray := check.OutputToStringArray()
		Expect(outputArray).To(HaveLen(2))

		check = podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.PIDNS}}"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		outputArray = check.OutputToStringArray()
		Expect(outputArray).To(HaveLen(2))

		PID1 := outputArray[0]
		PID2 := outputArray[1]
		Expect(PID1).To(Not(Equal(PID2)))
	})

	It("podman pod container can override pod not sharing pid", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "net"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", podID, "--pid", "pod", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.PIDNS}}"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		outputArray := check.OutputToStringArray()
		Expect(outputArray).To(HaveLen(2))

		PID1 := outputArray[0]
		PID2 := outputArray[1]
		Expect(PID1).To(Equal(PID2))
	})

	It("podman pod container can override pod ipc NS", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "ipc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", podID, "--ipc", "host", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.IPC}}"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		outputArray := check.OutputToStringArray()
		Expect(outputArray).To(HaveLen(2))

		PID1 := outputArray[0]
		PID2 := outputArray[1]
		Expect(PID1).To(Not(Equal(PID2)))
	})

	It("podman pod infra container deletion", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--share", "ipc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"ps", "-aq", "--no-trunc"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		infraID := session.OutputToString()

		session = podmanTest.Podman([]string{"rm", infraID})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, fmt.Sprintf("container %s is the infra container of pod %s and cannot be removed without removing the pod", infraID, podID)))

		session = podmanTest.Podman([]string{"pod", "rm", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run in pod starts infra", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		result := podmanTest.Podman([]string{"ps", "-aq"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		infraID := result.OutputToString()

		result = podmanTest.Podman([]string{"run", "--pod", podID, "-d", ALPINE, "top"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"ps", "-aq"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())

		Expect(result.OutputToString()).To(ContainSubstring(infraID))
	})

	It("podman start in pod starts infra", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		result := podmanTest.Podman([]string{"ps", "-aq"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		infraID := result.OutputToString()

		result = podmanTest.Podman([]string{"create", "--pod", podID, ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		ctrID := result.OutputToString()

		result = podmanTest.Podman([]string{"start", ctrID})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"ps", "-aq"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToStringArray()).ShouldNot(BeEmpty())

		Expect(result.OutputToString()).To(ContainSubstring(infraID))
	})

	It("podman run --add-host in pod should fail", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--add-host", "host1:127.0.0.1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"create", "--pod", podID, "--add-host", "foobar:127.0.0.1", ALPINE, "ping", "-c", "1", "foobar"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "extra host entries must be specified on the pod: network cannot be configured when it is shared with a pod"))

		// verify we can see the pods hosts
		session = podmanTest.Podman([]string{"run", "--cap-add", "net_raw", "--pod", podID, ALPINE, "ping", "-c", "1", "host1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run hostname is shared", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		// verify we can add a host to the infra's /etc/hosts
		session = podmanTest.Podman([]string{"run", "--pod", podID, ALPINE, "hostname"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		hostname := session.OutputToString()

		infraName := podID[:12] + "-infra"
		// verify we can see the other hosts of infra's /etc/hosts
		session = podmanTest.Podman([]string{"inspect", infraName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(hostname))
	})

	tests := []string{"", "none"}
	for _, test := range tests {
		It("podman pod create --share="+test+" should not create an infra ctr", func() {
			session := podmanTest.Podman([]string{"pod", "create", "--share", test})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())

			session = podmanTest.Podman([]string{"pod", "inspect", "--format", "{{.NumContainers}}", session.OutputToString()})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).Should(Equal("0"))
		})
	}
})
