package bindings_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/bindings/containers"
)

var _ = Describe("Podman containers exec", func() {
	var (
		bt *bindingTest
		s  *gexec.Session
	)

	BeforeEach(func() {
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("Podman exec create+start makes an exec session", func() {
		name := "testCtr"
		cid, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())

		execConfig := new(handlers.ExecCreateConfig)
		execConfig.Cmd = []string{"echo", "hello world"}

		sessionID, err := containers.ExecCreate(bt.conn, name, execConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(sessionID).To(Not(Equal("")))

		inspectOut, err := containers.ExecInspect(bt.conn, sessionID, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectOut.ContainerID).To(Equal(cid))
		Expect(inspectOut.ProcessConfig.Entrypoint).To(Equal("echo"))
		Expect(inspectOut.ProcessConfig.Arguments).To(HaveLen(1))
		Expect(inspectOut.ProcessConfig.Arguments[0]).To(Equal("hello world"))

		err = containers.ExecStart(bt.conn, sessionID, nil)
		Expect(err).ToNot(HaveOccurred())

		inspectOut, err = containers.ExecInspect(bt.conn, sessionID, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectOut.ContainerID).To(Equal(cid))
		Expect(inspectOut.Running).To(BeFalse(), "session should not be running")
		Expect(inspectOut.ExitCode).To(Equal(0), "exit code from echo")
	})

	It("Podman exec create with bad command fails", func() {
		name := "testCtr"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())

		execConfig := new(handlers.ExecCreateConfig)

		_, err = containers.ExecCreate(bt.conn, name, execConfig)
		Expect(err).To(HaveOccurred())
	})

	It("Podman exec create with invalid container fails", func() {
		execConfig := new(handlers.ExecCreateConfig)
		execConfig.Cmd = []string{"echo", "hello world"}

		_, err := containers.ExecCreate(bt.conn, "doesnotexist", execConfig)
		Expect(err).To(HaveOccurred())
	})

	It("Podman exec inspect on invalid session fails", func() {
		_, err := containers.ExecInspect(bt.conn, "0000000000000000000000000000000000000000000000000000000000000000", nil)
		Expect(err).To(HaveOccurred())
	})
})
