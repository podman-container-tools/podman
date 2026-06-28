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
		Expect(session.ErrorToStringArray()).To(Equal([]string{"PodmanSession", "test", "Podman Session", ""}))
	})

	It("Test GrepString", func() {
		match, backStr := session.GrepString("Session")
		Expect(match).To(BeTrue())
		Expect(backStr).To(Equal([]string{"PodmanSession", "Podman Session"}))

		match, backStr = session.GrepString("I am not here")
		Expect(match).To(Not(BeTrue()))
		Expect(backStr).To(BeNil())
	})

	It("Test LineInOutputContains", func() {
		Expect(session.LineInOutputContains("Podman")).To(BeTrue())
		Expect(session.LineInOutputContains("Session")).To(BeTrue())
		Expect(session.LineInOutputContains("I am not here")).To(Not(BeTrue()))
	})

	It("Test LineInOutputContainsTag", func() {
		session = StartFakeCmdSession([]string{"HEAD LINE", "docker.io/library/busybox   latest   e1ddd7948a1c   5 weeks ago   1.38MB"})
		session.WaitWithDefaultTimeout()
		Expect(session.LineInOutputContainsTag("docker.io/library/busybox", "latest")).To(BeTrue())
		Expect(session.LineInOutputContainsTag("busybox", "latest")).To(Not(BeTrue()))
	})

	It("Test IsJSONOutputValid", func() {
		session = StartFakeCmdSession([]string{`{"page":1,"fruits":["apple","peach","pear"]}`})
		session.WaitWithDefaultTimeout()
		Expect(session.IsJSONOutputValid()).To(BeTrue())

		session = StartFakeCmdSession([]string{"I am not JSON"})
		session.WaitWithDefaultTimeout()
		Expect(session.IsJSONOutputValid()).To(Not(BeTrue()))
	})

	It("Test WaitWithDefaultTimeout", func() {
		session = StartFakeCmdSession([]string{"sleep", "2"})
		Expect(session.ExitCode()).Should(Equal(-1))
		session.WaitWithDefaultTimeout()
		Expect(session.ExitCode()).Should(Equal(0))
	})
})
