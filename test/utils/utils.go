package utils

import (
	"bufio"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	crypto_rand "crypto/rand"

	. "github.com/onsi/ginkgo/v2"    //nolint:staticcheck // ST1001: should not use dot imports
	. "github.com/onsi/gomega"       //nolint:staticcheck // ST1001: should not use dot imports
	. "github.com/onsi/gomega/gexec" //nolint:staticcheck // ST1001: should not use dot imports
)

const (
	// Env variable for creating time files.
	EnvTimeDir = "_PODMAN_TIME_DIR"
)

var (
	DefaultWaitTimeout   = 90
	OSReleasePath        = "/etc/os-release"
	ProcessOneCgroupPath = "/proc/1/cgroup"
)

// PodmanTestCommon contains common functions will be updated later in
// the inheritance structs
type PodmanTestCommon interface {
	MakeOptions(args []string, options PodmanExecOptions) []string
	WaitContainerReady(id string, expStr string, timeout int, step int) bool
}

// PodmanTest struct for command line options
type PodmanTest struct {
	ImageCacheDir           string
	ImageCacheFS            string
	DatabaseBackend         string
	PodmanBinary            string
	PodmanMakeOptions       func(args []string, options PodmanExecOptions) []string
	RemoteCommand           *exec.Cmd
	RemotePodmanBinary      string
	RemoteSession           *os.Process
	RemoteSocket            string
	RemoteSocketScheme      string
	RemoteSocketLock        string // If not "", should be removed _after_ RemoteSocket is removed
	RemoteTLSClientCAFile   string
	RemoteTLSClientCAPool   *x509.CertPool
	RemoteTLSClientCerts    []tls.Certificate
	RemoteTLSServerCertFile string
	RemoteTLSServerKeyFile  string
	RemoteTLSServerCAFile   string
	RemoteTLSServerCAPool   *x509.CertPool
	RemoteTLSClientCertFile string
	RemoteTLSClientKeyFile  string
	RemoteTLSDetails        string
	RemoteTest              bool
	TempDir                 string // TempDir is a unique per test directory.
}

// PodmanSession wraps the gexec.session so we can extend it
type PodmanSession struct {
	*Session
}

// HostOS is a simple struct for the test os
type HostOS struct {
	Distribution string
	Version      string
	Arch         string
}

// MakeOptions assembles all podman options
func (p *PodmanTest) MakeOptions(args []string, options PodmanExecOptions) []string {
	return p.PodmanMakeOptions(args, options)
}

// PodmanExecOptions modify behavior of PodmanTest.PodmanExecBaseWithOptions and its callers.
// Users should typically leave most fields default-initialized, and only set those that are relevant to them.
type PodmanExecOptions struct {
	CWD              string   // default: inherited form the current process
	Env              []string // default: inherited form the current process
	NoEvents         bool
	NoCache          bool
	Wrapper          []string  // A command to run, receiving the Podman command line. default: none
	FullOutputWriter io.Writer // Receives the full output (stdout+stderr) of the command, in _approximately_ correct order. default: GinkgoWriter
	ExtraFiles       []*os.File
}

// PodmanExecBaseWithOptions execs podman with the specified args, and in an environment defined by options
func (p *PodmanTest) PodmanExecBaseWithOptions(args []string, options PodmanExecOptions) *PodmanSession {
	var command *exec.Cmd
	podmanOptions := p.MakeOptions(args, options)
	podmanBinary := p.PodmanBinary
	if p.RemoteTest {
		podmanBinary = p.RemotePodmanBinary
	}

	runCmd := options.Wrapper
	if timeDir := os.Getenv(EnvTimeDir); timeDir != "" {
		timeFile, err := os.CreateTemp(timeDir, ".time")
		if err != nil {
			Fail(fmt.Sprintf("Error creating time file: %v", err))
		}
		timeArgs := []string{"-f", "%M", "-o", timeFile.Name()}
		timeCmd := append([]string{"/usr/bin/time"}, timeArgs...)
		runCmd = append(timeCmd, runCmd...)
	}
	runCmd = append(runCmd, podmanBinary)

	if options.Env == nil {
		GinkgoWriter.Printf("Running: %s %s\n", strings.Join(runCmd, " "), strings.Join(podmanOptions, " "))
	} else {
		GinkgoWriter.Printf("Running: (env: %v) %s %s\n", options.Env, strings.Join(runCmd, " "), strings.Join(podmanOptions, " "))
	}

	runCmd = append(runCmd, podmanOptions...)
	command = exec.Command(runCmd[0], runCmd[1:]...)
	if options.Env != nil {
		command.Env = options.Env
	}
	if options.CWD != "" {
		command.Dir = options.CWD
	}

	command.ExtraFiles = options.ExtraFiles

	var fullOutputWriter io.Writer = GinkgoWriter
	if options.FullOutputWriter != nil {
		fullOutputWriter = options.FullOutputWriter
	}
	session, err := Start(command, fullOutputWriter, fullOutputWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run podman command: %s\n%v", strings.Join(podmanOptions, " "), err))
	}
	return &PodmanSession{session}
}

