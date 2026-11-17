//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman run exit", func() {
	It("podman run -d mount cleanup test", func() {
		SkipIfRemote("podman-remote does not support mount")
		SkipIfRootless("rootless podman mount requires podman unshare first")

		result := podmanTest.Podman([]string{"run", "-dt", ALPINE, "top"})
		result.WaitWithDefaultTimeout()
		cid := result.OutputToString()
		Expect(result).Should(ExitCleanly())

		mount := SystemExec("mount", nil)
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).To(ContainSubstring(cid))

		pmount := podmanTest.Podman([]string{"mount", "--no-trunc"})
		pmount.WaitWithDefaultTimeout()
		Expect(pmount).Should(ExitCleanly())
		Expect(pmount.OutputToString()).To(ContainSubstring(cid))

		podmanTest.StopContainer(cid)

		// We have to force cleanup so the unmount happens
		podmanCleanupSession := podmanTest.Podman([]string{"container", "cleanup", cid})
		podmanCleanupSession.WaitWithDefaultTimeout()
		Expect(podmanCleanupSession).Should(ExitCleanly())

		mount = SystemExec("mount", nil)
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).NotTo(ContainSubstring(cid))

		pmount = podmanTest.Podman([]string{"mount", "--no-trunc"})
		pmount.WaitWithDefaultTimeout()
		Expect(pmount).Should(ExitCleanly())
		Expect(pmount.OutputToString()).NotTo(ContainSubstring(cid))
	})

	It("podman run -d mount cleanup rootless test", func() {
		SkipIfRemote("podman-remote does not support mount")
		SkipIfNotRootless("Use unshare in rootless only")

		result := podmanTest.Podman([]string{"run", "-dt", ALPINE, "top"})
		result.WaitWithDefaultTimeout()
		cid := result.OutputToString()
		Expect(result).Should(ExitCleanly())

		mount := podmanTest.Podman([]string{"unshare", "mount"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).To(ContainSubstring(cid))

		// command: podman <options> unshare podman <options> image mount ALPINE
		args := []string{"unshare", podmanTest.PodmanBinary}
		opts := podmanTest.PodmanMakeOptions([]string{"mount", "--no-trunc"}, PodmanExecOptions{})
		args = append(args, opts...)

		pmount := podmanTest.Podman(args)
		pmount.WaitWithDefaultTimeout()
		Expect(pmount).Should(ExitCleanly())
		Expect(pmount.OutputToString()).To(ContainSubstring(cid))

		podmanTest.StopContainer(cid)

		// We have to force cleanup so the unmount happens
		podmanCleanupSession := podmanTest.Podman([]string{"container", "cleanup", cid})
		podmanCleanupSession.WaitWithDefaultTimeout()
		Expect(podmanCleanupSession).Should(ExitCleanly())

		mount = podmanTest.Podman([]string{"unshare", "mount"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).NotTo(ContainSubstring(cid))

		pmount = podmanTest.Podman(args)
		pmount.WaitWithDefaultTimeout()
		Expect(pmount).Should(ExitCleanly())
		Expect(pmount.OutputToString()).NotTo(ContainSubstring(cid))
	})
})
