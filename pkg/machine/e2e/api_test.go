package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	jsoniter "github.com/json-iterator/go"
	"github.com/moby/moby/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/machine"
)

const (
	NamedPipeProto = "npipe://"
)

var _ = Describe("run podman API test calls", func() {
	It("client connect to machine socket", func(ctx context.Context) {
		if runtime.GOOS == "windows" {
			Skip("Go docker client doesn't support unix socket on Windows")
		}
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(ctx, i.withImage(mb.imagePath).withNow()).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectJSON := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(ctx, inspectJSON).run(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))

		var inspectInfo []machine.InspectInfo
		err = jsoniter.Unmarshal(inspectSession.Bytes(), &inspectInfo)
		Expect(err).ToNot(HaveOccurred())
		sockPath := inspectInfo[0].ConnectionInfo.PodmanSocket.GetPath()

		// check with docker client
		cli, err := client.New(client.WithHost("unix://" + sockPath))
		Expect(err).ToNot(HaveOccurred())
		_, err = cli.Ping(ctx, client.PingOptions{})
		Expect(err).ToNot(HaveOccurred())

		// check with curl
		cmd := exec.CommandContext(ctx, "curl", "--unix-socket", sockPath, "http://d/v5.0.0/libpod/info")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		Expect(err).ToNot(HaveOccurred())

		if runtime.GOOS == "windows" {
			// check that the pipe connection also works on windows
			pipePath := inspectInfo[0].ConnectionInfo.PodmanPipe.GetPath()
			cli, err := client.New(client.WithHost(NamedPipeProto + filepath.ToSlash(pipePath)))
			Expect(err).ToNot(HaveOccurred())
			_, err = cli.Ping(ctx, client.PingOptions{})
			Expect(err).ToNot(HaveOccurred())
		}
	})
})