// NumberOfContainersRunning returns an int of how many
// containers are currently running.
func (p *PodmanTest) NumberOfContainersRunning() int {
	var containers []string
	ps := p.PodmanExecBaseWithOptions([]string{"ps", "-q"}, PodmanExecOptions{
		NoCache: true,
	})
	ps.WaitWithDefaultTimeout()
	Expect(ps).Should(Exit(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			containers = append(containers, i)
		}
	}
	return len(containers)
}

// NumberOfContainers returns an int of how many
// containers are currently defined.
func (p *PodmanTest) NumberOfContainers() int {
	var containers []string
	ps := p.PodmanExecBaseWithOptions([]string{"ps", "-aq"}, PodmanExecOptions{
		NoCache: true,
	})
	ps.WaitWithDefaultTimeout()
	Expect(ps.ExitCode()).To(Equal(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			containers = append(containers, i)
		}
	}
	return len(containers)
}

// NumberOfPods returns an int of how many
// pods are currently defined.
func (p *PodmanTest) NumberOfPods() int {
	var pods []string
	ps := p.PodmanExecBaseWithOptions([]string{"pod", "ps", "-q"}, PodmanExecOptions{
		NoCache: true,
	})
	ps.WaitWithDefaultTimeout()
	Expect(ps.ExitCode()).To(Equal(0))
	for _, i := range ps.OutputToStringArray() {
		if i != "" {
			pods = append(pods, i)
		}
	}
	return len(pods)
}

// GetContainerStatus returns the containers state.
// This function assumes only one container is active.
func (p *PodmanTest) GetContainerStatus() string {
	podmanArgs := []string{"ps", "--all", "--format={{.Status}}"}
	session := p.PodmanExecBaseWithOptions(podmanArgs, PodmanExecOptions{
		NoCache: true,
	})
	session.WaitWithDefaultTimeout()
	return session.OutputToString()
}

// WaitContainerReady waits process or service inside container start, and ready to be used.
func (p *PodmanTest) WaitContainerReady(id string, expStr string, timeout int, step int) bool {
	startTime := time.Now()
	s := p.PodmanExecBaseWithOptions([]string{"logs", id}, PodmanExecOptions{
		NoCache: true,
	})
	s.WaitWithDefaultTimeout()

	for {
		if strings.Contains(s.OutputToString(), expStr) || strings.Contains(s.ErrorToString(), expStr) {
			return true
		}

		if time.Since(startTime) >= time.Duration(timeout)*time.Second {
			GinkgoWriter.Printf("Container %s is not ready in %ds", id, timeout)
			return false
		}
		time.Sleep(time.Duration(step) * time.Second)
		s = p.PodmanExecBaseWithOptions([]string{"logs", id}, PodmanExecOptions{
			NoCache: true,
		})
		s.WaitWithDefaultTimeout()
	}
}

// WaitContainerReady is a wrapper function for accept inheritance PodmanTest struct.
func WaitContainerReady(p PodmanTestCommon, id string, expStr string, timeout int, step int) bool {
	return p.WaitContainerReady(id, expStr, timeout, step)
}

// OutputToString formats session output to string
func (s *PodmanSession) OutputToString() string {
	if s == nil || s.Out == nil || s.Out.Contents() == nil {
		return ""
	}

	fields := strings.Fields(string(s.Out.Contents()))
	return strings.Join(fields, " ")
}

