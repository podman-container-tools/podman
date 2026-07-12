package e2e_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("podman machine proxy settings propagation", func() {
	It("ssh to running machine and check proxy settings", func() {
		defer func() {
			os.Unsetenv("HTTP_PROXY")
			os.Unsetenv("HTTPS_PROXY")
		}()

		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		proxyURL := "http://abcdefghijklmnopqrstuvwxyz-proxy"
		os.Setenv("HTTP_PROXY", proxyURL)
		os.Setenv("HTTPS_PROXY", proxyURL)

		s := new(startMachine)
		startSession, err := mb.setName(name).setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		sshProxy := sshMachine{}
		sshSession, err := mb.setName(name).setCmd(sshProxy.withSSHCommand([]string{"printenv", "HTTP_PROXY"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		Expect(sshSession.outputToString()).To(ContainSubstring(proxyURL))

		sshSession, err = mb.setName(name).setCmd(sshProxy.withSSHCommand([]string{"printenv", "HTTPS_PROXY"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		Expect(sshSession.outputToString()).To(ContainSubstring(proxyURL))

		stop := new(stopMachine)
		stopSession, err := mb.setName(name).setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// Now update proxy env, lets use some special vars to make sure our scripts can handle it
		proxy1 := "http://foo:b%%40r@example.com:8080"
		proxy2 := "https://foo:bar%%3F@example.com:8080"
		noproxy := "noproxy1.example.com,noproxy2.example.com"
		os.Setenv("HTTP_PROXY", proxy1)
		os.Setenv("HTTPS_PROXY", proxy2)
		os.Setenv("NO_PROXY", noproxy)

		// start it again should update the proxies
		startSession, err = mb.setName(name).setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		sshSession, err = mb.setName(name).setCmd(sshProxy.withSSHCommand([]string{"printenv", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		Expect(string(sshSession.Out.Contents())).To(Equal(proxy1 + "\n" + proxy2 + "\n" + noproxy + "\n"))
	})
})
