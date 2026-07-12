//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman systemd", func() {
	It("podman run container with systemd PID1", func() {
		ctrName := "testSystemd"
		run := podmanTest.Podman([]string{"run", "--name", ctrName, "-t", "-i", "-d", SYSTEMD_IMAGE, "/sbin/init"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(Exit(0))

		logs := podmanTest.Podman([]string{"logs", ctrName})
		logs.WaitWithDefaultTimeout()
		Expect(logs).Should(ExitCleanly())

		// Give container 10 seconds to start
		started := podmanTest.WaitContainerReady(ctrName, "Reached target multi-user.target - Multi-User System.", 30, 1)
		Expect(started).To(BeTrue(), "Reached multi-user.target")

		systemctl := podmanTest.Podman([]string{"exec", ctrName, "systemctl", "status", "--no-pager"})
		systemctl.WaitWithDefaultTimeout()
		Expect(systemctl).Should(ExitCleanly())
		Expect(systemctl.OutputToString()).To(ContainSubstring("State:"))

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config).To(HaveField("SystemdMode", true))

		stats := podmanTest.Podman([]string{"stats", "--no-stream", ctrName})
		stats.WaitWithDefaultTimeout()
		Expect(stats).Should(ExitCleanly())

		cgroupPath := podmanTest.Podman([]string{"inspect", "--format='{{.State.CgroupPath}}'", ctrName})
		cgroupPath.WaitWithDefaultTimeout()
		Expect(cgroupPath).Should(ExitCleanly())
		Expect(cgroupPath.OutputToString()).To(Not(ContainSubstring("init.scope")))
	})

	It("podman create container with systemd entrypoint triggers systemd mode", func() {
		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--name", ctrName, "--entrypoint", "/sbin/init", SYSTEMD_IMAGE})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config).To(HaveField("SystemdMode", true))
	})

	It("podman systemd in command triggers systemd mode", func() {
		containerfile := fmt.Sprintf(`FROM %s
RUN mkdir -p /usr/lib/systemd/; touch /usr/lib/systemd/systemd
CMD /usr/lib/systemd/systemd`, ALPINE)

		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err := os.WriteFile(containerfilePath, []byte(containerfile), 0o755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"build", "-t", "systemd", "--file", containerfilePath, podmanTest.TempDir})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--name", ctrName, "systemd"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config).To(HaveField("SystemdMode", true))
	})

	It("podman create container with --uidmap and conmon PidFile accessible", func() {
		ctrName := "testCtrUidMap"
		run := podmanTest.Podman([]string{"run", "-d", "--uidmap=0:1:1000", "--name", ctrName, ALPINE, "top"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"inspect", "--format", "{{.ConmonPidFile}}", ctrName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		pidFile := strings.TrimSuffix(session.OutputToString(), "\n")
		_, err := os.ReadFile(pidFile)
		Expect(err).ToNot(HaveOccurred())
	})

	It("podman create container with systemd=always triggers systemd mode", func() {
		ctrName := "testCtr"
		run := podmanTest.Podman([]string{"create", "--name", ctrName, "--systemd", "always", ALPINE})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())

		result := podmanTest.Podman([]string{"inspect", ctrName})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		conData := result.InspectContainerToJSON()
		Expect(conData).To(HaveLen(1))
		Expect(conData[0].Config).To(HaveField("SystemdMode", true))
	})

	It("podman run --systemd container should NOT mount /run noexec", func() {
		session := podmanTest.Podman([]string{"run", "--systemd", "always", ALPINE, "sh", "-c", "mount  | grep \"/run \""})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		Expect(session.OutputToString()).To(Not(ContainSubstring("noexec")))
	})

	It("podman run --systemd arg is case insensitive", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--systemd", "Always", ALPINE, "echo", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("test"))

		session = podmanTest.Podman([]string{"run", "--rm", "--systemd", "True", ALPINE, "echo", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("test"))

		session = podmanTest.Podman([]string{"run", "--rm", "--systemd", "False", ALPINE, "echo", "test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("test"))
	})
})
