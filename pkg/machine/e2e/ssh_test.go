package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/machine/define"
)

var _ = Describe("podman machine ssh", func() {
	It("bad machine name", func() {
		name := randomString()
		ssh := &sshMachine{}
		session, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("not exist"))
	})

	It("ssh to non-running machine", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh := &sshMachine{}
		sshSession, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession.errorToString()).To(ContainSubstring("is not running"))
		Expect(sshSession).To(Exit(125))
	})

	It("ssh to running machine and check os-type", func() {
		wsl := testProvider.VMType() == define.WSLVirt

		// setting this name instead of randomized because we want to test when the
		// machine name is and is not provided.  That's what the loop below is for

		name := "podman-machine-default"
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow().withUpdateConnection(ptrBool(true))).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// pass 1
		ssh := &sshMachine{}
		// pass 2
		bm := basicMachine{}
		var mcs []machineCommand
		// check with the machine name
		mcs = append(mcs, ssh.withSSHCommand([]string{"cat", "/etc/os-release"}))
		// check without the machine name
		mcs = append(mcs, bm.withPodmanCommand([]string{"machine", "ssh", "cat", "/etc/os-release"}))
		for _, mc := range mcs {
			sshSession, err := mb.setCmd(mc).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(sshSession).To(Exit(0))

			if wsl {
				Expect(sshSession.outputToString()).To(ContainSubstring("Fedora Linux"))
			} else {
				Expect(sshSession.outputToString()).To(ContainSubstring("Fedora CoreOS"))
			}
		}
		// keep exit code
		sshSession, err := mb.setName(name).setCmd(ssh.withSSHCommand([]string{"false"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(1))
		Expect(sshSession.outputToString()).To(Equal(""))

		// WSL will often emit an error message about the ssh key and keychains
		if !wsl {
			Expect(sshSession.errorToString()).To(Equal(""))
		}
	})

	It("verify machine rootfulness", func() {
		wsl := testProvider.VMType() == define.WSLVirt
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh := &sshMachine{}
		sshSession, err := mb.setName(name).setCmd(ssh.withSSHCommand([]string{"whoami"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		if wsl {
			Expect(sshSession.outputToString()).To(Equal("user"))
		} else {
			Expect(sshSession.outputToString()).To(Equal("core"))
		}

		stop := &stopMachine{}
		stopSession, err := mb.setName(name).setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		set := &setMachine{}
		setSession, err := mb.setName(name).setCmd(set.withRootful(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(setSession).To(Exit(0))

		start := &startMachine{}
		startSession, err := mb.setName(name).setCmd(start).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		sshSession, err = mb.setName(name).setCmd(ssh.withSSHCommand([]string{"whoami"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
		Expect(sshSession.outputToString()).To(Equal("root"))
	})
})
