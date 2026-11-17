//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman mount", func() {
	BeforeEach(func() {
		SkipIfRemote("Podman mount not supported for remote connections")
		SkipIfRootless("Podman mount requires podman unshare first to work")
		podmanTest.AddImageToRWStore(ALPINE)
	})

	It("podman mount", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid := setup.OutputToString()

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		umount := podmanTest.Podman([]string{"umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())
	})

	It("podman container mount", func() {
		setup := podmanTest.Podman([]string{"container", "create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid := setup.OutputToString()

		mount := podmanTest.Podman([]string{"container", "mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		umount := podmanTest.Podman([]string{"container", "umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())
	})

	It("podman mount with json format", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid := setup.OutputToString()

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		j := podmanTest.Podman([]string{"mount", "--format=json"})
		j.WaitWithDefaultTimeout()
		Expect(j).Should(ExitCleanly())
		Expect(j.OutputToString()).To(BeValidJSON())

		j = podmanTest.Podman([]string{"mount", "--format={{.foobar}}"})
		j.WaitWithDefaultTimeout()
		Expect(j).To(ExitWithError(125, `unknown --format argument: "{{.foobar}}"`))

		umount := podmanTest.Podman([]string{"umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())
	})

	It("podman mount many", func() {
		setup1 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup1.WaitWithDefaultTimeout()
		Expect(setup1).Should(ExitCleanly())
		cid1 := setup1.OutputToString()

		setup2 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup2.WaitWithDefaultTimeout()
		Expect(setup2).Should(ExitCleanly())
		cid2 := setup2.OutputToString()

		setup3 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup3.WaitWithDefaultTimeout()
		Expect(setup3).Should(ExitCleanly())
		cid3 := setup3.OutputToString()

		mount1 := podmanTest.Podman([]string{"mount", cid1, cid2, cid3})
		mount1.WaitWithDefaultTimeout()
		Expect(mount1).Should(ExitCleanly())

		umount := podmanTest.Podman([]string{"umount", cid1, cid2, cid3})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())
	})

	It("podman umount many", func() {
		setup1 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup1.WaitWithDefaultTimeout()
		Expect(setup1).Should(ExitCleanly())
		cid1 := setup1.OutputToString()

		setup2 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup2.WaitWithDefaultTimeout()
		Expect(setup2).Should(ExitCleanly())
		cid2 := setup2.OutputToString()

		mount1 := podmanTest.Podman([]string{"mount", cid1})
		mount1.WaitWithDefaultTimeout()
		Expect(mount1).Should(ExitCleanly())

		mount2 := podmanTest.Podman([]string{"mount", cid2})
		mount2.WaitWithDefaultTimeout()
		Expect(mount2).Should(ExitCleanly())

		umount := podmanTest.Podman([]string{"umount", cid1, cid2})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())
	})

	It("podman umount all", func() {
		setup1 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup1.WaitWithDefaultTimeout()
		Expect(setup1).Should(ExitCleanly())
		cid1 := setup1.OutputToString()

		setup2 := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup2.WaitWithDefaultTimeout()
		Expect(setup2).Should(ExitCleanly())
		cid2 := setup2.OutputToString()

		mount1 := podmanTest.Podman([]string{"mount", cid1})
		mount1.WaitWithDefaultTimeout()
		Expect(mount1).Should(ExitCleanly())

		mount2 := podmanTest.Podman([]string{"mount", cid2})
		mount2.WaitWithDefaultTimeout()
		Expect(mount2).Should(ExitCleanly())

		umount := podmanTest.Podman([]string{"umount", "--all"})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())
	})

	It("podman list mounted container", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid := setup.OutputToString()

		lmount := podmanTest.Podman([]string{"mount", "--no-trunc"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount).Should(ExitCleanly())
		Expect(lmount.OutputToString()).To(Equal(""))

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		lmount = podmanTest.Podman([]string{"mount", "--no-trunc"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount).Should(ExitCleanly())
		Expect(lmount.OutputToString()).To(ContainSubstring(cid))

		umount := podmanTest.Podman([]string{"umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())
	})

	It("podman list running container", func() {
		setup := podmanTest.Podman([]string{"run", "-dt", ALPINE, "top"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid := setup.OutputToString()

		lmount := podmanTest.Podman([]string{"mount", "--no-trunc"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount).Should(ExitCleanly())
		Expect(lmount.OutputToString()).To(ContainSubstring(cid))

		podmanTest.StopContainer(cid)

		lmount = podmanTest.Podman([]string{"mount", "--no-trunc"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount).Should(ExitCleanly())
		Expect(lmount.OutputToString()).To(Equal(""))
	})

	It("podman list multiple mounted containers", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid1 := setup.OutputToString()

		setup = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid2 := setup.OutputToString()

		setup = podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid3 := setup.OutputToString()

		lmount := podmanTest.Podman([]string{"mount", "--no-trunc"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount).Should(ExitCleanly())
		Expect(lmount.OutputToString()).To(Equal(""))

		mount := podmanTest.Podman([]string{"mount", cid1, cid3})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		lmount = podmanTest.Podman([]string{"mount", "--no-trunc"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount).Should(ExitCleanly())
		Expect(lmount.OutputToString()).To(ContainSubstring(cid1))
		Expect(lmount.OutputToString()).ToNot(ContainSubstring(cid2))
		Expect(lmount.OutputToString()).To(ContainSubstring(cid3))

		umount := podmanTest.Podman([]string{"umount", cid1, cid3})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())

		lmount = podmanTest.Podman([]string{"mount", "--no-trunc"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount).Should(ExitCleanly())
		Expect(lmount.OutputToString()).To(Equal(""))
	})

	It("podman list mounted container", func() {
		setup := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())
		cid := setup.OutputToString()

		lmount := podmanTest.Podman([]string{"mount", "--no-trunc"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount).Should(ExitCleanly())
		Expect(lmount.OutputToString()).To(Equal(""))

		mount := podmanTest.Podman([]string{"mount", cid})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		// test --notruncate alias
		lmount = podmanTest.Podman([]string{"mount", "--notruncate"})
		lmount.WaitWithDefaultTimeout()
		Expect(lmount).Should(ExitCleanly())
		Expect(lmount.OutputToString()).To(ContainSubstring(cid))

		umount := podmanTest.Podman([]string{"umount", cid})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())
	})

	It("podman image mount", func() {
		images := podmanTest.Podman([]string{"images"})
		images.WaitWithDefaultTimeout()
		Expect(images).Should(ExitCleanly())

		mount := podmanTest.Podman([]string{"image", "mount", ALPINE})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		umount := podmanTest.Podman([]string{"image", "umount", ALPINE})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())

		mount = podmanTest.Podman([]string{"image", "mount"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).To(Equal(""))

		// Mount multiple times
		mount = podmanTest.Podman([]string{"image", "mount", ALPINE})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		mount = podmanTest.Podman([]string{"image", "mount", ALPINE})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		// Unmount once
		mount = podmanTest.Podman([]string{"image", "mount", ALPINE})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		mount = podmanTest.Podman([]string{"image", "mount"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).To(ContainSubstring(ALPINE))

		mount = podmanTest.Podman([]string{"image", "umount", "--all"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
	})

	It("podman mount with json format", func() {
		podmanTest.AddImageToRWStore(FEDORA_MINIMAL)
		mount := podmanTest.Podman([]string{"image", "mount", FEDORA_MINIMAL})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		j := podmanTest.Podman([]string{"image", "mount", "--format=json"})
		j.WaitWithDefaultTimeout()
		Expect(j).Should(ExitCleanly())
		Expect(j.OutputToString()).To(BeValidJSON())

		umount := podmanTest.Podman([]string{"image", "umount", FEDORA_MINIMAL})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())
	})

	It("podman umount --all", func() {
		podmanTest.AddImageToRWStore(FEDORA_MINIMAL)
		mount := podmanTest.Podman([]string{"image", "mount", FEDORA_MINIMAL})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())

		umount := podmanTest.Podman([]string{"image", "umount", "--all"})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())
		Expect(umount.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman mount many", func() {
		Skip("Issue where using short name when we have a lookaside store")
		podmanTest.AddImageToRWStore(FEDORA_MINIMAL)
		podmanTest.AddImageToRWStore(BB)

		mount1 := podmanTest.Podman([]string{"image", "mount", FEDORA_MINIMAL, ALPINE, "busybox"})
		mount1.WaitWithDefaultTimeout()
		Expect(mount1).Should(ExitCleanly())

		umount := podmanTest.Podman([]string{"image", "umount", FEDORA_MINIMAL, ALPINE})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())

		mount := podmanTest.Podman([]string{"image", "mount"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).To(ContainSubstring("busybox"))

		mount1 = podmanTest.Podman([]string{"image", "unmount", "busybox"})
		mount1.WaitWithDefaultTimeout()
		Expect(mount1).Should(ExitCleanly())

		mount = podmanTest.Podman([]string{"image", "mount"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).To(Equal(""))

		mount1 = podmanTest.Podman([]string{"image", "mount", FEDORA_MINIMAL, ALPINE, "busybox"})
		mount1.WaitWithDefaultTimeout()
		Expect(mount1).Should(ExitCleanly())

		mount = podmanTest.Podman([]string{"image", "mount"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).To(ContainSubstring(FEDORA_MINIMAL))
		Expect(mount.OutputToString()).To(ContainSubstring(ALPINE))

		umount = podmanTest.Podman([]string{"image", "umount", "--all"})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())

		mount = podmanTest.Podman([]string{"image", "mount"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).To(Equal(""))

		umount = podmanTest.Podman([]string{"image", "umount", FEDORA_MINIMAL, ALPINE})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())

		mount1 = podmanTest.Podman([]string{"image", "mount", "--all"})
		mount1.WaitWithDefaultTimeout()
		Expect(mount1).Should(ExitCleanly())

		mount = podmanTest.Podman([]string{"image", "mount"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).To(ContainSubstring(FEDORA_MINIMAL))
		Expect(mount.OutputToString()).To(ContainSubstring(ALPINE))

		umount = podmanTest.Podman([]string{"image", "umount", "--all"})
		umount.WaitWithDefaultTimeout()
		Expect(umount).Should(ExitCleanly())

		mount = podmanTest.Podman([]string{"image", "mount"})
		mount.WaitWithDefaultTimeout()
		Expect(mount).Should(ExitCleanly())
		Expect(mount.OutputToString()).To(Equal(""))
	})
})
