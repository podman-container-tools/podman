package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("PodmanSession test", func() {
	var session *PodmanSession

	BeforeEach(func() {
		session = StartFakeCmdSession([]string{"PodmanSession", "test", "Podman Session"})
		session.WaitWithDefaultTimeout()
	})

	It("Test OutputToString", func() {
		Expect(session.OutputToString()).To(Equal("PodmanSession test Podman Session"))
	})

	It("Test OutputToStringArray", func() {
		Expect(session.OutputToStringArray()).To(Equal([]string{"PodmanSession", "test", "Podman Session"}))
	})

	It("Test ErrorToString", func() {
		Expect(session.ErrorToString()).To(Equal("PodmanSession test Podman Session"))
	})

	It("Test ErrorToStringArray", func() {
		Expect(session.ErrorToStringArray()).To(Equal([]string{"PodmanSession", "test", "Podman Session"}))
	})

	It("Test ErrorToStringArray with empty output", func() {
		session = StartFakeCmdSession([]string{})
		session.WaitWithDefaultTimeout()
		Expect(session.ErrorToStringArray()).To(BeEmpty())
	})

	It("Test BeValidJSON", func() {
		session = StartFakeCmdSession([]string{`{"page":1,"fruits":["apple","peach","pear"]}`})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(BeValidJSON())

		session = StartFakeCmdSession([]string{"I am not JSON"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).ToNot(BeValidJSON())
	})

	It("Test WaitWithDefaultTimeout", func() {
		session = StartFakeCmdSession([]string{"sleep", "2"})
		Expect(session.ExitCode()).Should(Equal(-1))
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).Should(Equal(0))
	})
})
