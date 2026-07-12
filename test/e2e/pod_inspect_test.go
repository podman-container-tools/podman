//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pod inspect", func() {
	It("podman inspect bogus pod", func() {
		session := podmanTest.Podman([]string{"pod", "inspect", "foobar"})
		session.WaitWithDefaultTimeout()
		expect := "no such pod foobar"
		if IsRemote() {
			expect = `no such pod "foobar"`
		}
		Expect(session).Should(ExitWithError(125, expect))
	})

	It("podman inspect a pod", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.RunTopContainerInPod("", podid)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"pod", "inspect", podid})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(BeValidJSON())
		podData := inspect.InspectPodToJSON()
		Expect(podData).To(HaveField("ID", podid))
	})

	It("podman pod inspect (CreateCommand)", func() {
		podName := "myTestPod"
		createCommand := []string{"pod", "create", "--name", podName, "--hostname", "rudolph", "--share", "net"}

		// Create the pod.
		session := podmanTest.Podman(createCommand)
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Inspect the pod and make sure that the create command is
		// exactly how we created the pod.
		inspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(BeValidJSON())
		podData := inspect.InspectPodToJSON()
		// Let's get the last len(createCommand) items in the command.
		inspectCreateCommand := podData.CreateCommand
		index := len(inspectCreateCommand) - len(createCommand)
		Expect(inspectCreateCommand[index:]).To(Equal(createCommand))
	})

	It("podman pod inspect outputs port bindings", func() {
		podName := "testPod"
		create := podmanTest.Podman([]string{"pod", "create", "--name", podName, "-p", "8383:80"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		inspectOut := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspectOut.WaitWithDefaultTimeout()
		Expect(inspectOut).Should(ExitCleanly())

		inspectJSON := inspectOut.InspectPodToJSON()
		Expect(inspectJSON.InfraConfig).To(Not(BeNil()))
		Expect(inspectJSON.InfraConfig.PortBindings["80/tcp"]).To(HaveLen(1))
		Expect(inspectJSON.InfraConfig.PortBindings["80/tcp"][0]).To(HaveField("HostPort", "8383"))
	})

	It("podman pod inspect outputs show correct MAC", func() {
		SkipIfRootless("--mac-address is not supported in rootless mode without network")
		podName := "testPod"
		macAddr := "42:43:44:00:00:01"
		create := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--mac-address", macAddr})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		create = podmanTest.Podman([]string{"run", "-d", "--pod", podName, ALPINE, "top"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		inspectOut := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspectOut.WaitWithDefaultTimeout()
		Expect(inspectOut).Should(ExitCleanly())

		Expect(inspectOut.OutputToString()).To(ContainSubstring(macAddr))
	})

	It("podman inspect two pods", func() {
		_, ec, podid1 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec, podid2 := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		inspect := podmanTest.Podman([]string{"pod", "inspect", podid1, podid2})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(BeValidJSON())
		podData := inspect.InspectPodArrToJSON()
		Expect(podData).To(HaveLen(2))
		Expect(podData[0]).To(HaveField("ID", podid1))
		Expect(podData[1]).To(HaveField("ID", podid2))
	})
})
