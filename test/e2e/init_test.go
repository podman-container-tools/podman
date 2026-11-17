//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman init", func() {
	It("podman init bogus container", func() {
		session := podmanTest.Podman([]string{"start", "123456"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, `Error: no container with name or ID "123456" found: no such container`))
	})

	It("podman init with no arguments", func() {
		session := podmanTest.Podman([]string{"start"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "Error: start requires at least one argument"))
	})

	It("podman init single container by ID", func() {
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		init := podmanTest.Podman([]string{"init", cid})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(ExitCleanly())
		Expect(init.OutputToString()).To(Equal(cid))
		result := podmanTest.Podman([]string{"inspect", cid})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State).To(HaveField("Status", "initialized"))
	})

	It("podman init single container by name", func() {
		name := "test1"
		session := podmanTest.Podman([]string{"create", "--name", name, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		init := podmanTest.Podman([]string{"init", name})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(ExitCleanly())
		Expect(init.OutputToString()).To(Equal(name))
		result := podmanTest.Podman([]string{"inspect", name})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State).To(HaveField("Status", "initialized"))
	})

	It("podman init latest container", func() {
		SkipIfRemote("--latest flag n/a")
		session := podmanTest.Podman([]string{"create", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		init := podmanTest.Podman([]string{"init", "--latest"})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(ExitCleanly())
		Expect(init.OutputToString()).To(Equal(cid))
		result := podmanTest.Podman([]string{"inspect", "--latest"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State).To(HaveField("Status", "initialized"))
	})

	It("podman init all three containers, one running", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test1", ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()
		session2 := podmanTest.Podman([]string{"create", "--name", "test2", ALPINE, "ls"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
		cid2 := session2.OutputToString()
		session3 := podmanTest.Podman([]string{"run", "--name", "test3", "-d", ALPINE, "top"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(ExitCleanly())
		cid3 := session3.OutputToString()
		init := podmanTest.Podman([]string{"init", "--all"})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(ExitCleanly())
		Expect(init.OutputToString()).To(ContainSubstring(cid))
		Expect(init.OutputToString()).To(ContainSubstring(cid2))
		Expect(init.OutputToString()).To(ContainSubstring(cid3))
		result := podmanTest.Podman([]string{"inspect", "test1"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData[0].State).To(HaveField("Status", "initialized"))
		result2 := podmanTest.Podman([]string{"inspect", "test2"})
		result2.WaitWithDefaultTimeout()
		Expect(result2).Should(ExitCleanly())
		conData2 := result2.InspectContainerToJSON()
		Expect(conData2[0].State).To(HaveField("Status", "initialized"))
		result3 := podmanTest.Podman([]string{"inspect", "test3"})
		result3.WaitWithDefaultTimeout()
		Expect(result3).Should(ExitCleanly())
		conData3 := result3.InspectContainerToJSON()
		Expect(conData3[0].State).To(HaveField("Status", "running"))
	})

	It("podman init running container errors", func() {
		session := podmanTest.Podman([]string{"run", "--name", "init_test", "-d", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		cid := session.OutputToString()

		init := podmanTest.Podman([]string{"init", "init_test"})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(ExitWithError(125, fmt.Sprintf("Error: container %s has already been created in runtime: container state improper", cid)))
	})

	It("podman init cleans up after initialization failure", func() {
		SkipIfRemote("requires local containers.conf configuration")

		confPath := filepath.Join(podmanTest.TempDir, "containers.conf")
		err := os.WriteFile(confPath, []byte("[containers]\nhost_containers_internal_ip = \"none\"\n"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		err = os.Setenv("CONTAINERS_CONF_OVERRIDE", confPath)
		Expect(err).ToNot(HaveOccurred())

		name := "init-failure-cleanup"

		podmanTest.PodmanExitCleanly("create", "--name", name, "--add-host", "host.docker.internal:host-gateway", ALPINE, "top")

		init := podmanTest.Podman([]string{"init", name})
		init.WaitWithDefaultTimeout()
		Expect(init).Should(ExitWithError(125, "host containers internal IP address is empty"))

		inspect := podmanTest.PodmanExitCleanly("inspect", "--format", "{{.State.Status}}", name)
		Expect(inspect.OutputToString()).To(Equal("created"))

		// Verify that start retries initialization instead of starting a partially initialized container.
		start := podmanTest.Podman([]string{"start", name})
		start.WaitWithDefaultTimeout()
		Expect(start).Should(ExitWithError(125, "host containers internal IP address is empty"))
	})
})
