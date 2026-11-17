package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/moby/moby/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
)

const (
	NamedPipeProto = "npipe://"
)

var _ = Describe("run podman API test calls", func() {
	It("client connect to machine socket", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectJSON := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(inspectJSON).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))

		var inspectInfo []machine.InspectInfo
		err = jsoniter.Unmarshal(inspectSession.Bytes(), &inspectInfo)
		Expect(err).ToNot(HaveOccurred())
		sockPath := inspectInfo[0].ConnectionInfo.PodmanSocket.GetPath()
		Eventually(sockPath, 10*time.Second).Should(BeAnExistingFile())

		checkAPI := func() {
			// check with docker client
			host := "unix://" + sockPath
			if runtime.GOOS == "windows" {
				// Go docker client doesn't support unix socket on Windows,
				// use the pipe connection instead
				host = NamedPipeProto + filepath.ToSlash(inspectInfo[0].ConnectionInfo.PodmanPipe.GetPath())
			}
			cli, err := client.New(client.WithHost(host))
			Expect(err).ToNot(HaveOccurred())
			_, err = cli.Ping(context.Background(), client.PingOptions{})
			Expect(err).ToNot(HaveOccurred())

			// check with curl
			cmd := exec.Command("curl", "--unix-socket", sockPath, "http://d/v5.0.0/libpod/info")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			Expect(err).ToNot(HaveOccurred())
		}
		checkAPI()

		// Kill the proxy without giving it a chance to remove its socket, as
		// happens when the host shuts down. The next start must still bring
		// the API back.
		// gvproxy writes its pid file next to the API socket.
		runtimeDir := filepath.Dir(inspectInfo[0].ConnectionInfo.PodmanSocket.Path)
		pid := apiProxyPID(name, runtimeDir)
		proxy, err := os.FindProcess(pid)
		Expect(err).ToNot(HaveOccurred())
		Expect(proxy.Kill()).To(Succeed())
		Eventually(func() bool { return processExists(pid) }, 10*time.Second).Should(BeFalse())
		Expect(sockPath).To(BeAnExistingFile())

		stop := new(stopMachine)
		stopSession, err := mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		start := new(startMachine)
		startSession, err := mb.setCmd(start).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		checkAPI()
	})
})

// gvproxyPID returns the PID recorded in the gvproxy pid file in runtimeDir.
func gvproxyPID(runtimeDir string) int {
	pidFile, err := define.NewMachineFile(filepath.Join(runtimeDir, "gvproxy.pid"), nil)
	Expect(err).ToNot(HaveOccurred())
	pid, err := pidFile.ReadPIDFrom()
	Expect(err).ToNot(HaveOccurred())
	return pid
}
