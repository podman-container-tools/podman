//go:build linux || freebsd

package integration

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman config", func() {
	AfterEach(func() {
		cleanup := podmanTest.Podman([]string{"config", "rm", "--all", "--ignore"})
		cleanup.WaitWithDefaultTimeout()
	})

	It("creates, inspects, lists, checks, and removes configs", func() {
		configPath := filepath.Join(podmanTest.TempDir, "config")
		Expect(os.WriteFile(configPath, []byte("config-data"), 0o600)).To(Succeed())

		created := podmanTest.Podman([]string{"config", "create", "app-config", configPath})
		created.WaitWithDefaultTimeout()
		Expect(created).To(ExitCleanly())
		configID := created.OutputToString()

		duplicate := podmanTest.Podman([]string{"config", "create", "app-config", configPath})
		duplicate.WaitWithDefaultTimeout()
		Expect(duplicate).To(ExitWithError(125, "config name in use"))

		ignored := podmanTest.Podman([]string{"config", "create", "--ignore", "app-config", configPath})
		ignored.WaitWithDefaultTimeout()
		Expect(ignored).To(ExitCleanly())
		Expect(ignored.OutputToString()).To(Equal(configID))

		inspected := podmanTest.Podman([]string{"config", "inspect", "--format", "{{.Spec.Name}}", configID})
		inspected.WaitWithDefaultTimeout()
		Expect(inspected).To(ExitCleanly())
		Expect(inspected.OutputToString()).To(Equal("app-config"))

		inspected = podmanTest.Podman([]string{"config", "inspect", "--showconfig", "--format", "{{.ConfigData}}", configID})
		inspected.WaitWithDefaultTimeout()
		Expect(inspected).To(ExitCleanly())
		Expect(inspected.OutputToString()).To(Equal("config-data"))

		listed := podmanTest.Podman([]string{"config", "ls", "--format", "{{.Name}}"})
		listed.WaitWithDefaultTimeout()
		Expect(listed).To(ExitCleanly())
		Expect(listed.OutputToString()).To(ContainSubstring("app-config"))

		exists := podmanTest.Podman([]string{"config", "exists", "app-config"})
		exists.WaitWithDefaultTimeout()
		Expect(exists).To(ExitCleanly())

		run := podmanTest.Podman([]string{"run", "--rm", "--config", "source=app-config,target=/tmp/app-config", ALPINE, "cat", "/tmp/app-config"})
		run.WaitWithDefaultTimeout()
		Expect(run).To(ExitCleanly())
		Expect(run.OutputToString()).To(Equal("config-data"))

		write := podmanTest.Podman([]string{"run", "--rm", "--config", "source=app-config,target=/tmp/app-config", ALPINE, "sh", "-c", "echo changed > /tmp/app-config 2>/dev/null && exit 0; exit 42"})
		write.WaitWithDefaultTimeout()
		Expect(write).To(Exit(42))

		removed := podmanTest.Podman([]string{"config", "rm", configID})
		removed.WaitWithDefaultTimeout()
		Expect(removed).To(ExitCleanly())

		exists = podmanTest.Podman([]string{"config", "exists", "app-config"})
		exists.WaitWithDefaultTimeout()
		Expect(exists).To(Exit(1))
	})
})
