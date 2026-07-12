//go:build linux || freebsd

package integration

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman commit", func() {
	It("podman commit container", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "test1", "--change", "BOGUS=foo", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `applying changes: processing change "BOGUS foo": did not understand change instruction "BOGUS foo"`))

		session = podmanTest.Podman([]string{"commit", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))

		messages := session.ErrorToString()
		Expect(messages).To(ContainSubstring("Getting image source signatures"))
		Expect(messages).To(ContainSubstring("Copying blob"))
		Expect(messages).To(ContainSubstring("Writing manifest to image destination"))
		Expect(messages).To(Not(ContainSubstring("level=")), "Unexpected logrus messages in stderr")

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0].RepoTags).To(ContainElement("foobar.com/test1-image:latest"))

		// commit second time with --quiet, should not write to stderr
		session = podmanTest.Podman([]string{"commit", "--quiet", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Exit(0))
		Expect(session.ErrorToString()).To(BeEmpty())

		// commit second time with --quiet, should not write to stderr
		session = podmanTest.Podman([]string{"commit", "--quiet", "bogus", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `no container with name or ID "bogus" found: no such container`))
	})

	It("podman commit single letter container", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "-q", "test1", "a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"inspect", "localhost/a:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0].RepoTags).To(ContainElement("localhost/a:latest"))
	})

	It("podman container commit container", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"container", "commit", "-q", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"image", "inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0].RepoTags).To(ContainElement("foobar.com/test1-image:latest"))
	})

	It("podman commit container with message", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "-q", "-f", "docker", "--message", "testing-commit", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0]).To(HaveField("Comment", "testing-commit"))
	})

	It("podman commit container with author", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "-q", "--author", "snoopy", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0]).To(HaveField("Author", "snoopy"))
	})

	It("podman commit container with change flag", func() {
		test := podmanTest.Podman([]string{"run", "--name", "test1", "-d", ALPINE, "ls"})
		test.WaitWithDefaultTimeout()
		Expect(test).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "-q", "--change", "LABEL=image=blue", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		inspectResults := check.InspectImageJSON()
		Expect(inspectResults[0].Labels).To(HaveKeyWithValue("image", "blue"))
	})

	It("podman commit container with --config flag", func() {
		test := podmanTest.Podman([]string{"run", "--name", "test1", "-d", ALPINE, "ls"})
		test.WaitWithDefaultTimeout()
		Expect(test).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		configFile, err := os.CreateTemp(podmanTest.TempDir, "")
		Expect(err).Should(Succeed())
		_, err = configFile.WriteString(`{"Labels":{"image":"green"}}`)
		Expect(err).Should(Succeed())
		configFile.Close()

		session := podmanTest.Podman([]string{"commit", "-q", "--config", configFile.Name(), "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		inspectResults := check.InspectImageJSON()
		Expect(inspectResults[0].Labels).To(HaveKeyWithValue("image", "green"))
	})

	It("podman commit container with --config pointing to trash", func() {
		test := podmanTest.Podman([]string{"run", "--name", "test1", "-d", ALPINE, "ls"})
		test.WaitWithDefaultTimeout()
		Expect(test).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		configFile, err := os.CreateTemp(podmanTest.TempDir, "")
		Expect(err).Should(Succeed())
		_, err = configFile.WriteString("this is not valid JSON\n")
		Expect(err).Should(Succeed())
		configFile.Close()

		session := podmanTest.Podman([]string{"commit", "-q", "--config", configFile.Name(), "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(Not(ExitCleanly()))
	})

	It("podman commit container with --squash", func() {
		test := podmanTest.Podman([]string{"run", "--name", "test1", "-d", ALPINE, "ls"})
		test.WaitWithDefaultTimeout()
		Expect(test).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "-q", "--squash", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.RootFS.Layers}}", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// Check for one layers
		Expect(strings.Fields(session.OutputToString())).To(HaveLen(1))
	})

	It("podman commit container with change flag and JSON entrypoint with =", func() {
		test := podmanTest.Podman([]string{"run", "--name", "test1", "-d", ALPINE, "ls"})
		test.WaitWithDefaultTimeout()
		Expect(test).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "-q", "--change", `ENTRYPOINT ["foo", "bar=baz"]`, "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config.Entrypoint).To(HaveLen(2))
		Expect(data[0].Config.Entrypoint[0]).To(Equal("foo"))
		Expect(data[0].Config.Entrypoint[1]).To(Equal("bar=baz"))
	})

	It("podman commit container with change CMD flag", func() {
		test := podmanTest.Podman([]string{"run", "--name", "test1", "-d", ALPINE, "ls"})
		test.WaitWithDefaultTimeout()
		Expect(test).Should(ExitCleanly())
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "-q", "--change", "CMD a b c", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.Config.Cmd}}", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("sh -c a b c"))

		session = podmanTest.Podman([]string{"commit", "-q", "--change", "CMD=[\"a\",\"b\",\"c\"]", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"inspect", "--format", "{{.Config.Cmd}}", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("sh -c")))
	})

	It("podman commit container with pause flag", func() {
		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "-q", "--pause=false", "test1", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
	})

	It("podman commit with volumes mounts and no include-volumes", func() {
		s := podmanTest.Podman([]string{"run", "--name", "test1", "-v", "/tmp:/foo", "alpine", "date"})
		s.WaitWithDefaultTimeout()
		Expect(s).Should(ExitCleanly())

		c := podmanTest.Podman([]string{"commit", "-q", "test1", "newimage"})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "newimage"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		image := inspect.InspectImageJSON()
		Expect(image[0].Config.Volumes).To(Not(HaveKey("/foo")))
	})

	It("podman commit with volume mounts and --include-volumes", func() {
		// We need to figure out how volumes are going to work correctly with the remote
		// client.  This does not currently work.
		SkipIfRemote("--testing Remote Volumes")
		s := podmanTest.Podman([]string{"run", "--name", "test1", "-v", "/tmp:/foo", "alpine", "date"})
		s.WaitWithDefaultTimeout()
		Expect(s).Should(ExitCleanly())

		c := podmanTest.Podman([]string{"commit", "-q", "--include-volumes", "test1", "newimage"})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "newimage"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		image := inspect.InspectImageJSON()
		Expect(image[0].Config.Volumes).To(HaveKey("/foo"))

		r := podmanTest.Podman([]string{"run", "newimage"})
		r.WaitWithDefaultTimeout()
		Expect(r).Should(ExitCleanly())
	})

	It("podman commit container check env variables", func() {
		s := podmanTest.Podman([]string{"run", "--name", "test1", "-e", "TEST=1=1-01=9.01", "alpine", "true"})
		s.WaitWithDefaultTimeout()
		Expect(s).Should(ExitCleanly())

		c := podmanTest.Podman([]string{"commit", "-q", "test1", "newimage"})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "newimage"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		image := inspect.InspectImageJSON()

		envMap := make(map[string]bool)
		for _, v := range image[0].Config.Env {
			envMap[v] = true
		}
		Expect(envMap).To(HaveKey("TEST=1=1-01=9.01"))
	})

	It("podman commit container and print id to external file", func() {
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(os.TempDir())).To(Succeed())
		targetPath := podmanTest.TempDir
		targetFile := filepath.Join(targetPath, "idFile")
		defer Expect(os.Chdir(cwd)).To(BeNil())

		_, ec, _ := podmanTest.RunLsContainer("test1")
		Expect(ec).To(Equal(0))
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		session := podmanTest.Podman([]string{"commit", "-q", "test1", "foobar.com/test1-image:latest", "--iidfile", targetFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		id, _ := os.ReadFile(targetFile)
		check := podmanTest.Podman([]string{"inspect", "foobar.com/test1-image:latest"})
		check.WaitWithDefaultTimeout()
		data := check.InspectImageJSON()
		Expect(data[0]).To(HaveField("ID", string(id)))
	})

	It("podman commit should not commit secret", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--secret", "mysecret", "--name", "secr", ALPINE, "cat", "/run/secrets/mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(secretsString))

		session = podmanTest.Podman([]string{"commit", "-q", "secr", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "foobar.com/test1-image:latest", "cat", "/run/secrets/mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(1, "can't open '/run/secrets/mysecret': No such file or directory"))
	})

	It("podman commit should not commit env secret", func() {
		secretsString := "somesecretdata"
		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte(secretsString), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", "mysecret", secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--secret", "source=mysecret,type=env", "--name", "secr", ALPINE, "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(secretsString))

		session = podmanTest.Podman([]string{"commit", "-q", "secr", "foobar.com/test1-image:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "foobar.com/test1-image:latest", "printenv", "mysecret"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Not(ContainSubstring(secretsString)))
	})

	It("podman commit adds exposed ports", func() {
		name := "testcon"
		s := podmanTest.Podman([]string{"run", "--name", name, "-p", "8585:80", ALPINE, "true"})
		s.WaitWithDefaultTimeout()
		Expect(s).Should(ExitCleanly())

		newImageName := "newimage"
		c := podmanTest.Podman([]string{"commit", "-q", name, newImageName})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", newImageName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		images := inspect.InspectImageJSON()
		Expect(images).To(HaveLen(1))
		Expect(images[0].Config.ExposedPorts).To(HaveKey("80/tcp"))

		name = "testcon2"
		s = podmanTest.Podman([]string{"run", "--name", name, "-d", NGINX_IMAGE})
		s.WaitWithDefaultTimeout()
		Expect(s).Should(ExitCleanly())

		newImageName = "newimage2"
		c = podmanTest.Podman([]string{"commit", "-q", name, newImageName})
		c.WaitWithDefaultTimeout()
		Expect(c).Should(ExitCleanly())

		inspect = podmanTest.Podman([]string{"inspect", newImageName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		images = inspect.InspectImageJSON()
		Expect(images).To(HaveLen(1))
		Expect(images[0].Config.ExposedPorts).To(HaveKey("80/tcp"))
	})
})
