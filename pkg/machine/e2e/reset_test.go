package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine reset", func() {
	It("starting from scratch should not error", func() {
		i := resetMachine{}
		session, err := mb.setCmd(i.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
	})

	It("reset machine with one defined machine", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ls := new(listMachine)
		beforeSession, err := mb.setCmd(ls.withNoHeading()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(beforeSession).To(Exit(0))
		Expect(beforeSession.outputToStringSlice()).To(HaveLen(1))

		reset := resetMachine{}
		resetSession, err := mb.setCmd(reset.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(resetSession).To(Exit(0))

		afterSession, err := mb.setCmd(ls.withNoHeading()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(afterSession).To(Exit(0))
		Expect(afterSession.outputToStringSlice()).To(BeEmpty())
	})

	It("reset with running machine and other machines idle ", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ls := new(listMachine)
		beforeSession, err := mb.setCmd(ls.withNoHeading()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(beforeSession).To(Exit(0))
		Expect(beforeSession.outputToStringSlice()).To(HaveLen(1))

		name2 := randomString()
		i2 := new(initMachine)
		session2, err := mb.setName(name2).setCmd(i2.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session2).To(Exit(0))

		beforeSession, err = mb.setCmd(ls.withNoHeading()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(beforeSession).To(Exit(0))
		Expect(beforeSession.outputToStringSlice()).To(HaveLen(2))

		reset := resetMachine{}
		resetSession, err := mb.setCmd(reset.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(resetSession).To(Exit(0))

		afterSession, err := mb.setCmd(ls.withNoHeading()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(afterSession).To(Exit(0))
		Expect(afterSession.outputToStringSlice()).To(BeEmpty())
	})
})
