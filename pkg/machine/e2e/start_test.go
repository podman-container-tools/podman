package e2e_test

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	jsoniter "github.com/json-iterator/go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/machine/define"
)

var _ = Describe("podman machine start", func() {
	It("start simple machine", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))
		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		info, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())
		Expect(info[0].State).To(Equal(define.Running))

		stop := new(stopMachine)
		stopSession, err := mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// suppress output
		startSession, err = mb.setCmd(s.withNoInfo()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))
		Expect(startSession.outputToString()).ToNot(ContainSubstring("API forwarding"))

		stopSession, err = mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		startSession, err = mb.setCmd(s.withQuiet()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))
		Expect(startSession.outputToStringSlice()).To(HaveLen(1))
	})

	It("bad start name", func() {
		i := startMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("VM does not exist"))
	})

	It("start machine already started", func() {
		name := randomString()
		i := new(initMachine)
		machineTestBuilderInit := mb.setName(name).setCmd(i.withImage(mb.imagePath))
		session, err := machineTestBuilderInit.run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		info, ec, err := mb.toQemuInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())
		Expect(info[0].State).To(Equal(define.Running))

		startSession, err = mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(125))
		Expect(startSession.errorToString()).To(ContainSubstring(fmt.Sprintf("Error: unable to start %q: already running", machineTestBuilderInit.name)))
	})

	It("start machine with conflict on SSH port", func() {
		i := new(initMachine)
		session, err := mb.setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		inspect := new(inspectMachine)
		inspectSession, err := mb.setCmd(inspect.withFormat("{{.SSHConfig.Port}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		inspectPort := inspectSession.outputToString()

		connections := new(listSystemConnection)
		connectionsSession, err := mb.setCmd(connections.withFormat("{{.URI}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(connectionsSession).To(Exit(0))
		connectionURLs := connectionsSession.outputToStringSlice()
		connectionPorts, err := mapToPort(connectionURLs)
		Expect(err).ToNot(HaveOccurred())
		Expect(connectionPorts).To(HaveEach(inspectPort))

		// start a listener on the ssh port
		listener, err := net.Listen("tcp", "127.0.0.1:"+inspectPort)
		Expect(err).ToNot(HaveOccurred())
		defer listener.Close()

		s := new(startMachine)
		startSession, err := mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))
		Expect(startSession.errorToString()).To(ContainSubstring("detected port conflict on machine ssh port"))

		inspect2 := new(inspectMachine)
		inspectSession2, err := mb.setCmd(inspect2.withFormat("{{.SSHConfig.Port}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession2).To(Exit(0))
		inspectPort2 := inspectSession2.outputToString()
		Expect(inspectPort2).To(Not(Equal(inspectPort)))

		connections2 := new(listSystemConnection)
		connectionsSession2, err := mb.setCmd(connections2.withFormat("{{.URI}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(connectionsSession2).To(Exit(0))
		connectionURLs2 := connectionsSession2.outputToStringSlice()
		connectionPorts2, err := mapToPort(connectionURLs2)
		Expect(err).ToNot(HaveOccurred())
		Expect(connectionPorts2).To(HaveEach(inspectPort2))
	})

	It("start only starts specified machine", func() {
		j := initMachine{}
		dontstartme := randomString()
		session2, err := mb.setName(dontstartme).setCmd(j.withFakeImage(mb)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session2).To(Exit(0))

		i := initMachine{}
		startme := randomString()
		session, err := mb.setName(startme).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		s := &startMachine{}
		// Provide a buffer as stdin to simulate non-tty input (e.g., piped or redirected stdin)
		// When stdin is not a tty, the command should not prompt for connection updates
		stdinBuf := bytes.NewBufferString("n\n")
		session3, err := mb.setName(startme).setCmd(s).setTimeout(time.Minute * 10).setStdin(stdinBuf).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session3).Should(Exit(0))
		// Verify that the prompt message did not appear (no prompting when stdin is not a tty)
		combinedOutput := session3.outputToString() + session3.errorToString()
		Expect(combinedOutput).ToNot(ContainSubstring("Set the default Podman connection to this machine"), "should not prompt when stdin is not a tty")

		inspect := new(inspectMachine)
		inspect = inspect.withFormat("{{.State}}")
		inspectSession, err := mb.setName(startme).setCmd(inspect).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal(define.Running))

		inspect2 := new(inspectMachine)
		inspect2 = inspect2.withFormat("{{.State}}")
		inspectSession2, err := mb.setName(dontstartme).setCmd(inspect2).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession2).To(Exit(0))
		Expect(inspectSession2.outputToString()).To(Not(Equal(define.Running)))
	})

	It("start two machines in parallel", func() {
		i := initMachine{}
		machine1 := "m1-" + randomString()
		session, err := mb.setName(machine1).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		machine2 := "m2-" + randomString()
		session, err = mb.setName(machine2).setCmd(i.withImage(mb.imagePath)).run()
		Expect(session).To(Exit(0))

		var startSession1, startSession2 *machineSession
		wg := sync.WaitGroup{}
		wg.Add(2)
		// now start two machine start process in parallel
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			s := &startMachine{}
			startSession1, err = mb.setName(machine1).setCmd(s.withUpdateConnection(ptrBool(false))).setTimeout(time.Minute * 10).run()
			Expect(err).ToNot(HaveOccurred())
		}()
		go func() {
			defer GinkgoRecover()
			defer wg.Done()
			s := &startMachine{}
			// ok this is a hack and should not be needed but the way these test are setup they all
			// share "mb" which stores the name that is used for the VM, thus running two parallel
			// can overwrite the name from the other, work around that by creating a new mb for the
			// second run.
			nmb, err := newMB()
			Expect(err).ToNot(HaveOccurred())
			startSession2, err = nmb.setName(machine2).setCmd(s.withUpdateConnection(ptrBool(false))).setTimeout(time.Minute * 10).run()
			Expect(err).ToNot(HaveOccurred())
		}()
		wg.Wait()

		// WSL can start in parallel so just check both command exit 0 there
		if testProvider.VMType() == define.WSLVirt {
			Expect(startSession1).To(Exit(0))
			Expect(startSession2).To(Exit(0))
			return
		}
		// other providers have a check that only one VM can be running at any given time so make sure our check is race free
		Expect(startSession1).To(Or(Exit(0), Exit(125)), "start command should succeed or fail with 125")
		if startSession1.ExitCode() == 0 {
			Expect(startSession2).To(Exit(125), "first start worked, second start must fail")
			Expect(startSession2.errorToString()).To(ContainSubstring("%s already starting or running: only one VM can be active at a time", machine1))
		} else {
			Expect(startSession2).To(Exit(0), "first start failed, second start succeed")
			Expect(startSession1.errorToString()).To(ContainSubstring("%s already starting or running: only one VM can be active at a time", machine2))
		}
	})

	It("machine start with --update-connection", func() {
		// Add a connection and verify it was set to the default
		defConnName := "QA"
		err := addSystemConnection(defConnName, true)
		Expect(err).ToNot(HaveOccurred())

		listings, err := getSystemConnectionsAsSysConns()
		Expect(err).ToNot(HaveOccurred())
		Expect(listings.IsDefault(defConnName)).To(BeTrue())

		// Create a new machine
		i := initMachine{}
		machineName := randomString()
		initSession, err := mb.setName(machineName).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(initSession).To(Exit(0))

		// Start the new machine with --update-connection=false
		s := startMachine{}
		startSession, err := mb.setName(machineName).setCmd(s.withUpdateConnection(ptrBool(false))).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		// We started the machine with --update-connection=false so it should not be default
		listings, err = getSystemConnectionsAsSysConns()
		Expect(err).ToNot(HaveOccurred())
		Expect(listings.IsDefault(defConnName)).To(BeTrue())

		// Stop the machine
		halt := stopMachine{}
		stopSession, err := mb.setName(machineName).setCmd(halt).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// Start the new machine with --update-connection
		startSession, err = mb.setName(machineName).setCmd(s.withUpdateConnection(ptrBool(true))).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		// We set true so the new default connection should have changed
		listings, err = getSystemConnectionsAsSysConns()
		Expect(err).ToNot(HaveOccurred())
		Expect(listings.IsDefault(machineName)).To(BeTrue())
	})
	It("machine init --now with --update-connection", func() {
		// Add a connection and verify it was set to the default
		defConnName := "QA"
		err := addSystemConnection(defConnName, true)
		Expect(err).ToNot(HaveOccurred())

		listings, err := getSystemConnectionsAsSysConns()
		Expect(err).ToNot(HaveOccurred())
		Expect(listings.IsDefault(defConnName)).To(BeTrue())

		// Create a new machine
		i := initMachine{}
		machineName1 := randomString()
		initSession, err := mb.setName(machineName1).setCmd(i.withImage(mb.imagePath).withUpdateConnection(ptrBool(false)).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(initSession).To(Exit(0))

		// We started the machine with --update-connection=false so it should not be default
		listings, err = getSystemConnectionsAsSysConns()
		Expect(err).ToNot(HaveOccurred())
		Expect(listings.IsDefault(defConnName)).To(BeTrue())

		// Stop the machine
		halt := stopMachine{}
		stopSession, err := mb.setName(machineName1).setCmd(halt).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// Create another machine
		machineName2 := randomString()
		initSession2, err := mb.setName(machineName2).setCmd(i.withImage(mb.imagePath).withUpdateConnection(ptrBool(true)).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(initSession2).To(Exit(0))

		listings, err = getSystemConnectionsAsSysConns()
		Expect(err).ToNot(HaveOccurred())
		Expect(listings.IsDefault(machineName2)).To(BeTrue())
	})
	It("machine init --now with --import-native-ca with mounted data folder", func() {
		// Create a new machine
		i := initMachine{}
		initCommand := i.withImage(mb.imagePath).withImportNativeCA(true).withNow()
		m := randomString()
		initSession, err := mb.setName(m).setCmd(initCommand).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(initSession).To(Exit(0))

		// Verify that the file in the guest exist
		certFilePath := "/etc/pki/ca-trust/source/anchors"
		certFileName := "host-ca-certs.pem"
		sshMachine := sshMachine{}
		sshCertFile, err := mb.setName(m).setCmd(sshMachine.withSSHCommand([]string{"ls", certFilePath})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshCertFile).To(Exit(0))
		Expect(sshCertFile.outputToString()).To(Equal(certFileName))
	})
	It("machine init --now with --import-native-ca with SCP file transfer", func() {
		skipIfVmtype(define.WSLVirt, "WSL doesn't allow handling volumes (the machine data folder is always mounted")

		// Create a new machine
		i := initMachine{}
		initCommand := i.withImage(mb.imagePath).withImportNativeCA(true).withNow()
		// Don't mount any volume to force the transfer the certificates file via SCP
		initCommand = initCommand.withVolume("")
		m := randomString()
		initSession, err := mb.setName(m).setCmd(initCommand).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(initSession).To(Exit(0))

		certFilePath := "/etc/pki/ca-trust/source/anchors"
		certFileName := "host-ca-certs.pem"
		sshMachine := sshMachine{}
		sshCertFile, err := mb.setName(m).setCmd(sshMachine.withSSHCommand([]string{"ls", certFilePath})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshCertFile).To(Exit(0))
		Expect(sshCertFile.outputToString()).To(Equal(certFileName))
	})
	It("start interrupted by SIGTERM while waiting for VM start", func() {
		if !isVmtype(define.AppleHvVirt) && !isVmtype(define.LibKrun) {
			Skip("SIGTERM interruption is supported on macOS only")
		}
		// Use the provider binary to pgrep the VM process
		var vmProcess string
		switch testProvider.VMType() {
		case define.AppleHvVirt:
			vmProcess = "vfkit"
		case define.LibKrun:
			vmProcess = "krunkit"
		default:
		}

		i := new(initMachine)
		initSession, err := mb.setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(initSession).To(Exit(0))

		s := new(startMachine)
		startSession, err := mb.setCmd(s).runWithoutWait()
		Expect(err).ToNot(HaveOccurred())

		// Wait 45s for the VM process spawned by `podman machine start`
		Eventually(func() error {
			_, err := exec.Command("pgrep", vmProcess).Output()
			return err
		}, 45*time.Second, 500*time.Millisecond).Should(Succeed())

		// Send a term signal now (podman machine start is waiting for
		// the VM process readiness)
		startSession.Signal(syscall.SIGTERM)

		// Wait 30s for the podman machine start process to return (no deadlock)
		Eventually(startSession, 30*time.Second).Should(Exit())
		Expect(startSession.ExitCode()).ToNot(Equal(0))

		// Wait 30s for the VM process to return (SIGTERM has been forwarded)
		Eventually(func() error {
			_, err := exec.Command("pgrep", vmProcess).Output()
			return err
		}, 30*time.Second, 500*time.Millisecond).ShouldNot(Succeed())

		// Verify machine state is Stopped
		inspect := new(inspectMachine)
		inspectSession, err := mb.setCmd(inspect.withFormat("{{.State}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal(define.Stopped))

		// Verify no orphan gvproxy
		_, err = pgrep("gvproxy")
		Expect(err).To(HaveOccurred(), "gvproxy should not be running after SIGTERM cleanup")

		// Restart without interrupting and confirm that completes without error
		startSession, err = mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))
	})
})

func mapToPort(uris []string) ([]string, error) {
	ports := []string{}

	for _, uri := range uris {
		u, err := url.Parse(uri)
		if err != nil {
			return nil, err
		}

		port := u.Port()
		if port == "" {
			return nil, fmt.Errorf("no port in URI: %s", uri)
		}

		ports = append(ports, port)
	}
	return ports, nil
}

func addSystemConnection(name string, setDefault bool) error {
	addConn := []string{
		"system", "connection", "add",
		fmt.Sprintf("--default=%s", strconv.FormatBool(setDefault)),
		"--identity", "~/.ssh/id_rsa",
		name,
		"ssh://root@podman.test:2222/run/podman/podman.sock",
	}
	mb.cmd = addConn
	addConnSession, err := mb.run()
	if err != nil {
		return err
	}
	if addConnSession.ExitCode() != 0 {
		fmt.Println(addConnSession.outputToString())
		return fmt.Errorf("error: %s", addConnSession.errorToString())
	}
	return nil
}

func systemConnectionLsToSysConns(output []byte) (SysConns, error) {
	var conns SysConns
	err := jsoniter.Unmarshal(output, &conns)
	return conns, err
}

type SysConn struct {
	Name      string
	URI       string
	Identity  string
	IsMachine bool
	Default   bool
	ReadWrite bool
}

type SysConns []SysConn

func (s SysConns) IsDefault(name string) bool {
	for _, conn := range s {
		if conn.Name == name {
			return conn.Default
		}
	}
	return false
}

func (s SysConns) GetDefault() (SysConn, error) {
	for _, conn := range s {
		if conn.Default {
			return conn, nil
		}
	}
	return SysConn{}, fmt.Errorf("no default connection found")
}

func getSystemConnectionsAsSysConns() (SysConns, error) {
	connections := new(listSystemConnection)
	connSession, err := mb.setCmd(connections.withFormat("json")).run()
	if err != nil {
		return nil, err
	}
	if connSession.ExitCode() != 0 {
		return nil, fmt.Errorf("error: %s", connSession.errorToString())
	}
	return systemConnectionLsToSysConns(connSession.Out.Contents())
}
