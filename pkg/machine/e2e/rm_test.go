package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/machine/define"
)

var _ = Describe("podman machine rm", func() {
	It("bad init name", func() {
		i := rmMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
	})

	It("Remove machine", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		rm := rmMachine{}
		removeSession, err := mb.setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(removeSession).To(Exit(0))

		// Inspecting a non-existent machine should fail
		// which means it is gone
		_, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(Equal(125))

		// Removing non-existent machine should fail
		removeSession2, err := mb.setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(removeSession2).To(Exit(125))
		Expect(removeSession2.errorToString()).To(ContainSubstring(fmt.Sprintf("%s: VM does not exist", name)))

		// Ensure that the system connections have the right rootfulness
		name = randomString()
		i = new(initMachine)
		session, err = mb.setName(name).setCmd(i.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		name2 := randomString()
		i = new(initMachine)
		session, err = mb.setName(name2).setCmd(i.withFakeImage(mb).withRootful(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		bm := basicMachine{}
		sysConnOutput, err := mb.setCmd(bm.withPodmanCommand([]string{"system", "connection", "list", "--format", "{{.Name}}--{{.Default}}"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sysConnOutput.outputToString()).To(ContainSubstring(name + "--true"))

		rm = rmMachine{}
		removeSession, err = mb.setName(name).setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(removeSession).To(Exit(0))

		sysConnOutput, err = mb.setCmd(bm.withPodmanCommand([]string{"system", "connection", "list", "--format", "{{.Name}}--{{.Default}}"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sysConnOutput.outputToString()).To(ContainSubstring(name2 + "-root--true"))
	})

	It("Remove running machine", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		rm := new(rmMachine)

		// Removing a running machine should fail
		stop, err := mb.setCmd(rm).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stop).To(Exit(125))
		Expect(stop.errorToString()).To(ContainSubstring(fmt.Sprintf("vm \"%s\" cannot be destroyed", name)))

		// Removing again with force
		stopAgain, err := mb.setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopAgain).To(Exit(0))

		// Inspect to be dead sure
		_, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(Equal(125))
	})

	It("machine rm --save-ignition --save-image", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.SSHConfig.IdentityPath}}")
		inspectSession, err := mb.setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		key := inspectSession.outputToString()
		pubkey := key + ".pub"

		rm := rmMachine{}
		removeSession, err := mb.setCmd(rm.withForce().withSaveIgnition().withSaveImage()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(removeSession).To(Exit(0))

		// Inspecting a non-existent machine should fail
		// which means it is gone
		_, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(Equal(125))

		_, err = os.Stat(key)
		Expect(err).ToNot(HaveOccurred())
		_, err = os.Stat(pubkey)
		Expect(err).ToNot(HaveOccurred())

		// WSL does not use ignition
		if testProvider.VMType() != define.WSLVirt {
			ignPath := filepath.Join(testDir, ".config", "containers", "podman", "machine", testProvider.VMType().String(), mb.name+".ign")
			_, err = os.Stat(ignPath)
			Expect(err).ToNot(HaveOccurred())
		}
		_, err = os.Stat(mb.imagePath)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Remove machine sharing ssh key with another machine", func() {
		expectedIdentityPathSuffix := filepath.Join(".local", "share", "containers", "podman", "machine", define.DefaultIdentityName)

		fooName := "foo"
		foo := new(initMachine)
		session, err := mb.setName(fooName).setCmd(foo.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		barName := "bar"
		bar := new(initMachine)
		session, err = mb.setName(barName).setCmd(bar.withUpdateConnection(ptrBool(false)).withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectFoo := new(inspectMachine)
		inspectFoo = inspectFoo.withFormat("{{.SSHConfig.IdentityPath}}")
		inspectSession, err := mb.setName(fooName).setCmd(inspectFoo).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(ContainSubstring(expectedIdentityPathSuffix))
		fooIdentityPath := inspectSession.outputToString()

		inspectBar := new(inspectMachine)
		inspectBar = inspectBar.withFormat("{{.SSHConfig.IdentityPath}}")
		inspectSession, err = mb.setName(barName).setCmd(inspectBar).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal(fooIdentityPath))

		rmFoo := new(rmMachine)
		stop, err := mb.setName(fooName).setCmd(rmFoo.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stop).To(Exit(0))

		// removal of foo should not affect the ability to ssh into the bar machine
		sshBar := new(sshMachine)
		sshSession, err := mb.setName(barName).setCmd(sshBar.withSSHCommand([]string{"echo", "foo"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession).To(Exit(0))
	})

	It("Removing all machines doesn't delete ssh keys", func() {
		fooName := "foo"
		foo := new(initMachine)
		session, err := mb.setName(fooName).setCmd(foo.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspectFoo := new(inspectMachine)
		inspectFoo = inspectFoo.withFormat("{{.SSHConfig.IdentityPath}}")
		inspectSession, err := mb.setName(fooName).setCmd(inspectFoo).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		fooIdentityPath := inspectSession.outputToString()

		rmFoo := new(rmMachine)
		stop, err := mb.setName(fooName).setCmd(rmFoo.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stop).To(Exit(0))

		_, err = os.Stat(fooIdentityPath)
		Expect(err).ToNot(HaveOccurred())
		_, err = os.Stat(fooIdentityPath + ".pub")
		Expect(err).ToNot(HaveOccurred())
	})
})
