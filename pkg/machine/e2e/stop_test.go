package e2e_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine stop", func() {
	It("stop bad name", func() {
		i := stopMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
	})

	It("Stop running machine", func() {
		name := randomString()
		i := new(initMachine)
		starttime := time.Now()
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		stop := new(stopMachine)
		stopSession, err := mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// Stopping it again should not result in an error
		stopAgain, err := mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopAgain).To(Exit(0))
		Expect(stopAgain.outputToString()).To(ContainSubstring(fmt.Sprintf("Machine \"%s\" stopped successfully", name)))

		// Stopping a machine should update the last up time
		inspect := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(inspect.withFormat("{{.LastUp.Format \"2006-01-02T15:04:05Z07:00\"}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		lastupTime, err := time.Parse(time.RFC3339, inspectSession.outputToString())
		Expect(err).ToNot(HaveOccurred())
		Expect(lastupTime).To(BeTemporally(">", starttime))
	})
})
