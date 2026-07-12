//go:build linux || freebsd

package integration

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var PodmanDockerfile = fmt.Sprintf(`
FROM  %s
LABEL RUN podman --version`, ALPINE)

var LsDockerfile = fmt.Sprintf(`
FROM  %s
LABEL RUN ls -la`, ALPINE)

var PodmanRunlabelNameDockerfile = fmt.Sprintf(`
FROM  %s
LABEL RUN podman run --name NAME IMAGE`, ALPINE)

var _ = Describe("podman container runlabel", func() {
	BeforeEach(func() {
		SkipIfRemote("runlabel is not supported for remote connections")
	})

	It("podman container runlabel (podman --version)", func() {
		image := "podman-runlabel-test:podman"
		podmanTest.BuildImage(PodmanDockerfile, image, "false")

		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman container runlabel (ls -la)", func() {
		image := "podman-runlabel-test:ls"
		podmanTest.BuildImage(LsDockerfile, image, "false")

		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})
	It("podman container runlabel --display", func() {
		image := "podman-runlabel-test:ls"
		podmanTest.BuildImage(LsDockerfile, image, "false")

		result := podmanTest.Podman([]string{"container", "runlabel", "--display", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring(podmanTest.PodmanBinary + " -la"))

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman container runlabel bogus label should result in non-zero exit code", func() {
		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", ALPINE})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, fmt.Sprintf("cannot find the value of label: RUN in image: %s", ALPINE)))
		// should not panic when label missing the value or don't have the label
		Expect(result.OutputToString()).To(Not(ContainSubstring("panic")))
	})

	It("podman container runlabel bogus label in remote image should result in non-zero exit", func() {
		remoteImage := "quay.io/libpod/testimage:00000000"
		result := podmanTest.Podman([]string{"container", "runlabel", "RUN", remoteImage})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, fmt.Sprintf("cannot find the value of label: RUN in image: %s", remoteImage)))
		// should not panic when label missing the value or don't have the label
		Expect(result.OutputToString()).To(Not(ContainSubstring("panic")))
	})

	It("runlabel should fail with nonexistent authfile", func() {
		image := "podman-runlabel-test:podman"
		podmanTest.BuildImage(PodmanDockerfile, image, "false")

		// runlabel should fail with nonexistent authfile
		result := podmanTest.Podman([]string{"container", "runlabel", "--authfile", "/tmp/nonexistent", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError(125, "credential file is not accessible: faccessat /tmp/nonexistent: no such file or directory"))

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman container runlabel name removes tag from image", func() {
		image := "podman-runlabel-name:sometag"
		podmanTest.BuildImage(PodmanRunlabelNameDockerfile, image, "false")

		result := podmanTest.Podman([]string{"container", "runlabel", "--display", "RUN", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(Equal("command: " + podmanTest.PodmanBinary + " run --name podman-runlabel-name localhost/" + image))

		result = podmanTest.Podman([]string{"rmi", image})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})
})
