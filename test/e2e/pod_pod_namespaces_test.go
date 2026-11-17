//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pod create", func() {
	It("podman pod container share Namespaces", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", podID, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.Namespaces.IPC}} {{.Namespaces.UTS}} {{.Namespaces.NET}}"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		outputArray := check.OutputToStringArray()
		Expect(outputArray).To(HaveLen(2))

		NAMESPACE1 := outputArray[0]
		GinkgoWriter.Println("NAMESPACE1:", NAMESPACE1)
		NAMESPACE2 := outputArray[1]
		GinkgoWriter.Println("NAMESPACE2:", NAMESPACE2)
		Expect(NAMESPACE1).To(Equal(NAMESPACE2))
	})

	It("podman pod container share ipc && /dev/shm ", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "--pod", podID, ALPINE, "touch", "/dev/shm/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "--pod", podID, ALPINE, "ls", "/dev/shm/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman pod container dontshare PIDNS", func() {
		session := podmanTest.Podman([]string{"pod", "create"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podID := session.OutputToString()

		session = podmanTest.Podman([]string{"pod", "start", podID})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", podID, "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"ps", "-a", "--ns", "--format", "{{.Namespaces.PIDNS}}"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		outputArray := check.OutputToStringArray()
		Expect(outputArray).To(HaveLen(2))

		NAMESPACE1 := outputArray[0]
		GinkgoWriter.Println("NAMESPACE1:", NAMESPACE1)
		NAMESPACE2 := outputArray[1]
		GinkgoWriter.Println("NAMESPACE2:", NAMESPACE2)
		Expect(NAMESPACE1).To(Not(Equal(NAMESPACE2)))
	})
})