// OutputToStringArray returns the output as a []string
// where each array item is a line split by newline
func (s *PodmanSession) OutputToStringArray() []string {
	var results []string
	output := string(s.Out.Contents())
	for line := range strings.SplitSeq(output, "\n") {
		if line != "" {
			results = append(results, line)
		}
	}
	return results
}

// ErrorToString formats session stderr to string
func (s *PodmanSession) ErrorToString() string {
	fields := strings.Fields(string(s.Err.Contents()))
	return strings.Join(fields, " ")
}

// ErrorToStringArray returns the stderr output as a []string
// where each array item is a line split by newline
func (s *PodmanSession) ErrorToStringArray() []string {
	output := string(s.Err.Contents())
	return strings.Split(output, "\n")
}

// GrepString takes session output and behaves like grep. it returns a bool
// if successful and an array of strings on positive matches
func (s *PodmanSession) GrepString(term string) (bool, []string) {
	var (
		greps   []string
		matches bool
	)

	for _, line := range s.OutputToStringArray() {
		if strings.Contains(line, term) {
			matches = true
			greps = append(greps, line)
		}
	}
	return matches, greps
}

// LineInOutputContains returns true if a line in a
// session output contains the supplied string
func (s *PodmanSession) LineInOutputContains(term string) bool {
	for _, i := range s.OutputToStringArray() {
		if strings.Contains(i, term) {
			return true
		}
	}
	return false
}

// LineInOutputContainsTag returns true if a line in the
// session's output contains the repo-tag pair as returned
// by podman-images(1).
func (s *PodmanSession) LineInOutputContainsTag(repo, tag string) bool {
	tagMap := tagOutputToMap(s.OutputToStringArray())
	return tagMap[repo][tag]
}

// IsJSONOutputValid attempts to unmarshal the session buffer
// and if successful, returns true, else false
func (s *PodmanSession) IsJSONOutputValid() bool {
	var i any
	if err := json.Unmarshal(s.Out.Contents(), &i); err != nil {
		GinkgoWriter.Println(err)
		return false
	}
	return true
}

// WaitWithDefaultTimeout waits for process finished with DefaultWaitTimeout
func (s *PodmanSession) WaitWithDefaultTimeout() {
	s.WaitWithTimeout(DefaultWaitTimeout)
}

// WaitWithTimeout waits for process finished with DefaultWaitTimeout
func (s *PodmanSession) WaitWithTimeout(timeout int) {
	Eventually(s, timeout).Should(Exit(), func() string {
		// Note eventually does not kill the command as such the command is leaked forever without killing it
		// Also let's use SIGABRT to create a go stack trace so in case there is a deadlock we see it.
		s.Signal(syscall.SIGABRT)
		// Give some time to let the command print the output so it is not printed much later
		// in the log at the wrong place.
		time.Sleep(1 * time.Second)
		// As the output is logged by default there no need to dump it here.
		return fmt.Sprintf("command timed out after %ds: %v",
			timeout, s.Command.Args)
	})
	os.Stdout.Sync()
	os.Stderr.Sync()
}

// SystemExec is used to exec a system command to check its exit code or output
func SystemExec(command string, args []string) *PodmanSession {
	c := exec.Command(command, args...)
	GinkgoWriter.Println("Execing " + c.String() + "\n")
	session, err := Start(c, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run command: %s %s", command, strings.Join(args, " ")))
	}
	session.Wait(DefaultWaitTimeout)
	return &PodmanSession{session}
}

// StartSystemExec is used to start exec a system command
func StartSystemExec(command string, args []string) *PodmanSession {
	c := exec.Command(command, args...)
	GinkgoWriter.Println("Execing " + c.String() + "\n")
	session, err := Start(c, GinkgoWriter, GinkgoWriter)
	if err != nil {
		Fail(fmt.Sprintf("unable to run command: %s %s", command, strings.Join(args, " ")))
	}
	return &PodmanSession{session}
}

