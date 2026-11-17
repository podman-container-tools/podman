package e2e_test

import (
	"path/filepath"
	"runtime"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"go.podman.io/podman/v6/pkg/machine"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine compose", func() {
	It("compose test environment variable setup", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectJSON := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(inspectJSON).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))

		var inspectInfo []machine.InspectInfo
		err = jsoniter.Unmarshal(inspectSession.Bytes(), &inspectInfo)
		Expect(err).ToNot(HaveOccurred())

		compose := new(fakeCompose)
		composeSession, err := mb.setName(name).setCmd(compose).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(composeSession).To(Exit(0))

		lines := composeSession.outputToStringSlice()
		if runtime.GOOS != "windows" {
			Expect(lines[0]).To(Equal("unix://" + inspectInfo[0].ConnectionInfo.PodmanSocket.GetPath()))
		} else {
			Expect(strings.TrimSuffix(lines[0], "\r")).To(Equal("npipe://" + filepath.ToSlash(inspectInfo[0].ConnectionInfo.PodmanPipe.GetPath())))
		}
		Expect(strings.TrimSuffix(lines[1], "\r")).To(Equal("0"))
	})
})
