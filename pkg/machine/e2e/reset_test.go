package e2e_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine reset", func() {
	It("starting from scratch should not error", func(ctx context.Context) {
		i := resetMachine{}
		session, err := mb.setCmd(ctx, i.withForce()).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
	})

	It("reset machine with one defined machine", func(ctx context.Context) {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(ctx, i.withFakeImage(mb)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ls := new(listMachine)
		beforeSession, err := mb.setCmd(ctx, ls.withNoHeading()).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(beforeSession).To(Exit(0))
		Expect(beforeSession.outputToStringSlice()).To(HaveLen(1))

		reset := resetMachine{}
		resetSession, err := mb.setCmd(ctx, reset.withForce()).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(resetSession).To(Exit(0))

		afterSession, err := mb.setCmd(ctx, ls.withNoHeading()).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(afterSession).To(Exit(0))
		Expect(afterSession.outputToStringSlice()).To(BeEmpty())
	})

	It("reset with running machine and other machines idle ", func(ctx context.Context) {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(ctx, i.withImage(mb.imagePath).withNow()).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ls := new(listMachine)
		beforeSession, err := mb.setCmd(ctx, ls.withNoHeading()).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(beforeSession).To(Exit(0))
		Expect(beforeSession.outputToStringSlice()).To(HaveLen(1))

		name2 := randomString()
		i2 := new(initMachine)
		session2, err := mb.setName(name2).setCmd(ctx, i2.withFakeImage(mb)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session2).To(Exit(0))

		beforeSession, err = mb.setCmd(ctx, ls.withNoHeading()).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(beforeSession).To(Exit(0))
		Expect(beforeSession.outputToStringSlice()).To(HaveLen(2))

		reset := resetMachine{}
		resetSession, err := mb.setCmd(ctx, reset.withForce()).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(resetSession).To(Exit(0))

		afterSession, err := mb.setCmd(ctx, ls.withNoHeading()).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(afterSession).To(Exit(0))
		Expect(afterSession.outputToStringSlice()).To(BeEmpty())
	})
})
