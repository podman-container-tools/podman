package e2e_test

import (
	"context"
	"fmt"

	"go.podman.io/podman/v6/pkg/machine/define"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine restart", func() {
	It("should tell you when trying to restart a machine that doesn't exist", func(ctx context.Context) {
		r := restartMachine{}
		name := "aVMThatDoesntExist"
		session, err := mb.setName(name).setCmd(ctx, r).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("VM does not exist"))
	})

	It("should restart machine", func(ctx context.Context) {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(ctx, i.withImage(mb.imagePath).withVolume("")).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspect := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(ctx, inspect.withFormat("{{.State}}")).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal(define.Stopped))

		// Restart stopped machine
		r := restartMachine{}
		restartSession, err := mb.setName(name).setCmd(ctx, r).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(restartSession).To(Exit(0))

		inspectSession, err = mb.setName(name).setCmd(ctx, inspect.withFormat("{{.State}}")).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal(define.Running))

		// Now restart running machine
		r = restartMachine{}
		restartSession, err = mb.setName(name).setCmd(ctx, r).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(restartSession).To(Exit(0))
		Expect(restartSession.outputToString()).To(ContainSubstring(fmt.Sprintf("Machine %q restarted successfully", name)))

		inspect = new(inspectMachine)
		inspectSession, err = mb.setName(name).setCmd(ctx, inspect.withFormat("{{.State}}")).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal(define.Running))
	})
})
