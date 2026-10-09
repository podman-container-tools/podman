package e2e_test

import (
	"context"
	"runtime"

	jsoniter "github.com/json-iterator/go"
	"go.podman.io/podman/v6/pkg/machine"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman inspect stop", func() {
	It("inspect bad name", func(ctx context.Context) {
		i := inspectMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(ctx, &i).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
	})

	It("inspect two machines", func(ctx context.Context) {
		i := new(initMachine)
		foo1, err := mb.setName("foo1").setCmd(ctx, i.withFakeImage(mb)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(foo1).To(Exit(0))

		ii := new(initMachine)
		foo2, err := mb.setName("foo2").setCmd(ctx, ii.withFakeImage(mb)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(foo2).To(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.Name}}")
		inspectSession, err := mb.setName("foo1").setCmd(ctx, inspect).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.Bytes()).To(ContainSubstring("foo1"))
	})

	It("inspect with go format", func(ctx context.Context) {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(ctx, i.withFakeImage(mb)).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// regular inspect should
		inspectJSON := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(ctx, inspectJSON).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))

		var inspectInfo []machine.InspectInfo
		err = jsoniter.Unmarshal(inspectSession.Bytes(), &inspectInfo)
		Expect(err).ToNot(HaveOccurred())

		if runtime.GOOS == "windows" {
			Expect(inspectInfo[0].ConnectionInfo.PodmanPipe.GetPath()).To(ContainSubstring("podman-"))
		}
		Expect(inspectInfo[0].ConnectionInfo.PodmanSocket.GetPath()).To(HaveSuffix("api.sock"))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.Name}}")
		inspectSession, err = mb.setName(name).setCmd(ctx, inspect).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.Bytes()).To(ContainSubstring(name))

		// check invalid template returns error
		inspect = new(inspectMachine)
		inspect = inspect.withFormat("{{.Abcde}}")
		inspectSession, err = mb.setName(name).setCmd(ctx, inspect).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(125))
		Expect(inspectSession.errorToString()).To(ContainSubstring("can't evaluate field Abcde in type machine.InspectInfo"))
	})

	It("inspect shows a unique socket name per machine", func(ctx context.Context) {
		var socks []string
		for range 2 {
			name := randomString()
			i := new(initMachine)
			session, err := mb.setName(name).setCmd(ctx, i.withFakeImage(mb)).run(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(session).To(Exit(0))

			// regular inspect should
			inspectJSON := new(inspectMachine)
			inspectSession, err := mb.setName(name).setCmd(ctx, inspectJSON).run(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(inspectSession).To(Exit(0))

			var inspectInfo []machine.InspectInfo
			err = jsoniter.Unmarshal(inspectSession.Bytes(), &inspectInfo)
			Expect(err).ToNot(HaveOccurred())
			socks = append(socks, inspectInfo[0].ConnectionInfo.PodmanSocket.GetPath())
		}

		Expect(socks[0]).ToNot(Equal(socks[1]))
	})
})
