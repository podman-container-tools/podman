//go:build linux || freebsd

package integration

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman volume create", func() {
	AfterEach(func() {
		podmanTest.CleanupVolume()
	})

	It("podman create volume", func() {
		session := podmanTest.Podman([]string{"volume", "create"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"volume", "ls", "-q"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring(volName))
		Expect(check.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman create volume with name", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"volume", "ls", "-q"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring(volName))
		Expect(check.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman create volume with existing name fails", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "volume with name myvol already exists: volume already exists"))
	})

	It("podman create volume --ignore", func() {
		session := podmanTest.Podman([]string{"volume", "create", "myvol"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "--ignore", "myvol"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(volName))
	})

	It("podman create and export volume", func() {
		volName := "my_vol_" + RandomString(10)
		session := podmanTest.Podman([]string{"volume", "create", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(volName))

		helloString := "hello-" + RandomString(20)
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data", ALPINE, "sh", "-c", "echo " + helloString + " >> /data/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// export to tar file...
		helloTar := filepath.Join(podmanTest.TempDir, "hello.tar")
		check := podmanTest.Podman([]string{"volume", "export", "-o", helloTar, volName})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())

		// ...then confirm that tar file has our desired content.
		// These flags emit filename to stderr (-v), contents to stdout
		tar := SystemExec("tar", []string{"-x", "-v", "--to-stdout", "-f", helloTar})
		tar.WaitWithDefaultTimeout()
		Expect(tar).To(Exit(0))
		Expect(tar.ErrorToString()).To(Equal("test"))
		Expect(tar.OutputToString()).To(Equal(helloString))
	})

	It("podman create and import volume", func() {
		volName := "my_vol_" + RandomString(10)
		session := podmanTest.Podman([]string{"volume", "create", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(volName))

		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data", ALPINE, "sh", "-c", "echo hello >> /data/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		helloTar := filepath.Join(podmanTest.TempDir, "hello.tar")
		session = podmanTest.Podman([]string{"volume", "export", volName, "--output", helloTar})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "create", "my_vol2"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "import", "my_vol2", helloTar})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(""), "output of volume import")

		session = podmanTest.Podman([]string{"run", "--volume", "my_vol2:/data", ALPINE, "cat", "/data/test"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman import/export volume should fail", func() {
		// try import on volume or source which does not exist
		SkipIfRemote("Volume export check does not work with a remote client")

		session := podmanTest.Podman([]string{"volume", "import", "notfound", "notfound.tar"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "open notfound.tar: no such file or directory"))

		session = podmanTest.Podman([]string{"volume", "import", "notfound", "-"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "no volume with name \"notfound\" found"))

		session = podmanTest.Podman([]string{"volume", "export", "notfound"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "no volume with name \"notfound\" found"))
	})

	It("podman create volume with bad volume option", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--opt", "badOpt=bad"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "invalid mount option badOpt for driver 'local': invalid argument"))
	})

	It("podman create volume with o=uid,gid", func() {
		volName := "testVol"
		uid := "3000"
		gid := "4000"
		session := podmanTest.Podman([]string{"volume", "create", "--opt", fmt.Sprintf("o=uid=%s,gid=%s", uid, gid), volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspectUID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .UID }}", volName})
		inspectUID.WaitWithDefaultTimeout()
		Expect(inspectUID).Should(ExitCleanly())
		Expect(inspectUID.OutputToString()).To(Equal(uid))

		inspectGID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .GID }}", volName})
		inspectGID.WaitWithDefaultTimeout()
		Expect(inspectGID).Should(ExitCleanly())
		Expect(inspectGID.OutputToString()).To(Equal(gid))

		// options should contain `uid=3000,gid=4000:3000:4000`
		optionFormat := `{{ .Options.o }}:{{ .Options.UID }}:{{ .Options.GID }}`
		optionStrFormatExpect := fmt.Sprintf(`uid=%s,gid=%s:%s:%s`, uid, gid, uid, gid)
		inspectOpts := podmanTest.Podman([]string{"volume", "inspect", "--format", optionFormat, volName})
		inspectOpts.WaitWithDefaultTimeout()
		Expect(inspectOpts).Should(ExitCleanly())
		Expect(inspectOpts.OutputToString()).To(Equal(optionStrFormatExpect))
	})

	It("podman create volume with --uid and --gid flags", func() {
		volName := "testVolFlags"
		uid := "3001"
		gid := "4001"
		session := podmanTest.Podman([]string{"volume", "create", "--uid", uid, "--gid", gid, volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspectUID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .UID }}", volName})
		inspectUID.WaitWithDefaultTimeout()
		Expect(inspectUID).Should(ExitCleanly())
		Expect(inspectUID.OutputToString()).To(Equal(uid))

		inspectGID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .GID }}", volName})
		inspectGID.WaitWithDefaultTimeout()
		Expect(inspectGID).Should(ExitCleanly())
		Expect(inspectGID.OutputToString()).To(Equal(gid))

		// The specified values must not be passed down to the -o option.
		fn := func(typ string) string {
			return fmt.Sprintf("{{ if .Options.%s }}{{ .Options.%s }}{{ else }}EMPTY{{ end }}", typ, typ)
		}
		optionFormat := fmt.Sprintf("%s:%s:%s", fn("o"), fn("UID"), fn("GID"))
		optionStrFormatExpected := "EMPTY:EMPTY:EMPTY"
		inspectOpts := podmanTest.Podman([]string{"volume", "inspect", "--format", optionFormat, volName})
		inspectOpts.WaitWithDefaultTimeout()
		Expect(inspectOpts).Should(ExitCleanly())
		Expect(inspectOpts.OutputToString()).To(Equal(optionStrFormatExpected))
	})

	It("podman create volume with --uid flag only", func() {
		volName := "testVolUidOnly"
		uid := "3002"
		session := podmanTest.Podman([]string{"volume", "create", "--uid", uid, volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspectUID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .UID }}", volName})
		inspectUID.WaitWithDefaultTimeout()
		Expect(inspectUID).Should(ExitCleanly())
		Expect(inspectUID.OutputToString()).To(Equal(uid))

		inspectGID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ if .GID }}{{ .GID }}{{ else }} EMPTY {{ end }}", volName})
		inspectGID.WaitWithDefaultTimeout()
		Expect(inspectGID).Should(ExitCleanly())
		Expect(inspectGID.OutputToString()).To(Equal("EMPTY"))
	})

	It("podman create volume with --gid flag only", func() {
		volName := "testVolGidOnly"
		gid := "4002"
		session := podmanTest.Podman([]string{"volume", "create", "--gid", gid, volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspectGID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .GID }}", volName})
		inspectGID.WaitWithDefaultTimeout()
		Expect(inspectGID).Should(ExitCleanly())
		Expect(inspectGID.OutputToString()).To(Equal(gid))

		inspectUID := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ if .UID }}{{ .UID }}{{ else }} EMPTY {{ end }}", volName})
		inspectUID.WaitWithDefaultTimeout()
		Expect(inspectUID).Should(ExitCleanly())
		Expect(inspectUID.OutputToString()).To(Equal("EMPTY"))
	})

	It("podman create volume --uid and --gid flags with invalid values", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--uid", "invalid", "testVol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "invalid argument \"invalid\" for \"--uid\" flag"))

		session = podmanTest.Podman([]string{"volume", "create", "--gid", "invalid", "testVol"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "invalid argument \"invalid\" for \"--gid\" flag"))
	})

	It("image-backed volume basic functionality", func() {
		podmanTest.AddImageToRWStore(FEDORA_MINIMAL)
		volName := "testvol"
		volCreate := podmanTest.Podman([]string{"volume", "create", "--driver", "image", "--opt", fmt.Sprintf("image=%s", FEDORA_MINIMAL), volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		runCmd := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "cat", "/test/etc/redhat-release"})
		runCmd.WaitWithDefaultTimeout()
		Expect(runCmd).Should(ExitCleanly())
		Expect(runCmd.OutputToString()).To(ContainSubstring("Fedora"))

		rmCmd := podmanTest.Podman([]string{"rmi", "--force", FEDORA_MINIMAL})
		rmCmd.WaitWithDefaultTimeout()
		Expect(rmCmd).Should(ExitCleanly())

		psCmd := podmanTest.Podman([]string{"ps", "-aq"})
		psCmd.WaitWithDefaultTimeout()
		Expect(psCmd).Should(ExitCleanly())
		Expect(psCmd.OutputToString()).To(BeEmpty())

		volumesCmd := podmanTest.Podman([]string{"volume", "ls", "-q"})
		volumesCmd.WaitWithDefaultTimeout()
		Expect(volumesCmd).Should(ExitCleanly())
		Expect(volumesCmd.OutputToString()).To(Not(ContainSubstring(volName)))
	})

	It("image-backed volume force removal", func() {
		podmanTest.AddImageToRWStore(FEDORA_MINIMAL)
		volName := "testvol"
		volCreate := podmanTest.Podman([]string{"volume", "create", "--driver", "image", "--opt", fmt.Sprintf("image=%s", FEDORA_MINIMAL), volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		runCmd := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "cat", "/test/etc/redhat-release"})
		runCmd.WaitWithDefaultTimeout()
		Expect(runCmd).Should(ExitCleanly())
		Expect(runCmd.OutputToString()).To(ContainSubstring("Fedora"))

		rmCmd := podmanTest.Podman([]string{"volume", "rm", "--force", volName})
		rmCmd.WaitWithDefaultTimeout()
		Expect(rmCmd).Should(ExitCleanly())

		psCmd := podmanTest.Podman([]string{"ps", "-aq"})
		psCmd.WaitWithDefaultTimeout()
		Expect(psCmd).Should(ExitCleanly())
		Expect(psCmd.OutputToString()).To(BeEmpty())

		volumesCmd := podmanTest.Podman([]string{"volume", "ls", "-q"})
		volumesCmd.WaitWithDefaultTimeout()
		Expect(volumesCmd).Should(ExitCleanly())
		Expect(volumesCmd.OutputToString()).To(Not(ContainSubstring(volName)))
	})
})
