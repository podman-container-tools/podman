package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/moby/moby/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/machine/define"
)

var _ = Describe("podman machine start on Windows", func() {
	It("releases the working directory after starting a WSL machine", func(ctx SpecContext) {
		skipIfNotVmtype(define.WSLVirt, "WSL test only")
		init := new(initMachine).withImage(mb.imagePath)
		session, err := mb.setCmd(init).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		dir := filepath.Join(testDir, "start here")
		Expect(os.Mkdir(dir, 0o755)).To(Succeed())
		binary, err := filepath.Abs(mb.podmanBinary)
		Expect(err).ToNot(HaveOccurred())
		startCtx, cancelStart := context.WithTimeout(ctx, mb.timeout)
		defer cancelStart()
		cmd := exec.CommandContext(startCtx, binary, "machine", "start", mb.name)
		cmd.Dir = dir
		cmd.WaitDelay = 5 * time.Second
		output, err := cmd.CombinedOutput()
		Expect(err).ToNot(HaveOccurred(), "%s", output)

		info, code, err := mb.toInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(code).To(BeZero())
		Expect(info).To(HaveLen(1))
		Expect(info[0].State).To(Equal(define.Running))
		Expect(os.Remove(dir)).To(Succeed())

		pipe := info[0].ConnectionInfo.PodmanPipe
		Expect(pipe).ToNot(BeNil())
		api, err := client.New(client.WithHost(NamedPipeProto + filepath.ToSlash(pipe.GetPath())))
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(api.Close)
		pingCtx, cancelPing := context.WithTimeout(ctx, mb.timeout)
		defer cancelPing()
		_, err = api.Ping(pingCtx, client.PingOptions{})
		Expect(err).ToNot(HaveOccurred())

		stopped, err := mb.setCmd(new(stopMachine)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopped).To(Exit(0))
	})
})
