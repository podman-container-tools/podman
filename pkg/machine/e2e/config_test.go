package e2e_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	. "github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/types"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/storage/pkg/stringid"
)

var originalHomeDir = os.Getenv("HOME")

const (
	defaultTimeout = 10 * time.Minute
)

type machineCommand interface {
	buildCmd(m *machineTestBuilder) []string
}

type MachineTestBuilder interface {
	setName(name string) *MachineTestBuilder
	setCmd(mc machineCommand) *MachineTestBuilder
	setTimeout(duration time.Duration) *MachineTestBuilder
	run() (*machineSession, error)
}
type machineSession struct {
	*Session
}

type machineTestBuilder struct {
	cmd          []string
	stdin        io.Reader
	imagePath    string
	name         string
	names        []string
	podmanBinary string
	timeout      time.Duration
}

// waitWithTimeout waits for a command to complete for a given
// number of seconds
func (ms *machineSession) waitWithTimeout(timeout time.Duration) {
	Eventually(ms, timeout).Should(Exit(), func() string {
		// Note eventually does not kill the command as such the command is leaked forever without killing it
		// Also let's use SIGABRT to create a go stack trace so in case there is a deadlock we see it.
		ms.Signal(syscall.SIGABRT)
		// Give some time to let the command print the output so it is not printed much later
		// in the log at the wrong place.
		time.Sleep(1 * time.Second)
		return fmt.Sprintf("command timed out after %fs: %v",
			timeout.Seconds(), ms.Command.Args)
	})
}

func (ms *machineSession) Bytes() []byte {
	return []byte(ms.outputToString())
}

func (ms *machineSession) outputToStringSlice() []string {
	var results []string
	output := string(ms.Out.Contents())
	for line := range strings.SplitSeq(output, "\n") {
		if line != "" {
			results = append(results, line)
		}
	}
	return results
}

// outputToString returns the output from a session in string form
func (ms *machineSession) outputToString() string {
	if ms == nil || ms.Out == nil || ms.Out.Contents() == nil {
		return ""
	}

	fields := strings.Fields(string(ms.Out.Contents()))
	return strings.Join(fields, " ")
}

// errorToString returns the error output from a session in string form
func (ms *machineSession) errorToString() string {
	if ms == nil || ms.Err == nil || ms.Err.Contents() == nil {
		return ""
	}
	return string(ms.Err.Contents())
}

// newMB constructor for machine test builders
func newMB() (*machineTestBuilder, error) {
	mb := machineTestBuilder{
		timeout: defaultTimeout,
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	mb.podmanBinary = filepath.Join(cwd, podmanBinary)
	if os.Getenv("PODMAN_BINARY") != "" {
		mb.podmanBinary = os.Getenv("PODMAN_BINARY")
	}
	if os.Getenv("MACHINE_TEST_TIMEOUT") != "" {
		seconds, err := strconv.Atoi(os.Getenv("MACHINE_TEST_TIMEOUT"))
		if err != nil {
			return nil, err
		}
		mb.timeout = time.Duration(seconds) * time.Second
	}
	return &mb, nil
}

// setName sets the name of the virtuaql machine for the command
func (m *machineTestBuilder) setName(name string) *machineTestBuilder {
	m.name = name
	return m
}

// setCmd takes a machineCommand struct and assembles a cmd line
// representation of the podman machine command
func (m *machineTestBuilder) setCmd(mc machineCommand) *machineTestBuilder {
	// If no name for the machine exists, we set a random name.
	if !slices.Contains(m.names, m.name) {
		if len(m.name) < 1 {
			m.name = randomString()
		}
		m.names = append(m.names, m.name)
	}
	m.cmd = mc.buildCmd(m)
	m.stdin = nil
	return m
}

// setStdin sets the stdin for the next command to be run
func (m *machineTestBuilder) setStdin(data io.Reader) *machineTestBuilder {
	m.stdin = data
	return m
}

func (m *machineTestBuilder) setTimeout(timeout time.Duration) *machineTestBuilder { //nolint: unparam
	m.timeout = timeout
	return m
}

// toQemuInspectInfo is only for inspecting qemu machines.  Other providers will need
// to make their own.
func (m *machineTestBuilder) toQemuInspectInfo() ([]machine.InspectInfo, int, error) {
	args := []string{"machine", "inspect"}
	args = append(args, m.names...)
	session, err := runWrapper(m.podmanBinary, args, nil, defaultTimeout, true)
	if err != nil {
		return nil, -1, err
	}
	mii := []machine.InspectInfo{}
	err = json.Unmarshal(session.Bytes(), &mii)
	return mii, session.ExitCode(), err
}

func (m *machineTestBuilder) runWithoutWait() (*machineSession, error) {
	return runWrapper(m.podmanBinary, m.cmd, m.stdin, m.timeout, false)
}

func (m *machineTestBuilder) run() (*machineSession, error) {
	s, err := runWrapper(m.podmanBinary, m.cmd, m.stdin, m.timeout, true)
	return s, err
}

func runWrapper(podmanBinary string, cmdArgs []string, stdinData io.Reader, timeout time.Duration, wait bool) (*machineSession, error) {
	if len(os.Getenv("DEBUG")) > 0 {
		cmdArgs = append([]string{"--log-level=debug"}, cmdArgs...)
	}
	GinkgoWriter.Println(podmanBinary + " " + strings.Join(cmdArgs, " "))
	c := exec.Command(podmanBinary, cmdArgs...)
	if stdinData != nil {
		c.Stdin = stdinData
	}

	session, err := Start(c, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("Unable to start session: %q", err))
		return nil, err
	}
	ms := machineSession{session}
	if wait {
		ms.waitWithTimeout(timeout)
	}
	return &ms, nil
}

// randomString returns a string of given length composed of random characters
func randomString() string {
	return stringid.GenerateRandomID()[0:12]
}

type ValidJSONMatcher struct {
	types.GomegaMatcher
}

func BeValidJSON() *ValidJSONMatcher {
	return &ValidJSONMatcher{}
}

func (matcher *ValidJSONMatcher) Match(actual any) (success bool, err error) {
	s, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("ValidJSONMatcher expects a string, not %q", actual)
	}

	var i any
	if err := json.Unmarshal([]byte(s), &i); err != nil {
		return false, err
	}
	return true, nil
}

func (matcher *ValidJSONMatcher) FailureMessage(actual any) (message string) {
	return format.Message(actual, "to be valid JSON")
}

func (matcher *ValidJSONMatcher) NegatedFailureMessage(actual any) (message string) {
	return format.Message(actual, "to _not_ be valid JSON")
}

func skipIfVmtype(vmType define.VMType, message string) {
	if isVmtype(vmType) {
		Skip(message)
	}
}

func skipIfNotVmtype(vmType define.VMType, message string) {
	if !isVmtype(vmType) {
		Skip(message)
	}
}

func skipIfWSL(message string) {
	skipIfVmtype(define.WSLVirt, message)
}

func isVmtype(vmType define.VMType) bool {
	return testProvider.VMType() == vmType
}

// isWSL is a simple wrapper to determine if the testprovider is WSL
func isWSL() bool {
	return isVmtype(define.WSLVirt)
}
