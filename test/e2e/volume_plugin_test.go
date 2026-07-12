//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

var _ = Describe("Podman volume plugins", func() {
	BeforeEach(func() {
		os.Setenv("CONTAINERS_CONF", "config/containers.conf")
		SkipIfRemote("Volume plugins only supported as local")
		SkipIfRootless("Root is required for volume plugin testing")
		SkipIfNotAMD64() // https://github.com/containers/podman/issues/28270
		err = os.MkdirAll("/run/docker/plugins", 0o755)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		podmanTest.CleanupVolume()
	})

	It("volume create with nonexistent plugin errors", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--driver", "notexist", "test_volume_name"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "volume test_volume_name uses volume plugin notexist but it could not be retrieved: no volume plugin with name notexist available: required plugin missing"))
	})

	It("volume create with not-running plugin does not error", func() {
		session := podmanTest.Podman([]string{"volume", "create", "--driver", "testvol0", "test_volume_name"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `Error: volume test_volume_name uses volume plugin testvol0 but it could not be retrieved: cannot access plugin testvol0 socket "/run/docker/plugins/testvol0.sock": stat /run/docker/plugins/testvol0.sock: no such file or directory`))
	})

	It("volume create and remove with running plugin succeeds", func() {
		podmanTest.AddImageToRWStore(volumeTest)

		pluginStatePath := filepath.Join(podmanTest.TempDir, "volumes")
		err := os.Mkdir(pluginStatePath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// Keep this distinct within tests to avoid multiple tests using the same plugin.
		// This one verifies that the "image" plugin uses a volume plugin, not the "image" driver.
		pluginName := "image"
		plugin := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "-v", "/run/docker/plugins:/run/docker/plugins", "-v", fmt.Sprintf("%v:%v", pluginStatePath, pluginStatePath), "-d", volumeTest, "--sock-name", pluginName, "--path", pluginStatePath})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(ExitCleanly())

		// Make sure the socket is available (see #17956)
		err = WaitForFile(fmt.Sprintf("/run/docker/plugins/%s.sock", pluginName))
		Expect(err).ToNot(HaveOccurred())

		volName := "testVolume1"
		create := podmanTest.Podman([]string{"volume", "create", "--driver", pluginName, volName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		ls1 := podmanTest.Podman([]string{"volume", "ls", "-q"})
		ls1.WaitWithDefaultTimeout()
		Expect(ls1).Should(ExitCleanly())
		arrOutput := ls1.OutputToStringArray()
		Expect(arrOutput).To(HaveLen(1))
		Expect(arrOutput[0]).To(ContainSubstring(volName))

		// Verify this is not an image volume.
		inspect := podmanTest.Podman([]string{"volume", "inspect", volName, "--format", "{{.StorageID}}"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(BeEmpty())

		remove := podmanTest.Podman([]string{"volume", "rm", volName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).Should(ExitCleanly())

		ls2 := podmanTest.Podman([]string{"volume", "ls", "-q"})
		ls2.WaitWithDefaultTimeout()
		Expect(ls2).Should(ExitCleanly())
		Expect(ls2.OutputToStringArray()).To(BeEmpty())
	})

	It("volume inspect with running plugin succeeds", func() {
		podmanTest.AddImageToRWStore(volumeTest)

		pluginStatePath := filepath.Join(podmanTest.TempDir, "volumes")
		err := os.Mkdir(pluginStatePath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// Keep this distinct within tests to avoid multiple tests using the same plugin.
		pluginName := "testvol2"
		plugin := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "-v", "/run/docker/plugins:/run/docker/plugins", "-v", fmt.Sprintf("%v:%v", pluginStatePath, pluginStatePath), "-d", volumeTest, "--sock-name", pluginName, "--path", pluginStatePath})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(ExitCleanly())

		// Make sure the socket is available (see #17956)
		err = WaitForFile(fmt.Sprintf("/run/docker/plugins/%s.sock", pluginName))
		Expect(err).ToNot(HaveOccurred())

		volName := "testVolume1"
		create := podmanTest.Podman([]string{"volume", "create", "--driver", pluginName, volName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		volInspect := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .Driver }}", volName})
		volInspect.WaitWithDefaultTimeout()
		Expect(volInspect).Should(ExitCleanly())
		Expect(volInspect.OutputToString()).To(ContainSubstring(pluginName))
	})

	It("remove plugin with stopped plugin succeeds", func() {
		podmanTest.AddImageToRWStore(volumeTest)

		pluginStatePath := filepath.Join(podmanTest.TempDir, "volumes")
		err := os.Mkdir(pluginStatePath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// Keep this distinct within tests to avoid multiple tests using the same plugin.
		pluginName := "testvol3"
		ctrName := "pluginCtr"
		plugin := podmanTest.Podman([]string{"run", "--name", ctrName, "--security-opt", "label=disable", "-v", "/run/docker/plugins:/run/docker/plugins", "-v", fmt.Sprintf("%v:%v", pluginStatePath, pluginStatePath), "-d", volumeTest, "--sock-name", pluginName, "--path", pluginStatePath})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(ExitCleanly())

		// Make sure the socket is available (see #17956)
		err = WaitForFile(fmt.Sprintf("/run/docker/plugins/%s.sock", pluginName))
		Expect(err).ToNot(HaveOccurred())

		volName := "testVolume1"
		create := podmanTest.Podman([]string{"volume", "create", "--driver", pluginName, volName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		ls1 := podmanTest.Podman([]string{"volume", "ls", "-q"})
		ls1.WaitWithDefaultTimeout()
		Expect(ls1).Should(ExitCleanly())
		arrOutput := ls1.OutputToStringArray()
		Expect(arrOutput).To(HaveLen(1))
		Expect(arrOutput[0]).To(ContainSubstring(volName))

		podmanTest.StopContainer(ctrName)

		// Remove should exit non-zero because missing plugin
		remove := podmanTest.Podman([]string{"volume", "rm", volName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).To(ExitWithError(125, "cannot remove volume testVolume1 from plugin testvol3, but it has been removed from Podman: required plugin missing"))

		// But the volume should still be gone
		ls2 := podmanTest.Podman([]string{"volume", "ls", "-q"})
		ls2.WaitWithDefaultTimeout()
		Expect(ls2).Should(ExitCleanly())
		Expect(ls2.OutputToStringArray()).To(BeEmpty())
	})

	It("use plugin in containers", func() {
		podmanTest.AddImageToRWStore(volumeTest)

		pluginStatePath := filepath.Join(podmanTest.TempDir, "volumes")
		err := os.Mkdir(pluginStatePath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// Keep this distinct within tests to avoid multiple tests using the same plugin.
		pluginName := "testvol4"
		plugin := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "-v", "/run/docker/plugins:/run/docker/plugins", "-v", fmt.Sprintf("%v:%v", pluginStatePath, pluginStatePath), "-d", volumeTest, "--sock-name", pluginName, "--path", pluginStatePath})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(ExitCleanly())

		// Make sure the socket is available (see #17956)
		err = WaitForFile(fmt.Sprintf("/run/docker/plugins/%s.sock", pluginName))
		Expect(err).ToNot(HaveOccurred())

		volName := "testVolume1"
		create := podmanTest.Podman([]string{"volume", "create", "--driver", pluginName, volName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		ctr1Name := "ctr1"
		ctr1 := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "--name", ctr1Name, "-v", fmt.Sprintf("%v:/test", volName), ALPINE, "sh", "-c", "touch /test/testfile && echo helloworld > /test/testfile"})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		ctr2Name := "ctr2"
		ctr2 := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "--name", ctr2Name, "-v", fmt.Sprintf("%v:/test", volName), ALPINE, "cat", "/test/testfile"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitCleanly())
		Expect(ctr2.OutputToString()).To(ContainSubstring("helloworld"))
	})

	It("podman volume reload", func() {
		podmanTest.AddImageToRWStore(volumeTest)

		confFile := filepath.Join(podmanTest.TempDir, "containers.conf")
		err := os.WriteFile(confFile, []byte(`[engine]
[engine.volume_plugins]
testvol5 = "/run/docker/plugins/testvol5.sock"`), 0o644)
		Expect(err).ToNot(HaveOccurred())
		os.Setenv("CONTAINERS_CONF", confFile)

		pluginStatePath := filepath.Join(podmanTest.TempDir, "volumes")
		err = os.Mkdir(pluginStatePath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// Keep this distinct within tests to avoid multiple tests using the same plugin.
		pluginName := "testvol5"
		ctrName := "pluginCtr"
		plugin := podmanTest.Podman([]string{
			"run", "--name", ctrName, "--security-opt", "label=disable", "-v", "/run/docker/plugins:/run/docker/plugins",
			"-v", fmt.Sprintf("%v:%v", pluginStatePath, pluginStatePath), "-d", volumeTest, "--sock-name", pluginName, "--path", pluginStatePath,
		})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(ExitCleanly())

		// Make sure the socket is available (see #17956)
		err = WaitForFile(fmt.Sprintf("/run/docker/plugins/%s.sock", pluginName))
		Expect(err).ToNot(HaveOccurred())

		localvol := "local-" + stringid.GenerateRandomID()
		// create local volume
		session := podmanTest.Podman([]string{"volume", "create", localvol})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		vol1 := "vol1-" + stringid.GenerateRandomID()
		session = podmanTest.Podman([]string{"volume", "create", "--driver", pluginName, vol1})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		// now create volume in plugin without podman
		vol2 := "vol2-" + stringid.GenerateRandomID()
		plugin = podmanTest.Podman([]string{"exec", ctrName, "/usr/local/bin/testvol", "--sock-name", pluginName, "create", vol2})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"volume", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())
		Expect(session.OutputToStringArray()).To(ContainElements(localvol, vol1))
		Expect(session.ErrorToString()).To(Equal("")) // make sure no errors are shown

		plugin = podmanTest.Podman([]string{"exec", ctrName, "/usr/local/bin/testvol", "--sock-name", pluginName, "remove", vol1})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(ExitCleanly())

		// now reload volumes from plugins
		session = podmanTest.Podman([]string{"volume", "reload"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())
		Expect(string(session.Out.Contents())).To(Equal(fmt.Sprintf(`Added:
%s
Removed:
%s
`, vol2, vol1)))
		Expect(session.ErrorToString()).To(Equal("")) // make sure no errors are shown

		session = podmanTest.Podman([]string{"volume", "ls", "-q"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())
		Expect(session.OutputToStringArray()).To(ContainElements(localvol, vol2))
		Expect(session.ErrorToString()).To(Equal("")) // make no errors are shown
	})

	It("volume driver timeouts test", func() {
		podmanTest.AddImageToRWStore(volumeTest)

		pluginStatePath := filepath.Join(podmanTest.TempDir, "volumes")
		err := os.Mkdir(pluginStatePath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// Keep this distinct within tests to avoid multiple tests using the same plugin.
		pluginName := "testvol6"
		plugin := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "-v", "/run/docker/plugins:/run/docker/plugins", "-v", fmt.Sprintf("%v:%v", pluginStatePath, pluginStatePath), "-d", volumeTest, "--sock-name", pluginName, "--path", pluginStatePath})
		plugin.WaitWithDefaultTimeout()
		Expect(plugin).Should(ExitCleanly())

		// Make sure the socket is available (see #17956)
		err = WaitForFile(fmt.Sprintf("/run/docker/plugins/%s.sock", pluginName))
		Expect(err).ToNot(HaveOccurred())

		volName := "testVolume1"
		create := podmanTest.Podman([]string{"volume", "create", "--driver", pluginName, volName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		volInspect := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .Timeout }}", volName})
		volInspect.WaitWithDefaultTimeout()
		Expect(volInspect).Should(ExitCleanly())
		Expect(volInspect.OutputToString()).To(ContainSubstring("15"))

		volName2 := "testVolume2"
		create2 := podmanTest.Podman([]string{"volume", "create", "--driver", pluginName, "--opt", "o=timeout=3", volName2})
		create2.WaitWithDefaultTimeout()
		Expect(create2).Should(ExitCleanly())

		volInspect2 := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{ .Timeout }}", volName2})
		volInspect2.WaitWithDefaultTimeout()
		Expect(volInspect2).Should(ExitCleanly())
		Expect(volInspect2.OutputToString()).To(ContainSubstring("3"))
	})
})
