//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman create passwd", func() {
	It("podman create without --passwd flag (default true)", func() {
		podmanTest.PodmanExitCleanly("create", "--name", "test_passwd", "--user", "1234:1234", ALPINE, "cat", "/etc/passwd")
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		run := podmanTest.PodmanExitCleanly("start", "-a", "test_passwd")
		Expect(run.OutputToString()).To(ContainSubstring("1234"))
	})

	It("podman create with --passwd=false", func() {
		podmanTest.PodmanExitCleanly("create", "--name", "test_no_passwd", "--passwd=false", "--user", "1234:1234", ALPINE, "cat", "/etc/passwd")
		Expect(podmanTest.NumberOfContainers()).To(Equal(1))

		run := podmanTest.PodmanExitCleanly("start", "-a", "test_no_passwd")
		Expect(run.OutputToString()).NotTo(ContainSubstring("1234"))
	})

	It("podman create with explicit --passwd=true (same as default)", func() {
		podmanTest.PodmanExitCleanly("create", "--name", "test_passwd_explicit", "--passwd=true", "--user", "5678:5678", ALPINE, "cat", "/etc/passwd")

		run := podmanTest.PodmanExitCleanly("start", "-a", "test_passwd_explicit")
		Expect(run.OutputToString()).To(ContainSubstring("5678"))
	})
})
