//go:build linux || freebsd

package integration

import (
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman history", func() {
	It("podman history output flag", func() {
		session := podmanTest.Podman([]string{"history", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())
	})

	It("podman history with GO template", func() {
		session := podmanTest.Podman([]string{"history", "--format", "{{.ID}} {{.Created}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())

		session = podmanTest.Podman([]string{"history", "--format", "{{.CreatedAt}};{{.Size}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(MatchRegexp("[0-9-]{10}T[0-9:]{8}[Z0-9+:-]+;[0-9.]+[MG]*B( .+)?"))
	})

	It("podman history with human flag", func() {
		session := podmanTest.Podman([]string{"history", "--human=false", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())
	})

	It("podman history with quiet flag", func() {
		session := podmanTest.Podman([]string{"history", "-qH", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())
	})

	It("podman history with no-trunc flag", func() {
		session := podmanTest.Podman([]string{"history", "--no-trunc", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).ToNot(BeEmpty())

		session = podmanTest.Podman([]string{"history", "--no-trunc", "--format", "{{.ID}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		lines := session.OutputToStringArray()
		Expect(lines).ToNot(BeEmpty())
		// the image id must be 64 chars long
		Expect(lines[0]).To(HaveLen(64))

		session = podmanTest.Podman([]string{"history", "--no-trunc", "--format", "{{.CreatedBy}}", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		lines = session.OutputToStringArray()
		Expect(lines).ToNot(BeEmpty())
		Expect(session.OutputToString()).ToNot(ContainSubstring("..."))
		// the second line in the alpine history contains a command longer than 45 chars
		Expect(len(lines[1])).To(BeNumerically(">", 45))

		// Size is different on other architectures
		if runtime.GOARCH == "amd64" {
			session = podmanTest.Podman([]string{"history", "--no-trunc", "--format", "{{.Size}}", ALPINE})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToStringArray()).To(Equal([]string{"0B", "5.84MB"}))
		}
	})

	It("podman history with json flag", func() {
		session := podmanTest.Podman([]string{"history", "--format=json", ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(BeValidJSON())
	})
})
