package e2e_test

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
	It("bad start name", func() {
		i := startMachine{}
		reallyLongName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		session, err := mb.setName(reallyLongName).setCmd(&i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(125))
		Expect(session.errorToString()).To(ContainSubstring("VM does not exist"))
	})

	It("start machine already started and stop machine already stopped", func() {
		name := randomString()
		i := new(initMachine)
		machineTestBuilderInit := mb.setName(name).setCmd(i.withImage(mb.imagePath))
		session, err := machineTestBuilderInit.run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		starttime := time.Now()
		s := new(startMachine)
		// suppress output with no info and check for that.
		startSession, err := mb.setCmd(s.withNoInfo()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))
		Expect(startSession.outputToString()).ToNot(ContainSubstring("API forwarding"))

		info, ec, err := mb.toInspectInfo()
		Expect(err).ToNot(HaveOccurred())
		Expect(ec).To(BeZero())
		Expect(info[0].State).To(Equal(define.Running))

		startSession, err = mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(125))
		Expect(startSession.errorToString()).To(ContainSubstring(fmt.Sprintf("Error: unable to start %q: already running", machineTestBuilderInit.name)))

		stop := new(stopMachine)
		stopSession, err := mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// Stopping it again should not result in an error
		stopAgain, err := mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopAgain).To(Exit(0))
		Expect(stopAgain.outputToString()).To(ContainSubstring(fmt.Sprintf("Machine \"%s\" stopped successfully", name)))

		// Stopping a machine should update the last up time
		inspect := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(inspect.withFormat("{{.LastUp.Format \"2006-01-02T15:04:05Z07:00\"}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		lastupTime, err := time.Parse(time.RFC3339, inspectSession.outputToString())
		Expect(err).ToNot(HaveOccurred())
		Expect(lastupTime).To(BeTemporally(">", starttime))
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
		// Also test with quiet to ensure no extra stout is logged but the error is still logged.
		startSession, err := mb.setCmd(s.withQuiet()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))
		Expect(startSession.errorToString()).To(ContainSubstring("detected port conflict on machine ssh port"))
		Expect(startSession.outputToString()).To(Equal(fmt.Sprintf("Machine %q started successfully", mb.name)))

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

	It("start only starts specified machine and remove running machine", func() {
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
		session3, err := mb.setName(startme).setCmd(s).setStdin(stdinBuf).run()
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

		rm := new(rmMachine)
		// Removing a running machine should fail
		stop, err := mb.setName(startme).setCmd(rm).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stop).To(Exit(125))
		Expect(stop.errorToString()).To(ContainSubstring(fmt.Sprintf("vm \"%s\" cannot be destroyed", startme)))

		// Removing again with force should work
		stopAgain, err := mb.setCmd(rm.withForce()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopAgain).To(Exit(0))

		// Inspect to be sure it is gone
		inspect3 := new(inspectMachine)
		inspectSession3, err := mb.setName(startme).setCmd(inspect3).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession3).To(Exit(125))
		Expect(inspectSession3.errorToString()).To(ContainSubstring("VM does not exist"))
	})

	It("start two machines in parallel", func() {
		skipIfVmtype(define.AppleHvVirt, "parallel machine start is not supported on Apple Hypervisor")
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
			startSession1, err = mb.setName(machine1).setCmd(s.withUpdateConnection(new(false))).run()
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
			startSession2, err = nmb.setName(machine2).setCmd(s.withUpdateConnection(new(false))).run()
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
		startSession, err := mb.setName(machineName).setCmd(s.withUpdateConnection(new(false))).run()
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
		startSession, err = mb.setName(machineName).setCmd(s.withUpdateConnection(new(true))).run()
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
		initSession, err := mb.setName(machineName1).setCmd(i.withImage(mb.imagePath).withUpdateConnection(new(false)).withNow()).run()
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
		initSession2, err := mb.setName(machineName2).setCmd(i.withImage(mb.imagePath).withUpdateConnection(new(true)).withNow()).run()
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
		_, err = pgrep(gvproxy)
		Expect(err).To(HaveOccurred(), "gvproxy should not be running after SIGTERM cleanup")

		// Restart without interrupting and confirm that completes without error
		startSession, err = mb.setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))
	})

	It("start recovers from stale Starting state", func() {
		name := randomString()
		i := new(initMachine)
		initSession, err := mb.setName(name).setCmd(i.withImage(mb.imagePath)).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(initSession).To(Exit(0))

		// Verify machine state is Stopped (init does not start the machine)
		inspect := new(inspectMachine)
		inspectSession, err := mb.setName(name).setCmd(inspect.withFormat("{{.State}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession).To(Exit(0))
		Expect(inspectSession.outputToString()).To(Equal(define.Stopped))

		// Get the machine config file path
		inspect2 := new(inspectMachine)
		inspectSession2, err := mb.setName(name).setCmd(inspect2.withFormat("{{.ConfigDir.Path}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession2).To(Exit(0))
		configDir := inspectSession2.outputToString()
		configPath := filepath.Join(configDir, name+".json")

		// Read the config file
		configContent, err := os.ReadFile(configPath)
		Expect(err).ToNot(HaveOccurred())

		// Parse the JSON config
		var config map[string]any
		err = jsoniter.Unmarshal(configContent, &config)
		Expect(err).ToNot(HaveOccurred())

		// Verify Starting is false in the persisted config
		Expect(config["Starting"]).To(BeFalse())

		// Set Starting=true to simulate stale persisted state
		config["Starting"] = true

		// Write the modified config back
		modifiedConfig, err := jsoniter.Marshal(config)
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(configPath, modifiedConfig, 0o644)
		Expect(err).ToNot(HaveOccurred())

		// Verify machine is still Stopped (actual provider state hasn't changed)
		inspect3 := new(inspectMachine)
		inspectSession3, err := mb.setName(name).setCmd(inspect3.withFormat("{{.State}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession3).To(Exit(0))
		Expect(inspectSession3.outputToString()).To(Equal(define.Stopped))

		// Now attempt to start the machine - the stale Starting state should be recovered
		s := new(startMachine)
		startSession, err := mb.setName(name).setCmd(s).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(startSession).To(Exit(0))

		// Verify the machine is now running
		inspect4 := new(inspectMachine)
		inspectSession4, err := mb.setName(name).setCmd(inspect4.withFormat("{{.State}}")).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(inspectSession4).To(Exit(0))
		Expect(inspectSession4.outputToString()).To(Equal(define.Running))

		// Read the config file again to verify Starting was cleared
		configContent2, err := os.ReadFile(configPath)
		Expect(err).ToNot(HaveOccurred())
		var config2 map[string]any
		err = jsoniter.Unmarshal(configContent2, &config2)
		Expect(err).ToNot(HaveOccurred())
		Expect(config2["Starting"]).To(BeFalse())

		// Clean up
		stop := new(stopMachine)
		stopSession, err := mb.setName(name).setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))
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
