package e2e_test

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/machine/define"
)

var _ = Describe("podman machine set", func() {
	It("machine set rejects excessive cpus", func() {
		skipIfWSL("WSL cannot change cpus via set")
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		badSet := setMachine{}
		badCPUSession, err := mb.setName(name).setCmd(badSet.withCPUs(9999999)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(badCPUSession).To(Exit(125))
		Expect(badCPUSession.errorToString()).To(ContainSubstring("greater than number of host CPUs"))
	})

	It("set machine cpus, disk, memory", func() {
		skipIfWSL("WSL cannot change set properties of disk, processor, or memory")
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		setMem := setMachine{}
		SetMemSession, err := mb.setName(name).setCmd(setMem.withMemory(524288)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(SetMemSession).To(Exit(125))

		set := setMachine{}
		setSession, err := mb.setName(name).setCmd(set.withCPUs(2).withDiskSize(102).withMemory(4096)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(setSession).To(Exit(0))

		// shrinking disk size is verboten
		shrink, err := mb.setName(name).setCmd(set.withDiskSize(5)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(shrink).To(Exit(125))

		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		sshCPU := sshMachine{}
		CPUsession, err := mb.setName(name).setCmd(sshCPU.withSSHCommand([]string{"lscpu", "|", "grep", "\"CPU(s):\"", "|", "head", "-1"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(CPUsession).To(Exit(0))
		Expect(CPUsession.outputToString()).To(ContainSubstring("2"))

		sshDisk := sshMachine{}
		diskSession, err := mb.setName(name).setCmd(sshDisk.withSSHCommand([]string{"sudo", "fdisk", "-l", "|", "grep", "Disk"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(diskSession).To(Exit(0))
		Expect(diskSession.outputToString()).To(ContainSubstring("102 GiB"))

		sshMemory := sshMachine{}
		memorySession, err := mb.setName(name).setCmd(sshMemory.withSSHCommand([]string{"cat", "/proc/meminfo", "|", "grep", "-i", "'memtotal'", "|", "grep", "-o", "'[[:digit:]]*'"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(memorySession).To(Exit(0))
		foundMemory, err := strconv.Atoi(memorySession.outputToString())
		Expect(err).ToNot(HaveOccurred())
		Expect(foundMemory).To(BeNumerically(">", 3800000))
		Expect(foundMemory).To(BeNumerically("<", 4200000))

		// Setting a running machine results in 125
		runner, err := mb.setName(name).setCmd(set.withCPUs(4)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(runner).To(Exit(125))
	})

	It("wsl cannot change disk, memory, processor", func() {
		skipIfNotVmtype(define.WSLVirt, "tests are only for WSL provider")
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		setMem := setMachine{}
		setMemSession, err := mb.setName(name).setCmd(setMem.withMemory(4096)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(setMemSession).To(Exit(125))
		Expect(setMemSession.errorToString()).To(ContainSubstring("changing memory not supported for WSL machines"))

		setProc := setMachine{}
		setProcSession, err := mb.setName(name).setCmd(setProc.withCPUs(2)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(setProcSession.errorToString()).To(ContainSubstring("changing CPUs not supported for WSL machines"))

		setDisk := setMachine{}
		setDiskSession, err := mb.setName(name).setCmd(setDisk.withDiskSize(102)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(setDiskSession.errorToString()).To(ContainSubstring("changing disk size not supported for WSL machines"))
	})

	It("no settings should change if no flags", func() {
		skipIfWSL("WSL cannot change set properties of disk, processor, or memory")
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		set := setMachine{}
		setSession, err := mb.setName(name).setCmd(&set).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(setSession).To(Exit(0))

		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		ssh2 := sshMachine{}
		cpus := runtime.NumCPU() / 2
		if cpus == 0 {
			cpus = 1
		}
		sshSession2, err := mb.setName(name).setCmd(ssh2.withSSHCommand([]string{"lscpu", "|", "grep", "\"CPU(s):\"", "|", "head", "-1"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession2).To(Exit(0))
		Expect(sshSession2.outputToString()).To(ContainSubstring(strconv.Itoa(cpus)))

		ssh3 := sshMachine{}
		sshSession3, err := mb.setName(name).setCmd(ssh3.withSSHCommand([]string{"sudo", "fdisk", "-l", "|", "grep", "Disk"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession3).To(Exit(0))
		Expect(sshSession3.outputToString()).To(ContainSubstring(fmt.Sprintf("%d GiB", defaultDiskSize)))
	})

	It("set rootful with docker sock change", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		set := setMachine{}
		setSession, err := mb.setName(name).setCmd(set.withRootful(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(setSession).To(Exit(0))

		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.Rootful}}")
		inspectSession, err := mb.setName(name).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal("true"))

		ssh2 := sshMachine{}
		sshSession2, err := mb.setName(name).setCmd(ssh2.withSSHCommand([]string{"readlink /var/run/docker.sock"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshSession2).To(Exit(0))
		output := strings.TrimSpace(sshSession2.outputToString())
		Expect(output).To(Equal("/run/podman/podman.sock"))
	})

	It("set user mode networking", func() {
		if testProvider.VMType() != define.WSLVirt {
			Skip("Test is only for WSL")
		}
		// TODO - this currently fails
		Skip("test fails bc usermode network needs plumbing for WSL")

		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		set := setMachine{}
		setSession, err := mb.setName(name).setCmd(set.withUserModeNetworking(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(setSession).To(Exit(0))

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.UserModeNetworking}}")
		inspectSession, err := mb.setName(name).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal("true"))
	})

	It("set while machine already running", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		set := setMachine{}
		setSession, err := mb.setName(name).setCmd(set.withRootful(true)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(setSession).To(Exit(125))
		Expect(setSession.errorToString()).To(ContainSubstring("Error: unable to change settings unless vm is stopped"))
	})
})
