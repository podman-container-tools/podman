//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman volume rename", func() {
	AfterEach(func() {
		podmanTest.CleanupVolume()
	})

	It("podman volume rename", func() {
		podmanTest.PodmanExitCleanly("volume", "create", "myvol")

		rename := podmanTest.PodmanExitCleanly("volume", "rename", "myvol", "newvol")
		Expect(rename.OutputToString()).To(BeEmpty())

		check := podmanTest.PodmanExitCleanly("volume", "inspect", "newvol")
		Expect(check.OutputToString()).To(ContainSubstring("newvol"))

		// Old name should no longer exist
		check = podmanTest.Podman([]string{"volume", "inspect", "myvol"})
		check.WaitWithDefaultTimeout()
		Expect(check).To(ExitWithError(125, "no such volume"))
	})

	It("podman volume rename data persists", func() {
		podmanTest.PodmanExitCleanly("volume", "create", "myvol")

		podmanTest.PodmanExitCleanly("run", "--rm", "--network=none", "-v", "myvol:/data", ALPINE, "sh", "-c", "echo hello > /data/testfile")

		podmanTest.PodmanExitCleanly("volume", "rename", "myvol", "newvol")

		session := podmanTest.PodmanExitCleanly("run", "--rm", "--network=none", "-v", "newvol:/data", ALPINE, "cat", "/data/testfile")
		Expect(session.OutputToString()).To(Equal("hello"))
	})

	It("podman volume rename fails when used by a stopped container", func() {
		podmanTest.PodmanExitCleanly("volume", "create", "myvol")

		podmanTest.PodmanExitCleanly("create", "-v", "myvol:/data", ALPINE, "true")

		session := podmanTest.Podman([]string{"volume", "rename", "myvol", "newvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "volume is being used"))
	})

	It("podman volume rename fails when used by a running container", func() {
		podmanTest.PodmanExitCleanly("volume", "create", "myvol")
		podmanTest.PodmanExitCleanly("run", "-d", "--network=none", "-v", "myvol:/data", ALPINE, "top")

		session := podmanTest.Podman([]string{"volume", "rename", "myvol", "newvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "volume is being used"))
	})

	It("podman volume rename handles error cases", func() {
		podmanTest.PodmanExitCleanly("volume", "create", "vol1")
		podmanTest.PodmanExitCleanly("volume", "create", "vol2")

		session := podmanTest.Podman([]string{"volume", "rename", "vol1", "vol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "volume already exists"))

		session = podmanTest.Podman([]string{"volume", "rename", "nosuchvol", "newvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "no such volume"))

		session = podmanTest.Podman([]string{"volume", "rename", "vol1", "invalid/name"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "invalid argument"))

		session = podmanTest.Podman([]string{"volume", "rename", "vol1", " newvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "names must match"))

		podmanTest.AddImageToRWStore(FEDORA_MINIMAL)
		podmanTest.PodmanExitCleanly("volume", "create", "--driver", "image", "--opt", "image="+FEDORA_MINIMAL, "imagevol")

		session = podmanTest.Podman([]string{"volume", "rename", "imagevol", "newvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "rename is not supported for volumes using driver \"image\""))
	})

	It("podman volume rename to same name succeeds", func() {
		podmanTest.PodmanExitCleanly("volume", "create", "myvol")

		rename := podmanTest.PodmanExitCleanly("volume", "rename", "myvol", "myvol")
		Expect(rename.OutputToString()).To(BeEmpty())

		inspect := podmanTest.PodmanExitCleanly("volume", "inspect", "myvol")
		Expect(inspect.OutputToString()).To(ContainSubstring("myvol"))
	})

	It("podman volume rename converts anonymous volumes to named volumes", func() {
		ctr := podmanTest.PodmanExitCleanly("create", "-v", "/data", ALPINE, "true")
		volumes := podmanTest.PodmanExitCleanly("volume", "list", "--quiet")
		volumeNames := volumes.OutputToStringArray()
		Expect(volumeNames).To(HaveLen(1))

		podmanTest.PodmanExitCleanly("rm", ctr.OutputToString())
		podmanTest.PodmanExitCleanly("volume", "rename", volumeNames[0], "namedvol")

		inspect := podmanTest.PodmanExitCleanly("volume", "inspect", "--format", "{{.Anonymous}}", "namedvol")
		Expect(inspect.OutputToString()).To(Equal("false"))
	})

	It("podman volume rename requires exactly 2 args", func() {
		session := podmanTest.Podman([]string{"volume", "rename", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "accepts 2 arg(s)"))
	})
})
