//go:build linux || freebsd

package integration

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/podman/v6/libpod/define"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman run exit", func() {
	It("podman run exit define.ExecErrorCodeGeneric", func() {
		result := podmanTest.Podman([]string{"run", "--foobar", ALPINE, "ls", "$tmp"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(define.ExecErrorCodeGeneric, "unknown flag: --foobar"))
	})

	It("podman run exit ExecErrorCodeCannotInvoke", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "/etc"})
		result.WaitWithDefaultTimeout()
		expected := ".*(exec: \"/etc\": is a directory|(open executable|the path `/etc` is not a regular file): Operation not permitted: OCI permission denied).*"
		Expect(result).Should(ExitWithErrorRegex(define.ExecErrorCodeCannotInvoke, expected))
	})

	It("podman run exit ExecErrorCodeNotFound", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "foobar"})
		result.WaitWithDefaultTimeout()
		expected := ".*(executable file not found in \\$PATH|executable file `foobar` not found in \\$PATH: No such file or directory: OCI runtime attempted to invoke a command that was not found).*"
		Expect(result).Should(ExitWithErrorRegex(define.ExecErrorCodeNotFound, expected))
	})

	It("podman run exit 0", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman run exit 50", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "sh", "-c", "exit 50"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(50, ""))
	})

	It("podman run exit 125", func() {
		result := podmanTest.Podman([]string{"run", ALPINE, "sh", "-c", fmt.Sprintf("exit %d", define.ExecErrorCodeGeneric)})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(define.ExecErrorCodeGeneric, ""))
	})
})
