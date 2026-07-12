//go:build linux || freebsd

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman run entrypoint", func() {
	It("podman run no command, entrypoint, or cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT []
CMD []
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		if session.ExitCode() == 126 {
			// special case for crun <= 1.20, remove once a new version is out
			Expect(session).Should(ExitWithError(126, "open executable: Operation not permitted: OCI permission denied"))
			return
		}
		Expect(session).Should(ExitWithErrorRegex(127, ".*(executable file not found in \\$PATH|cannot find `` in \\$PATH).*: OCI runtime attempted to invoke a command that was not found.*"))
	})

	It("podman run entrypoint == [\"\"]", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT [""]
CMD []
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest", "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("hello"))
	})

	It("podman run entrypoint", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman run entrypoint with cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
CMD [ "-v"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(4))
	})

	It("podman run entrypoint with user cmd overrides image cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
CMD [ "-v"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest", "-i"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(5))
	})

	It("podman run entrypoint with user cmd no image cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "foobar.com/entrypoint:latest", "-i"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(5))
	})

	It("podman run user entrypoint overrides image entrypoint and image cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
CMD ["-i"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "--entrypoint=uname", "foobar.com/entrypoint:latest"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("Linux")))

		session = podmanTest.Podman([]string{"run", "--entrypoint", "", "foobar.com/entrypoint:latest", "uname"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("Linux")))
	})

	It("podman run user entrypoint with command overrides image entrypoint and image cmd", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
CMD ["-i"]
ENTRYPOINT ["grep", "Alpine", "/etc/os-release"]
`
		podmanTest.BuildImage(dockerfile, "foobar.com/entrypoint:latest", "false")
		session := podmanTest.Podman([]string{"run", "--entrypoint=uname", "foobar.com/entrypoint:latest", "-r"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(Not(ContainElement(HavePrefix("Linux"))))
	})
})
