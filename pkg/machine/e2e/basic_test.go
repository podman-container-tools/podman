package e2e_test

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/version/rawversion"
	"go.podman.io/storage/pkg/configfile"
)

const TESTIMAGE = "quay.io/libpod/testimage:20241011"

var _ = Describe("run basic podman commands", func() {
	It("Basic ops", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// Check that we mount the host config dir to the machine /etc/containers and and can write content from the host there.
		path, err := configfile.UserConfigPath()
		Expect(err).ToNot(HaveOccurred())
		err = os.MkdirAll(path, 0o755)
		Expect(err).ToNot(HaveOccurred())

		content := randomString()
		f, err := os.Create(filepath.Join(path, "podman-machine-tmpfile"))
		Expect(err).ToNot(HaveOccurred())
		_, err = f.WriteString(content)
		Expect(err).ToNot(HaveOccurred())
		err = f.Close()
		Expect(err).ToNot(HaveOccurred())

		ssh := new(sshMachine).withSSHCommand([]string{"cat /etc/containers/podman-machine-tmpfile"})
		sshRun, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(sshRun).To(Exit(0))
		Expect(sshRun.outputToString()).To(Equal(content))

		// check some basic podman commands
		bm := basicMachine{}
		imgs, err := mb.setCmd(bm.withPodmanCommand([]string{"images", "-q"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(imgs).To(Exit(0))
		Expect(imgs.outputToStringSlice()).To(BeEmpty())

		newImgs, err := mb.setCmd(bm.withPodmanCommand([]string{"pull", TESTIMAGE})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(newImgs).To(Exit(0))
		Expect(newImgs.outputToStringSlice()).To(HaveLen(1))

		// seccomp option as regression test for https://github.com/containers/podman/issues/26855
		runAlp, err := mb.setCmd(bm.withPodmanCommand([]string{"run", "--security-opt", "seccomp=unconfined", TESTIMAGE, "cat", "/etc/os-release"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(runAlp).To(Exit(0))
		Expect(runAlp.outputToString()).To(ContainSubstring("Alpine Linux"))

		contextDir := GinkgoT().TempDir()
		cfile := filepath.Join(contextDir, "Containerfile")
		err = os.WriteFile(cfile, []byte("FROM "+TESTIMAGE+"\nRUN ip addr\n"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		build, err := mb.setCmd(bm.withPodmanCommand([]string{"build", contextDir})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(build).To(Exit(0))
		Expect(build.outputToString()).To(ContainSubstring("COMMIT"))

		rmCon, err := mb.setCmd(bm.withPodmanCommand([]string{"rm", "-a"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(rmCon).To(Exit(0))
	})

	It("Volume ops", func() {
		tDir, err := filepath.Abs(GinkgoT().TempDir())
		Expect(err).ToNot(HaveOccurred())
		roFile := filepath.Join(tDir, "attr-test-file")

		// Create the file as ready-only, since some platforms disallow selinux attr writes
		// The subsequent Z mount should still succeed in spite of that
		rf, err := os.OpenFile(roFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o444)
		Expect(err).ToNot(HaveOccurred())
		rf.Close()

		name := randomString()
		i := new(initMachine).withImage(mb.imagePath).withNow()

		// All other platforms have an implicit mount for the temp area
		if isVmtype(define.QemuVirt) {
			i.withVolume(tDir)
		}
		session, err := mb.setName(name).setCmd(i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		bm := basicMachine{}
		// Test relabel works on all platforms
		runAlp, err := mb.setCmd(bm.withPodmanCommand([]string{"run", "-v", tDir + ":/test:Z", TESTIMAGE, "ls", "/test/attr-test-file"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(runAlp).To(Exit(0))

		// Test overlay works on all platforms except Hyper-V (see #26210)
		if !isVmtype(define.HyperVVirt) {
			runAlp, err = mb.setCmd(bm.withPodmanCommand([]string{"run", "-v", tDir + ":/test:O", TESTIMAGE, "ls", "/test/attr-test-file"})).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(runAlp).To(Exit(0))
		}

		// Test build with --volume option
		cf := filepath.Join(tDir, "Containerfile")
		err = os.WriteFile(cf, []byte("FROM "+TESTIMAGE+"\nRUN ls /test/attr-test-file\n"), 0o644)
		Expect(err).ToNot(HaveOccurred())
		build, err := mb.setCmd(bm.withPodmanCommand([]string{"build", "-t", name, "-v", tDir + ":/test", tDir})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(build).To(Exit(0))

		// check we can use a single character volume name as mount
		volumeCreate, err := mb.setCmd(bm.withPodmanCommand([]string{"volume", "create", "a"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(volumeCreate).To(Exit(0))

		run, err := mb.setCmd(bm.withPodmanCommand([]string{"run", "-v", "a:/test:Z", TESTIMAGE, "true"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(run).To(Exit(0))

		if isVmtype(define.QemuVirt) {
			// ensure we are actually using virtiofs on linux
			ssh := new(sshMachine).withSSHCommand([]string{"findmnt", "-no", "FSTYPE", tDir})
			findmnt, err := mb.setName(name).setCmd(ssh).run()
			Expect(err).ToNot(HaveOccurred())
			Expect(findmnt).To(Exit(0))
			Expect(findmnt.outputToString()).To(ContainSubstring("virtiofs"))
		}
	})

	It("Volume should be disabled by command line", func() {
		skipIfWSL("Requires standard volume handling")
		skipIfVmtype(define.AppleHvVirt, "Skipped on Apple platform")
		skipIfVmtype(define.LibKrun, "Skipped on Apple platform")

		name := randomString()
		i := new(initMachine).withImage(mb.imagePath).withNow()

		// Empty arg forces no volumes
		i.withVolume("")
		session, err := mb.setName(name).setCmd(i).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ssh9p := new(sshMachine).withSSHCommand([]string{"findmnt", "-no", "FSTYPE", "-t", "9p"})
		findmnt9p, err := mb.setName(name).setCmd(ssh9p).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(findmnt9p).To(Exit(0))
		Expect(findmnt9p.outputToString()).To(BeEmpty())

		sshVirtiofs := new(sshMachine).withSSHCommand([]string{"findmnt", "-no", "FSTYPE", "-t", "virtiofs"})
		findmntVirtiofs, err := mb.setName(name).setCmd(sshVirtiofs).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(findmntVirtiofs).To(Exit(0))
		Expect(findmntVirtiofs.outputToString()).To(BeEmpty())
	})

	It("Podman ops with port forwarding and gvproxy", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		ctrName := "test"
		bm := basicMachine{}
		runAlp, err := mb.setCmd(bm.withPodmanCommand([]string{
			"run", "-dt", "--name", ctrName, "-p", "62544:80",
			"--stop-signal", "SIGKILL", TESTIMAGE,
			"/bin/busybox-extras", "httpd", "-f", "-p", "80",
		})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(runAlp).To(Exit(0))
		_, id, _ := strings.Cut(TESTIMAGE, ":")
		testHTTPServer("62544", false, id+"\n")

		// Test exec in machine scenario: https://github.com/containers/podman/issues/20821
		exec, err := mb.setCmd(bm.withPodmanCommand([]string{"exec", ctrName, "true"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(exec).To(Exit(0))

		out, err := pgrep("gvproxy")
		Expect(err).ToNot(HaveOccurred())
		Expect(out).ToNot(BeEmpty())

		rmCon, err := mb.setCmd(bm.withPodmanCommand([]string{"rm", "-af"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(rmCon).To(Exit(0))
		testHTTPServer("62544", true, "")

		stop := new(stopMachine)
		stopSession, err := mb.setCmd(stop).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(stopSession).To(Exit(0))

		// gxproxy should exit after machine is stopped
		out, _ = pgrep("gvproxy")
		Expect(out).ToNot(ContainSubstring("gvproxy"))
	})

	It("podman volume on non-standard path", func() {
		skipIfWSL("Requires standard volume handling")
		dir, err := os.MkdirTemp("", "machine-volume")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(dir)

		testString := "abcdefg1234567"
		testFile := "testfile"
		err = os.WriteFile(filepath.Join(dir, testFile), []byte(testString), 0o644)
		Expect(err).ToNot(HaveOccurred())

		name := randomString()
		machinePath := "/does/not/exist"
		init := new(initMachine).withVolume(fmt.Sprintf("%s:%s", dir, machinePath)).withImage(mb.imagePath).withNow()
		session, err := mb.setName(name).setCmd(init).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		// Must use path.Join to ensure forward slashes are used, even on Windows.
		ssh := new(sshMachine).withSSHCommand([]string{"cat", path.Join(machinePath, testFile)})
		ls, err := mb.setName(name).setCmd(ssh).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(ls).To(Exit(0))
		Expect(ls.outputToString()).To(ContainSubstring(testString))
	})

	It("podman build contexts", func() {
		name := randomString()
		i := new(initMachine)
		session, err := mb.setName(name).setCmd(i.withImage(mb.imagePath).withNow()).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(session).To(Exit(0))

		mainContextDir := GinkgoT().TempDir()
		cfile := filepath.Join(mainContextDir, "test1")
		err = os.WriteFile(cfile, []byte(name), 0o644)
		Expect(err).ToNot(HaveOccurred())

		additionalContextDir := GinkgoT().TempDir()
		cfile = filepath.Join(additionalContextDir, "test2")
		err = os.WriteFile(cfile, []byte(name), 0o644)
		Expect(err).ToNot(HaveOccurred())

		cfile = filepath.Join(mainContextDir, "Containerfile")
		err = os.WriteFile(cfile, []byte("FROM "+TESTIMAGE+"\nCOPY test1 /\nCOPY --from=test-context test2 /\n"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		bm := basicMachine{}
		build, err := mb.setCmd(bm.withPodmanCommand([]string{"build", "-t", name, "--build-context", "test-context=" + additionalContextDir, mainContextDir})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(build).To(Exit(0))
		Expect(build.outputToString()).To(ContainSubstring("COMMIT"))

		run, err := mb.setCmd(bm.withPodmanCommand([]string{"run", name, "cat", "/test1"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(run).To(Exit(0))
		Expect(build.outputToString()).To(ContainSubstring(name))

		run, err = mb.setCmd(bm.withPodmanCommand([]string{"run", name, "cat", "/test2"})).run()
		Expect(err).ToNot(HaveOccurred())
		Expect(run).To(Exit(0))
		Expect(build.outputToString()).To(ContainSubstring(name))
	})

	It("CVE-2025-6032 regression test - HTTP", func() {
		// ensure that trying to pull from a local HTTP server fails and the connection will be rejected
		// ensure that tlsVerify is true by default
		testImagePullTLS(nil, nil)
	})

	It("CVE-2025-6032 regression test - HTTPS unknown cert", func() {
		// ensure that trying to pull from a local HTTPS server with invalid certs fails and the connection will be rejected
		// ensure that tlsVerify is true by default
		testImagePullTLS(&TLSConfig{
			// Key/Cert was generated with:
			// openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:secp384r1 -days 3650 \
			// -nodes -keyout test-tls.key -out test-tls.crt -subj "/CN=test.podman.io" -addext "subjectAltName=IP:127.0.0.1"
			key:  "test-tls.key",
			cert: "test-tls.crt",
		}, nil)
	})

	It("machine init should not fail on TLS validation with --tls-verfy=false - HTTP", func() {
		// ensure that trying to pull from a local HTTP server doesn't fail when --tls-verify=false is set
		tlsVerify := false
		testImagePullTLS(nil, &tlsVerify)
	})

	It("machine init should not fail on TLS validation with --tls-verfy=false - HTTPS", func() {
		// ensure that trying to pull from a local HTTPS server with invalid certs
		// doesn't fail due to tls validation when --tls-verify=false is set
		tlsVerify := false
		testImagePullTLS(&TLSConfig{
			// Key/Cert was generated with:
			// openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:secp384r1 -days 3650 \
			// -nodes -keyout test-tls.key -out test-tls.crt -subj "/CN=test.podman.io" -addext "subjectAltName=IP:127.0.0.1"
			key:  "test-tls.key",
			cert: "test-tls.crt",
		}, &tlsVerify)
	})
})

func testHTTPServer(port string, shouldErr bool, expectedResponse string) {
	address := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort("localhost", port),
	}

	interval := 250 * time.Millisecond
	var err error
	var resp *http.Response
	for range 6 {
		resp, err = http.Get(address.String() + "/testimage-id")
		if err != nil && shouldErr {
			Expect(err.Error()).To(ContainSubstring(expectedResponse))
			return
		}
		if err == nil {
			defer resp.Body.Close()
			break
		}
		time.Sleep(interval)
		interval *= 2
	}
	Expect(err).ToNot(HaveOccurred())

	body, err := io.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())
	Expect(string(body)).Should(Equal(expectedResponse))
}

type TLSConfig struct {
	key  string
	cert string
}

// setup a local webserver in the test and then point podman machine init to it
// to verify the connection details.
func testImagePullTLS(tls *TLSConfig, tlsVerify *bool) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	Expect(err).ToNot(HaveOccurred())
	serverAddr := listener.Addr().String()

	var loggedRequests []string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		loggedRequests = append(loggedRequests, r.URL.Path)
		// don't care about an error, we should never get here
		_, _ = w.Write([]byte("Hello"))
	})

	srv := &http.Server{
		Handler:  mux,
		ErrorLog: log.New(io.Discard, "", 0),
	}
	defer srv.Close()
	serverErr := make(chan error)
	go func() {
		defer GinkgoRecover()
		if tls != nil {
			serverErr <- srv.ServeTLS(listener, tls.cert, tls.key)
		} else {
			serverErr <- srv.Serve(listener)
		}
	}()

	i := new(initMachine)

	manifestName := serverAddr + "/testimage"
	i.withImage("docker://" + manifestName)

	if tlsVerify != nil {
		i.withTlsVerify(tlsVerify)
	}

	name := randomString()
	session, err := mb.setName(name).setCmd(i).run()

	Expect(err).ToNot(HaveOccurred())
	Expect(session).To(Exit(125))

	manifestVersion := rawversion.RawVersion[0:3]

	// Note because we don't run a real registry the error you get when TLS is not checked is:
	// Error: wrong manifest type for disk artifact: text/plain
	// As such we match the errors strings exactly to ensure we have proper error messages that indicate the TLS error.
	expectedErr := "Error: fetching manifest " + manifestVersion + " in " + manifestName + ": pinging container registry " + serverAddr + ": Get \"https://" + serverAddr + "/v2/\": "

	switch {
	case tlsVerify != nil && *tlsVerify == false: // tls-verify explicitly disabled
		expectedErr = "Error: wrong manifest type for disk artifact: text/plain\n"
	case tls != nil:
		expectedErr += "tls: failed to verify certificate: x509: "
		if runtime.GOOS == "darwin" {
			// Apple doesn't like such long valid certs so the error is different but the purpose
			// is the same, it rejected a cert which is how we know TLS verification is turned on.
			// https://support.apple.com/en-au/102028
			expectedErr += "“test.podman.io” certificate is not standards compliant\n"
		} else {
			expectedErr += "certificate signed by unknown authority\n"
		}
	default:
		// With both tlsVerify and tls being nil, a HTTP server will be ran and machine init should
		// default to using tlsVerify
		expectedErr += "http: server gave HTTP response to HTTPS client\n"
	}

	Expect(session.errorToString()).To(Equal(expectedErr))

	// if the client enforces TLS verification then we should not have received any request
	if tlsVerify == nil || *tlsVerify == true {
		Expect(loggedRequests).To(BeEmpty(), "the server should have not process any request from the client")
	}

	srv.Close()
	Expect(<-serverErr).To(Equal(http.ErrServerClosed))
}
