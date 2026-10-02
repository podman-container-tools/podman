//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman log rotation options", func() {
	It("podman run --log-opt max-file sets rotation file count", func() {
		SkipIfConmonVersionLessThan("2.2.0")

		logDir := GinkgoT().TempDir()
		logPath := filepath.Join(logDir, "ctr.log")

		podmanTest.PodmanExitCleanly(
			"run", "--rm",
			"--log-driver", "k8s-file",
			"--log-opt", fmt.Sprintf("path=%s", logPath),
			"--log-opt", "max-file=3",
			ALPINE, "echo", "hello rotation",
		)

		// The log file must exist after the container exits.
		_, err := os.Stat(logPath)
		Expect(err).ShouldNot(HaveOccurred(), "log file should exist at %s", logPath)

		// Verify max-file appears in podman inspect output.
		ctrName := "log-rotate-inspect"
		podmanTest.PodmanExitCleanly(
			"create",
			"--name", ctrName,
			"--log-driver", "k8s-file",
			"--log-opt", "max-size=1mb",
			"--log-opt", "max-file=5",
			ALPINE, "echo", "hello",
		)

		inspect := podmanTest.PodmanExitCleanly(
			"inspect", "--format",
			"{{index .HostConfig.LogConfig.Config \"max-file\"}}",
			ctrName,
		)
		Expect(inspect.OutputToString()).To(Equal("5"))

		inspectRotate := podmanTest.PodmanExitCleanly(
			"inspect", "--format",
			"{{index .HostConfig.LogConfig.Config \"log-rotate\"}}",
			ctrName,
		)
		Expect(inspectRotate.OutputToString()).To(Equal("true"))
	})

	It("podman run --log-opt log-rotate=true sets log rotation", func() {
		ctrName := "log-rotate-true"
		podmanTest.PodmanExitCleanly(
			"create",
			"--name", ctrName,
			"--log-driver", "k8s-file",
			"--log-opt", "max-size=1mb",
			"--log-opt", "log-rotate=true",
			ALPINE, "echo", "hello",
		)

		inspect := podmanTest.PodmanExitCleanly(
			"inspect", "--format",
			"{{index .HostConfig.LogConfig.Config \"log-rotate\"}}",
			ctrName,
		)
		Expect(inspect.OutputToString()).To(Equal("true"))
	})

	It("podman run rejects conflicting --log-opt max-file and log-rotate=false", func() {
		session := podmanTest.Podman([]string{
			"run", "--rm",
			"--log-opt", "max-file=5",
			"--log-opt", "log-rotate=false",
			ALPINE, "true",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "conflicting log options"))
	})

	It("podman run --log-opt log-rotate=true without max-size", func() {
		ctrName := "log-rotate-no-maxsize"
		podmanTest.PodmanExitCleanly(
			"create",
			"--name", ctrName,
			"--log-driver", "k8s-file",
			"--log-opt", "log-rotate=true",
			ALPINE, "echo", "hello",
		)

		inspect := podmanTest.PodmanExitCleanly(
			"inspect", "--format",
			"{{index .HostConfig.LogConfig.Config \"log-rotate\"}}",
			ctrName,
		)
		Expect(inspect.OutputToString()).To(Equal("true"))
	})

	It("podman run rejects invalid --log-opt max-file=0", func() {
		session := podmanTest.Podman([]string{
			"run", "--rm",
			"--log-opt", "max-file=0",
			ALPINE, "true",
		})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "invalid value for log option"))
	})
})
