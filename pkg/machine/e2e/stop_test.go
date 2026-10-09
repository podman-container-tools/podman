package e2e_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine stop", func(ctx context.Context) {
	It("stop bad name", func() {
		i := stopMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(ctx, &i).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
	})
})
