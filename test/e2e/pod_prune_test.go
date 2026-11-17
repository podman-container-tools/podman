//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman pod prune", func() {
	It("podman pod prune empty pod", func() {
		_, ec, _ := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "prune", "--force"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman pod prune doesn't remove a pod with a running container", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		ec2 := podmanTest.RunTopContainerInPod("", podid)
		ec2.WaitWithDefaultTimeout()
		Expect(ec2).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"pod", "prune", "-f"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"ps", "-qa"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman pod prune removes a pod with a stopped container", func() {
		_, ec, podid := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		_, ec2, _ := podmanTest.RunLsContainerInPod("", podid)
		Expect(ec2).To(Equal(0))

		result := podmanTest.Podman([]string{"pod", "prune", "-f"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"ps", "-qa"})
		result.WaitWithDefaultTimeout()
		Expect(result.OutputToStringArray()).To(BeEmpty())
	})
})