// tagOutputToMap parses each string in imagesOutput and returns
// a map whose key is a repo, and value is another map whose keys
// are the tags found for that repo. Notice, the first array item will
// be skipped as it's considered to be the header.
func tagOutputToMap(imagesOutput []string) map[string]map[string]bool {
	m := make(map[string]map[string]bool)
	// iterate over output but skip the header
	for _, i := range imagesOutput[1:] {
		tmp := []string{}
		for x := range strings.SplitSeq(i, " ") {
			if x != "" {
				tmp = append(tmp, x)
			}
		}
		// podman-images(1) return a list like output
		// in the format of "Repository Tag [...]"
		if len(tmp) < 2 {
			continue
		}
		if m[tmp[0]] == nil {
			m[tmp[0]] = map[string]bool{}
		}
		m[tmp[0]][tmp[1]] = true
	}
	return m
}

// GetHostDistributionInfo returns a struct with its distribution Name and version
func GetHostDistributionInfo() HostOS {
	f, err := os.Open(OSReleasePath)
	if err != nil {
		return HostOS{}
	}
	defer f.Close()

	l := bufio.NewScanner(f)
	host := HostOS{}
	host.Arch = runtime.GOARCH
	for l.Scan() {
		if strings.HasPrefix(l.Text(), "ID=") {
			host.Distribution = strings.ReplaceAll(strings.TrimSpace(strings.Join(strings.Split(l.Text(), "=")[1:], "")), "\"", "")
		}
		if strings.HasPrefix(l.Text(), "VERSION_ID=") {
			host.Version = strings.ReplaceAll(strings.TrimSpace(strings.Join(strings.Split(l.Text(), "=")[1:], "")), "\"", "")
		}
	}
	return host
}

// IsCommandAvailable check if command exist
func IsCommandAvailable(command string) bool {
	check := exec.Command("bash", "-c", strings.Join([]string{"command -v", command}, " "))
	err := check.Run()
	return err == nil
}

// WriteJSONFile write json format data to a json file
func WriteJSONFile(data []byte, filePath string) error {
	var jsonData map[string]any
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return err
	}
	formatJSON, err := json.MarshalIndent(jsonData, "", "	")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, formatJSON, 0o644)
}

// Containerized check the podman command run inside container
func Containerized() bool {
	container := os.Getenv("container")
	if container != "" {
		return true
	}
	b, err := os.ReadFile(ProcessOneCgroupPath)
	if err != nil {
		// shrug, if we cannot read that file, return false
		return false
	}
	return strings.Contains(string(b), "docker")
}

var randomLetters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// RandomString returns a string of given length composed of random characters
func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = randomLetters[rand.Intn(len(randomLetters))]
	}
	return string(b)
}

// Encode *rsa.PublicKey and store it in a file.
// Adds appropriate extension to the fileName, and returns the complete fileName of
// the file storing the public key.
func savePublicKey(fileName string, publicKey *rsa.PublicKey) (string, error) {
	// Encode public key to PKIX, ASN.1 DER form
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	pubPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: pubBytes,
		},
	)

	// Write public key to file
	publicKeyFileName := fileName + ".rsa.pub"
	if err := os.WriteFile(publicKeyFileName, pubPEM, 0o600); err != nil {
		return "", err
	}

	return publicKeyFileName, nil
}

// Encode *rsa.PrivateKey and store it in a file.
// Adds appropriate extension to the fileName, and returns the complete fileName of
// the file storing the private key.
func savePrivateKey(fileName string, privateKey *rsa.PrivateKey) (string, error) {
	// Encode private key to PKCS#1, ASN.1 DER form
	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: privBytes,
		},
	)

	// Write private key to file
	privateKeyFileName := fileName + ".rsa"
	if err := os.WriteFile(privateKeyFileName, keyPEM, 0o600); err != nil {
		return "", err
	}

	return privateKeyFileName, nil
}

// Generate RSA key pair of specified bit size and write them to files.
// Adds appropriate extension to the fileName, and returns the complete fileName of
// the files storing the public and private key respectively.
func WriteRSAKeyPair(fileName string, bitSize int) (string, string, error) {
	// Generate RSA key
	privateKey, err := rsa.GenerateKey(crypto_rand.Reader, bitSize)
	if err != nil {
		return "", "", err
	}

	publicKey := privateKey.Public().(*rsa.PublicKey)

	publicKeyFileName, err := savePublicKey(fileName, publicKey)
	if err != nil {
		return "", "", err
	}

	privateKeyFileName, err := savePrivateKey(fileName, privateKey)
	if err != nil {
		return "", "", err
	}

	return publicKeyFileName, privateKeyFileName, nil
}
