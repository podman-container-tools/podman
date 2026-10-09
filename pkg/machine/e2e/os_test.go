package e2e_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/machine/define"
)

var _ = Describe("podman machine os apply", func() {
	It("apply machine", func(ctx context.Context) {
		machineName := "foobar"
		if p := testProvider.VMType(); p == define.WSLVirt {
			i := new(initMachine)
			session, err := mb.setName(machineName).setCmd(ctx, i.withFakeImage(mb)).run(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(session).To(Exit(0))
		}
		a := new(applyMachineOS)
		applySession, err := mb.setName(machineName).setCmd(ctx, a.withImage("quay.io/foobar:latest").withRestart()).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(applySession.ExitCode()).To(Equal(125))
		switch testProvider.VMType() {
		case define.WSLVirt:
			Expect(applySession.errorToString()).To(ContainSubstring("this command is not supported for WSL"))
		default:
			Expect(applySession.errorToString()).To(ContainSubstring(fmt.Sprintf("%s: VM does not exist", machineName)))
		}
	})
})
