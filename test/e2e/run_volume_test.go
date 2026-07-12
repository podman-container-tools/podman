//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/opencontainers/runtime-spec/specs-go"
	. "go.podman.io/podman/v6/test/utils"
)

// in-container mount point: using a path that is definitely not present
// on the host system might help to uncover some issues.
const dest = "/unique/path"

var _ = Describe("Podman run with volumes", func() {
	// Returns the /proc/self/mountinfo line for a given mount point
	getMountInfo := func(volume string) []string {
		containerDir := strings.SplitN(volume, ":", 3)[1]
		session := podmanTest.Podman([]string{"run", "--rm", "-v", volume, ALPINE, "awk", fmt.Sprintf(`$5 == "%s" { print }`, containerDir), "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(containerDir), "mount point not found in /proc/self/mountinfo")
		return strings.Fields(session.OutputToString())
	}

	//nolint:unparam
	mountVolumeAndCheckDirectory := func(volName, volPath, expectedOwner, imgName string) {
		check := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:%s", volName, volPath), imgName, "stat", "-c", "%U:%G", volPath})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		Expect(check.OutputToString()).Should(ContainSubstring(fmt.Sprintf("%s:%s", expectedOwner, expectedOwner)))
	}

	It("podman run with volume flag", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err = os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		vol := mountPath + ":" + dest

		// [5] is flags
		Expect(getMountInfo(vol)[5]).To(ContainSubstring("rw"))
		Expect(getMountInfo(vol + ":ro")[5]).To(ContainSubstring("ro"))

		mountinfo := getMountInfo(vol + ":shared")
		Expect(mountinfo[5]).To(ContainSubstring("rw"))
		Expect(mountinfo[6]).To(ContainSubstring("shared"))

		// Cached is ignored
		mountinfo = getMountInfo(vol + ":cached")
		Expect(mountinfo[5]).To(ContainSubstring("rw"))
		Expect(mountinfo).To(Not(ContainElement(ContainSubstring("cached"))))

		// Delegated is ignored
		mountinfo = getMountInfo(vol + ":delegated")
		Expect(mountinfo[5]).To(ContainSubstring("rw"))
		Expect(mountinfo).To(Not(ContainElement(ContainSubstring("delegated"))))
	})

	It("podman run with --mount flag", func() {
		if runtime.GOARCH == "ppc64le" {
			Skip("skip failing test on ppc64le")
		}
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err = os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		mount := "type=bind,src=" + mountPath + ",target=" + dest

		session := podmanTest.Podman([]string{"run", "--rm", "--mount", mount, ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(dest + " rw"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",ro", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(dest + " ro"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",readonly", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(dest + " ro"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",consistency=delegated,shared", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("rw"))
		Expect(session.OutputToString()).To(ContainSubstring("shared"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=" + dest, ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(dest + " rw,nosuid,nodev,relatime - tmpfs"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=/etc/ssl,tmpcopyup", ALPINE, "ls", "/etc/ssl"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("certs"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=/etc/ssl,tmpcopyup,notmpcopyup", ALPINE, "ls", "/etc/ssl"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "cannot pass 'tmpcopyup' and 'notmpcopyup' mnt.Options more than once: must provide an argument for option"))

		// test csv escaping
		session = podmanTest.Podman([]string{"run", "--rm", "--mount=type=tmpfs,tmpfs-size=512M,\"destination=/test,\"", ALPINE, "ls", "/test,"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=bind,src=/tmp,target=/tmp,tmpcopyup", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `"tmpcopyup" option not supported for "bind" mount types`))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=bind,src=/tmp,target=/tmp,notmpcopyup", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `"notmpcopyup" option not supported for "bind" mount types`))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=bind,src=/tmp,target=/tmp,bind-propagation=fake", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `invalid value "bind-propagation=fake"`))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", "type=tmpfs,target=/etc/ssl,notmpcopyup", ALPINE, "ls", "/etc/ssl"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("certs")))
	})

	It("podman run with conflicting volumes errors", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"run", "-v", mountPath + ":" + dest, "-v", "/tmp" + ":" + dest, ALPINE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, fmt.Sprintf("%s: duplicate mount destination", dest)))

		session = podmanTest.Podman([]string{"run", "-v", "myvol:" + dest, "-v", mountPath + ":" + dest + ":O", ALPINE, "ls", "/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, fmt.Sprintf("%s: duplicate mount destination", dest)))
	})

	It("podman run with single character volume", func() {
		// 1. create single character volume
		session := podmanTest.Podman([]string{"volume", "create", "a"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		// 2. create container with volume
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data", ALPINE, "sh", "-c", "echo hello world"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run with conflict between image volume and user mount succeeds", func() {
		err = podmanTest.RestoreArtifact(REDIS_IMAGE)
		Expect(err).ToNot(HaveOccurred())
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		testFile := filepath.Join(mountPath, "test1")
		f, err := os.Create(testFile)
		Expect(err).ToNot(HaveOccurred(), "os.Create(testfile)")
		f.Close()
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/data:Z", mountPath), REDIS_IMAGE, "ls", "/data/test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run with mount flag and boolean options", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		mount := "type=bind,src=" + mountPath + ",target=" + dest

		session := podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",ro=false", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(dest + " rw"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",ro=true", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(dest + " ro"))

		session = podmanTest.Podman([]string{"run", "--rm", "--mount", mount + ",ro=true,rw=false", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "cannot pass 'readonly', 'ro', or 'rw' mnt.Options more than once: must provide an argument for option"))
	})

	It("podman run with volume flag and multiple named volumes", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "-v", "testvol1:/testvol1", "-v", "testvol2:/testvol2", ALPINE, "grep", "/testvol", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("/testvol1"))
		Expect(session.OutputToString()).To(ContainSubstring("/testvol2"))
	})

	It("podman run with volumes and suid/dev/exec options", func() {
		if isRootless() {
			// We cannot undo nosuid,nodev,noexec when running rootless for obvious reasons.
			// Thus we should check first if our source dir contains such options and skip the test int his case
			session := SystemExec("findmnt", []string{"-n", "-o", "OPTIONS", "--target", podmanTest.TempDir})
			session.WaitWithDefaultTimeout()
			Expect(session).To(ExitCleanly())
			output := session.OutputToString()
			if strings.Contains(output, "noexec") ||
				strings.Contains(output, "nodev") ||
				strings.Contains(output, "nosuid") {
				Skip("test file system is mounted noexec, nodev or nosuid - cannot bind mount without these options as rootless")
			}
		}

		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "--rm", "-v", mountPath + ":" + dest + ":suid,dev,exec", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(Not(ContainSubstring("noexec")))
		Expect(output).To(Not(ContainSubstring("nodev")))
		Expect(output).To(Not(ContainSubstring("nosuid")))

		session = podmanTest.Podman([]string{"run", "--rm", "--tmpfs", dest + ":suid,dev,exec", ALPINE, "grep", dest, "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = session.OutputToString()
		Expect(output).To(Not(ContainSubstring("noexec")))
		Expect(output).To(Not(ContainSubstring("nodev")))
		Expect(output).To(Not(ContainSubstring("nosuid")))
	})

	// Container should start when workdir is overlay volume
	It("podman run with volume mounted as overlay and used as workdir", func() {
		SkipIfRemote("Overlay volumes only work locally")
		if os.Getenv("container") != "" {
			Skip("Overlay mounts not supported when running in a container")
		}
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// Container should be able to start with custom overlay volume
		session := podmanTest.Podman([]string{"run", "--rm", "-v", mountPath + ":/data:O", "--workdir=/data", ALPINE, "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman support overlay on named volume", func() {
		SkipIfRemote("Overlay volumes only work locally")
		if os.Getenv("container") != "" {
			Skip("Overlay mounts not supported when running in a container")
		}
		session := podmanTest.Podman([]string{"volume", "create", "myvolume"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		// create file on actual volume
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data", ALPINE, "sh", "-c", "echo hello >> " + "/data/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// create file on overlay volume
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data:O", ALPINE, "sh", "-c", "echo hello >> " + "/data/overlay"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// volume should contain only `test` not `overlay`
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data", ALPINE, "sh", "-c", "ls /data"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(Not(ContainSubstring("overlay")))
		Expect(session.OutputToString()).To(ContainSubstring("test"))
	})

	It("podman support overlay on named volume with custom upperdir and workdir", func() {
		SkipIfRemote("Overlay volumes only work locally")
		if os.Getenv("container") != "" {
			Skip("Overlay mounts not supported when running in a container")
		}

		// create persistent upperdir on host
		upperDir := filepath.Join(tempdir, "upper")
		err := os.Mkdir(upperDir, 0o755)
		Expect(err).ToNot(HaveOccurred(), "mkdir "+upperDir)

		// create persistent workdir on host
		workDir := filepath.Join(tempdir, "work")
		err = os.Mkdir(workDir, 0o755)
		Expect(err).ToNot(HaveOccurred(), "mkdir "+workDir)

		overlayOpts := fmt.Sprintf("upperdir=%s,workdir=%s", upperDir, workDir)

		session := podmanTest.Podman([]string{"volume", "create", "myvolume"})
		session.WaitWithDefaultTimeout()
		volName := session.OutputToString()
		Expect(session).Should(ExitCleanly())

		// create file on actual volume
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data", ALPINE, "sh", "-c", "echo hello >> " + "/data/test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// create file on overlay volume
		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data:O," + overlayOpts, ALPINE, "sh", "-c", "echo hello >> " + "/data/overlay"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data:O," + overlayOpts, ALPINE, "sh", "-c", "ls /data"})
		session.WaitWithDefaultTimeout()
		// must contain `overlay` file since it should be persistent on specified upper and workdir
		Expect(session.OutputToString()).To(ContainSubstring("overlay"))
		// this should be there since `test` was written on actual volume not on any overlay
		Expect(session.OutputToString()).To(ContainSubstring("test"))

		session = podmanTest.Podman([]string{"run", "--volume", volName + ":/data:O", ALPINE, "sh", "-c", "ls /data"})
		session.WaitWithDefaultTimeout()
		// must not contain `overlay` file which was on custom upper and workdir since we have not specified any upper or workdir
		Expect(session.OutputToString()).To(Not(ContainSubstring("overlay")))
		// this should be there since `test` was written on actual volume not on any overlay
		Expect(session.OutputToString()).To(ContainSubstring("test"))
	})

	It("podman support overlay volume with custom upperdir and workdir", func() {
		SkipIfRemote("Overlay volumes only work locally")
		if os.Getenv("container") != "" {
			Skip("Overlay mounts not supported when running in a container")
		}

		// Use bindsource instead of named volume
		bindSource := filepath.Join(tempdir, "bindsource")
		err := os.Mkdir(bindSource, 0o755)
		Expect(err).ToNot(HaveOccurred(), "mkdir "+bindSource)

		// create persistent upperdir on host
		upperDir := filepath.Join(tempdir, "upper")
		err = os.Mkdir(upperDir, 0o755)
		Expect(err).ToNot(HaveOccurred(), "mkdir "+upperDir)

		// create persistent workdir on host
		workDir := filepath.Join(tempdir, "work")
		err = os.Mkdir(workDir, 0o755)
		Expect(err).ToNot(HaveOccurred(), "mkdir "+workDir)

		overlayOpts := fmt.Sprintf("upperdir=%s,workdir=%s", upperDir, workDir)

		// create file on overlay volume
		session := podmanTest.Podman([]string{"run", "--volume", bindSource + ":/data:O," + overlayOpts, ALPINE, "sh", "-c", "echo hello >> " + "/data/overlay"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--volume", bindSource + ":/data:O," + overlayOpts, ALPINE, "sh", "-c", "ls /data"})
		session.WaitWithDefaultTimeout()
		// must contain `overlay` file since it should be persistent on specified upper and workdir
		Expect(session.OutputToString()).To(ContainSubstring("overlay"))

		session = podmanTest.Podman([]string{"run", "--volume", bindSource + ":/data:O", ALPINE, "sh", "-c", "ls /data"})
		session.WaitWithDefaultTimeout()
		// must not contain `overlay` file which was on custom upper and workdir since we have not specified any upper or workdir
		Expect(session.OutputToString()).To(Not(ContainSubstring("overlay")))
	})

	It("podman run with noexec can't exec", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "-v", "/bin:/hostbin:noexec", ALPINE, "/hostbin/ls", "/"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(126, "ermission denied"))
	})

	It("podman run with tmpfs named volume mounts and unmounts", func() {
		SkipIfRootless("rootless podman mount requires you to be in a user namespace")
		SkipIfRemote("podman-remote does not support --volumes. This test could be simplified to be tested on Remote.")
		volName := "testvol"
		mkVolume := podmanTest.Podman([]string{"volume", "create", "--opt", "type=tmpfs", "--opt", "device=tmpfs", "--opt", "o=nodev", "testvol"})
		mkVolume.WaitWithDefaultTimeout()
		Expect(mkVolume).Should(ExitCleanly())

		// Volume not mounted on create
		mountCmd1, err := Start(exec.Command("mount"), GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		mountCmd1.Wait(90)
		Expect(mountCmd1).Should(Exit(0))
		mountOut1 := strings.Join(strings.Fields(string(mountCmd1.Out.Contents())), " ")
		GinkgoWriter.Printf("Output: %s", mountOut1)
		Expect(mountOut1).To(Not(ContainSubstring(volName)))

		ctrName := "testctr"
		podmanSession := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, "-v", fmt.Sprintf("%s:/testvol", volName), ALPINE, "top"})
		podmanSession.WaitWithDefaultTimeout()
		Expect(podmanSession).Should(ExitCleanly())

		// Volume now mounted as container is running
		mountCmd2, err := Start(exec.Command("mount"), GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		mountCmd2.Wait(90)
		Expect(mountCmd2).Should(Exit(0))
		mountOut2 := strings.Join(strings.Fields(string(mountCmd2.Out.Contents())), " ")
		GinkgoWriter.Printf("Output: %s", mountOut2)
		Expect(mountOut2).To(ContainSubstring(volName))

		// Stop the container to unmount
		podmanTest.StopContainer(ctrName)

		// We have to force cleanup so the unmount happens
		podmanCleanupSession := podmanTest.Podman([]string{"container", "cleanup", ctrName})
		podmanCleanupSession.WaitWithDefaultTimeout()
		Expect(podmanCleanupSession).Should(ExitCleanly())

		// Ensure volume is unmounted
		mountCmd3, err := Start(exec.Command("mount"), GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		mountCmd3.Wait(90)
		Expect(mountCmd3).Should(Exit(0))
		mountOut3 := strings.Join(strings.Fields(string(mountCmd3.Out.Contents())), " ")
		GinkgoWriter.Printf("Output: %s", mountOut3)
		Expect(mountOut3).To(Not(ContainSubstring(volName)))
	})

	It("podman named volume copyup", func() {
		baselineSession := podmanTest.Podman([]string{"run", "--rm", ALPINE, "ls", "/etc/apk/"})
		baselineSession.WaitWithDefaultTimeout()
		Expect(baselineSession).Should(ExitCleanly())
		baselineOutput := baselineSession.OutputToString()

		inlineVolumeSession := podmanTest.Podman([]string{"run", "--rm", "-v", "testvol1:/etc/apk", ALPINE, "ls", "/etc/apk/"})
		inlineVolumeSession.WaitWithDefaultTimeout()
		Expect(inlineVolumeSession).Should(ExitCleanly())
		Expect(inlineVolumeSession.OutputToString()).To(Equal(baselineOutput))

		makeVolumeSession := podmanTest.Podman([]string{"volume", "create", "testvol2"})
		makeVolumeSession.WaitWithDefaultTimeout()
		Expect(makeVolumeSession).Should(ExitCleanly())

		separateVolumeSession := podmanTest.Podman([]string{"run", "--rm", "-v", "testvol2:/etc/apk", ALPINE, "ls", "/etc/apk/"})
		separateVolumeSession.WaitWithDefaultTimeout()
		Expect(separateVolumeSession).Should(ExitCleanly())
		Expect(separateVolumeSession.OutputToString()).To(Equal(baselineOutput))

		podmanTest.PodmanExitCleanly("run", "--name", "testctr", "-v", "testvol3:/etc/apk:copy", ALPINE, "stat", "-c", "%h", "/etc/apk/arch")

		inspect := podmanTest.PodmanExitCleanly("container", "inspect", "testctr", "--format", "{{.OCIConfigPath}}")

		// Make extra check that the OCI config does not contain the copy opt, runc 1.3.0 fails on that while crun does not.
		// We only test crun upstream so make sure the spec is sane: https://github.com/containers/podman/issues/26938
		f, err := os.Open(inspect.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()
		var spec specs.Spec
		err = json.NewDecoder(f).Decode(&spec)
		Expect(err).ToNot(HaveOccurred())

		found := false
		for _, m := range spec.Mounts {
			if m.Destination == "/etc/apk" {
				found = true
				Expect(m.Options).To(Equal([]string{"rprivate", "nosuid", "nodev", "rbind"}))
			}
		}
		Expect(found).To(BeTrue(), "OCI spec contains /etc/apk mount")

		noCopySession := podmanTest.Podman([]string{"run", "--rm", "-v", "testvol4:/etc/apk:nocopy", ALPINE, "stat", "-c", "%h", "/etc/apk/arch"})
		noCopySession.WaitWithDefaultTimeout()
		Expect(noCopySession).Should(ExitWithError(1, "stat: can't stat '/etc/apk/arch': No such file or directory"))
	})

	It("podman named volume copyup symlink", func() {
		imgName := "testimg"
		dockerfile := fmt.Sprintf(`FROM %s
RUN touch /testfile
RUN sh -c "cd /etc/apk && ln -s ../../testfile"`, ALPINE)
		podmanTest.BuildImage(dockerfile, imgName, "false")

		baselineSession := podmanTest.Podman([]string{"run", "--rm", imgName, "ls", "/etc/apk/"})
		baselineSession.WaitWithDefaultTimeout()
		Expect(baselineSession).Should(ExitCleanly())
		baselineOutput := baselineSession.OutputToString()

		outputSession := podmanTest.Podman([]string{"run", "-v", "/etc/apk/", imgName, "ls", "/etc/apk/"})
		outputSession.WaitWithDefaultTimeout()
		Expect(outputSession).Should(ExitCleanly())
		Expect(outputSession.OutputToString()).To(Equal(baselineOutput))
	})

	It("podman named volume copyup empty directory", func() {
		baselineSession := podmanTest.Podman([]string{"run", "--rm", ALPINE, "ls", "/srv"})
		baselineSession.WaitWithDefaultTimeout()
		Expect(baselineSession).Should(ExitCleanly())
		baselineOutput := baselineSession.OutputToString()

		outputSession := podmanTest.Podman([]string{"run", "-v", "/srv", ALPINE, "ls", "/srv"})
		outputSession.WaitWithDefaultTimeout()
		Expect(outputSession).Should(ExitCleanly())
		Expect(outputSession.OutputToString()).To(Equal(baselineOutput))
	})

	It("podman named volume copyup of /var", func() {
		baselineSession := podmanTest.Podman([]string{"run", "--rm", FEDORA_MINIMAL, "ls", "/var"})
		baselineSession.WaitWithDefaultTimeout()
		Expect(baselineSession).Should(ExitCleanly())
		baselineOutput := baselineSession.OutputToString()

		outputSession := podmanTest.Podman([]string{"run", "-v", "/var", FEDORA_MINIMAL, "ls", "/var"})
		outputSession.WaitWithDefaultTimeout()
		Expect(outputSession).Should(ExitCleanly())
		Expect(outputSession.OutputToString()).To(Equal(baselineOutput))
	})

	It("podman read-only tmpfs conflict with volume", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--read-only", "-v", "tmp_volume:" + dest, ALPINE, "touch", dest + "/a"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session2 := podmanTest.Podman([]string{"run", "--rm", "--read-only", "--tmpfs", dest, ALPINE, "touch", dest + "/a"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())
	})

	It("podman run with anonymous volume", func() {
		list1 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list1.WaitWithDefaultTimeout()
		Expect(list1).Should(ExitCleanly())
		Expect(list1.OutputToString()).To(Equal(""))

		session := podmanTest.Podman([]string{"create", "-v", "/test", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		list2 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list2.WaitWithDefaultTimeout()
		Expect(list2).Should(ExitCleanly())
		arr := list2.OutputToStringArray()
		Expect(arr).To(HaveLen(1))
		Expect(arr[0]).To(Not(Equal("")))
	})

	It("podman rm -v removes anonymous volume", func() {
		list1 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list1.WaitWithDefaultTimeout()
		Expect(list1).Should(ExitCleanly())
		Expect(list1.OutputToString()).To(Equal(""))

		ctrName := "testctr"
		session := podmanTest.Podman([]string{"create", "--name", ctrName, "-v", "/test", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		list2 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list2.WaitWithDefaultTimeout()
		Expect(list2).Should(ExitCleanly())
		arr := list2.OutputToStringArray()
		Expect(arr).To(HaveLen(1))
		Expect(arr[0]).To(Not(Equal("")))

		remove := podmanTest.Podman([]string{"rm", "-v", ctrName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).Should(ExitCleanly())

		list3 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list3.WaitWithDefaultTimeout()
		Expect(list3).Should(ExitCleanly())
		Expect(list3.OutputToString()).To(Equal(""))
	})

	It("podman rm -v retains named volume", func() {
		list1 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list1.WaitWithDefaultTimeout()
		Expect(list1).Should(ExitCleanly())
		Expect(list1.OutputToString()).To(Equal(""))

		ctrName := "testctr"
		volName := "testvol"
		session := podmanTest.Podman([]string{"create", "--name", ctrName, "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		list2 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list2.WaitWithDefaultTimeout()
		Expect(list2).Should(ExitCleanly())
		arr := list2.OutputToStringArray()
		Expect(arr).To(HaveLen(1))
		Expect(arr[0]).To(Equal(volName))

		remove := podmanTest.Podman([]string{"rm", "-v", ctrName})
		remove.WaitWithDefaultTimeout()
		Expect(remove).Should(ExitCleanly())

		list3 := podmanTest.Podman([]string{"volume", "list", "--quiet"})
		list3.WaitWithDefaultTimeout()
		Expect(list3).Should(ExitCleanly())
		arr2 := list3.OutputToStringArray()
		Expect(arr2).To(HaveLen(1))
		Expect(arr2[0]).To(Equal(volName))
	})

	It("podman run image volume is not noexec", func() {
		session := podmanTest.Podman([]string{"run", "--rm", REDIS_IMAGE, "grep", "/data", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Not(ContainSubstring("noexec")))
	})

	It("podman mount with invalid option fails", func() {
		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", "--opt", "type=tmpfs", "--opt", "device=tmpfs", "--opt", "o=invalid", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		volMount := podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/tmp", volName), ALPINE, "ls"})
		volMount.WaitWithDefaultTimeout()
		Expect(volMount).To(ExitWithError(126, "mounting volume testVol for container "))
	})

	It("Podman fix for CVE-2020-1726", func() {
		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		volPath := podmanTest.Podman([]string{"volume", "inspect", "--format", "{{.Mountpoint}}", volName})
		volPath.WaitWithDefaultTimeout()
		Expect(volPath).Should(ExitCleanly())
		path := volPath.OutputToString()

		fileName := "thisIsATestFile"
		file, err := os.Create(filepath.Join(path, fileName))
		Expect(err).ToNot(HaveOccurred())
		defer file.Close()

		runLs := podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%v:/etc/ssl", volName), ALPINE, "ls", "-1", "/etc/ssl"})
		runLs.WaitWithDefaultTimeout()
		Expect(runLs).Should(ExitCleanly())
		outputArr := runLs.OutputToStringArray()
		Expect(outputArr).To(HaveLen(1))
		Expect(outputArr[0]).To(ContainSubstring(fileName))
	})

	It("Podman mount over image volume with trailing /", func() {
		image := "podman-volume-test:trailing"
		dockerfile := fmt.Sprintf(`FROM %s
VOLUME /test/`, ALPINE)
		podmanTest.BuildImage(dockerfile, image, "false")

		ctrName := "testCtr"
		create := podmanTest.Podman([]string{"create", "-v", "/tmp:/test", "--name", ctrName, image, "ls"})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		data := podmanTest.InspectContainer(ctrName)
		Expect(data).To(HaveLen(1))
		Expect(data[0].Mounts).To(HaveLen(1))
		Expect(data[0].Mounts[0]).To(HaveField("Source", "/tmp"))
		Expect(data[0].Mounts[0]).To(HaveField("Destination", "/test"))
	})

	It("podman run with overlay volume flag", func() {
		SkipIfRemote("Overlay volumes only work locally")
		if os.Getenv("container") != "" {
			Skip("Overlay mounts not supported when running in a container")
		}
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		testFile := filepath.Join(mountPath, "test1")
		f, err := os.Create(testFile)
		Expect(err).ToNot(HaveOccurred(), "os.Create "+testFile)
		f.Close()

		// Make sure host directory gets mounted in to container as overlay
		volumeFlag := fmt.Sprintf("%s:/run/test:O", mountPath)
		Expect(getMountInfo(volumeFlag)[7]).To(Equal("overlay"))

		// Make sure host files show up in the container
		session := podmanTest.Podman([]string{"run", "--rm", "-v", volumeFlag, ALPINE, "ls", "/run/test/test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--rm", "-v", ".:/app:O", ALPINE, "ls", "/app"})
		session.WaitWithDefaultTimeout()
		Expect(session.OutputToString()).To(ContainSubstring(filepath.Base(CurrentSpecReport().FileName())))
		Expect(session).Should(ExitCleanly())

		// Make sure modifications in container do not show up on host
		session = podmanTest.Podman([]string{"run", "--rm", "-v", volumeFlag, ALPINE, "touch", "/run/test/container"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		_, err = os.Stat(filepath.Join(mountPath, "container"))
		Expect(err).To(HaveOccurred())

		// Make sure modifications in container disappear when container is stopped
		session = podmanTest.Podman([]string{"create", "-v", fmt.Sprintf("%s:/run/test:O", mountPath), ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"start", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"exec", "-l", "touch", "/run/test/container"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"exec", "-l", "ls", "/run/test/container"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		podmanTest.StopContainer("-l")
		session = podmanTest.Podman([]string{"start", "-l"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"exec", "-l", "ls", "/run/test/container"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(1, "ls: /run/test/container: No such file or directory"))
	})

	It("overlay volume conflicts with named volume and mounts", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err := os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		testFile := filepath.Join(mountPath, "test1")
		f, err := os.Create(testFile)
		Expect(err).ToNot(HaveOccurred())
		f.Close()
		mountSrc := filepath.Join(podmanTest.TempDir, "vol-test1")
		err = os.MkdirAll(mountSrc, 0o755)
		Expect(err).ToNot(HaveOccurred())
		mountDest := "/run/test"
		volName := "myvol"

		session := podmanTest.Podman([]string{"volume", "create", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		expectErr := fmt.Sprintf("%s: duplicate mount destination", mountDest)

		// overlay and named volume destinations conflict
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:%s:O", mountPath, mountDest), "-v", fmt.Sprintf("%s:%s", volName, mountDest), ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, expectErr))
		// overlay and bind mount destinations conflict
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:%s:O", mountPath, mountDest), "--mount", fmt.Sprintf("type=bind,src=%s,target=%s", mountSrc, mountDest), ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, expectErr))
		// overlay and tmpfs mount destinations conflict
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:%s:O", mountPath, mountDest), "--mount", fmt.Sprintf("type=tmpfs,target=%s", mountDest), ALPINE})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, expectErr))
	})

	It("same volume in multiple places does not deadlock", func() {
		volName := "testVol1"
		session := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/test1", volName), "-v", fmt.Sprintf("%s:/test2", volName), "--rm", ALPINE, "sh", "-c", "mount | grep /test"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(HaveLen(2))
	})

	It("podman run with --volume and U flag", func() {
		SkipIfRemote("Overlay volumes only work locally")

		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		name := u.Username
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}
		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err = os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		vol := mountPath + ":" + dest + ":U"

		session := podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "-v", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("888:888"))

		// test with an existing directory in the image
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "881:882", "-v", "NAMED-VOLUME:/mnt:U", ALPINE, "stat", "-c", "%u:%g", "/mnt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("881:882"))

		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--userns", "auto", "-v", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("888:888"))

		vol += ",O"
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--userns", "keep-id", "-v", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("888:888"))
	})

	It("podman run with --mount and U flag", func() {
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		name := u.Username
		if name == "root" {
			name = "containers"
		}

		content, err := os.ReadFile("/etc/subuid")
		if err != nil {
			Skip("cannot read /etc/subuid")
		}

		if !strings.Contains(string(content), name) {
			Skip("cannot find mappings for the current user")
		}

		mountPath := filepath.Join(podmanTest.TempDir, "foo")
		err = os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// false bind mount
		vol := "type=bind,src=" + mountPath + ",dst=" + dest + ",U=false"
		session := podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).ShouldNot(Equal("888:888"))

		// invalid bind mount
		vol = "type=bind,src=" + mountPath + ",dst=" + dest + ",U=invalid"
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `'U' or 'chown' must be set to true or false, instead received "invalid": must provide an argument for option`))

		// true bind mount
		vol = "type=bind,src=" + mountPath + ",dst=" + dest + ",U=true"
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("888:888"))

		// tmpfs mount
		vol = "type=tmpfs," + "dst=" + dest + ",chown"
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("888:888"))

		// anonymous volume mount
		vol = "type=volume," + "dst=" + dest
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("888:888"))

		// named volume mount
		namedVolume := podmanTest.Podman([]string{"volume", "create", "foo"})
		namedVolume.WaitWithDefaultTimeout()
		Expect(namedVolume).Should(ExitCleanly())

		vol = "type=volume,src=foo,dst=" + dest + ",chown=true"
		session = podmanTest.Podman([]string{"run", "--rm", "--user", "888:888", "--mount", vol, ALPINE, "stat", "-c", "%u:%g", dest})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(Equal("888:888"))
	})

	It("podman run with --mount and named volume with driver-opts", func() {
		// anonymous volume mount with driver opts
		vol := "type=volume,source=test_vol,dst=/test,volume-opt=type=tmpfs,volume-opt=device=tmpfs,volume-opt=o=nodev"
		// Loop twice to cover both the initial code path that creates the volume and the ones which reuses it.
		for i := range 2 {
			name := "testctr" + strconv.Itoa(i)
			podmanTest.PodmanExitCleanly("run", "--name", name, "--mount", vol, ALPINE, "echo", "hello")

			inspectVol := podmanTest.PodmanExitCleanly("volume", "inspect", "test_vol")
			Expect(inspectVol.OutputToString()).To(ContainSubstring("nodev"))

			inspect := podmanTest.PodmanExitCleanly("container", "inspect", name, "--format", "{{range .Mounts}}{{.Options}}{{end}}")
			Expect(inspect.OutputToString()).To(ContainSubstring("[nosuid nodev rbind]"))
		}
	})

	It("volume permissions after run", func() {
		imgName := "testimg"
		dockerfile := fmt.Sprintf(`FROM %s
RUN adduser -D -u 1005 testuser
USER testuser`, CITEST_IMAGE)
		podmanTest.BuildImage(dockerfile, imgName, "false")

		testString := "testuser testuser"

		test1 := podmanTest.Podman([]string{"run", "-v", "testvol1:/test", imgName, "sh", "-c", "ls -al /test | grep -v root | grep -v total"})
		test1.WaitWithDefaultTimeout()
		Expect(test1).Should(ExitCleanly())
		Expect(test1.OutputToString()).To(ContainSubstring(testString))

		volName := "testvol2"
		vol := podmanTest.Podman([]string{"volume", "create", volName})
		vol.WaitWithDefaultTimeout()
		Expect(vol).Should(ExitCleanly())

		test2 := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/test", volName), imgName, "sh", "-c", "ls -al /test | grep -v root | grep -v total"})
		test2.WaitWithDefaultTimeout()
		Expect(test2).Should(ExitCleanly())
		Expect(test2.OutputToString()).To(ContainSubstring(testString))
	})

	It("podman run with named volume check if we honor permission of target dir", func() {
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "stat", "-c", "%a %Y", "/var/tmp"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		perms := session.OutputToString()

		session = podmanTest.Podman([]string{"run", "--rm", "-v", "test:/var/tmp", ALPINE, "stat", "-c", "%a %Y", "/var/tmp"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(perms))
	})

	It("podman run with -v $SRC:/run does not create /run/.containerenv", func() {
		mountSrc := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(mountSrc, 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"run", "-v", mountSrc + ":/run", ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// the file should not have been created
		_, err = os.Stat(filepath.Join(mountSrc, ".containerenv"))
		Expect(err).To(HaveOccurred())
	})

	It("podman volume with uid and gid works", func() {
		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", "--opt", "o=uid=1000", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		volMount := podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "stat", "-c", "%u", "/test"})
		volMount.WaitWithDefaultTimeout()
		Expect(volMount).Should(ExitCleanly())
		Expect(volMount.OutputToString()).To(Equal("1000"))

		volName = "testVol2"
		volCreate = podmanTest.Podman([]string{"volume", "create", "--opt", "o=gid=1000", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		volMount = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "stat", "-c", "%g", "/test"})
		volMount.WaitWithDefaultTimeout()
		Expect(volMount).Should(ExitCleanly())
		Expect(volMount.OutputToString()).To(Equal("1000"))

		volName = "testVol3"
		volCreate = podmanTest.Podman([]string{"volume", "create", "--opt", "o=uid=1000,gid=1000", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		volMount = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/test", volName), ALPINE, "stat", "-c", "%u:%g", "/test"})
		volMount.WaitWithDefaultTimeout()
		Expect(volMount).Should(ExitCleanly())
		Expect(volMount.OutputToString()).To(Equal("1000:1000"))
	})

	It("podman run -v with a relative dir", func() {
		mountPath := filepath.Join(podmanTest.TempDir, "vol")
		err = os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.RemoveAll(mountPath)
			Expect(err).ToNot(HaveOccurred())
		}()

		f, err := os.CreateTemp(mountPath, "podman")
		Expect(err).ToNot(HaveOccurred())

		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())

		err = os.Chdir(mountPath)
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.Chdir(cwd)
			Expect(err).ToNot(HaveOccurred())
		}()

		run := podmanTest.Podman([]string{"run", "--security-opt", "label=disable", "-v", "./:" + dest, ALPINE, "ls", dest})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		Expect(run.OutputToString()).Should(ContainSubstring(strings.TrimLeft("/vol/", f.Name())))
	})

	It("podman run --mount type=image with subpath", func() {
		podmanTest.AddImageToRWStore(ALPINE)

		pathToCheck := "/sbin"
		pathInCtr := "/mnt"

		ctrCommand := []string{"run", "--mount", fmt.Sprintf("type=image,source=%s,dst=%s,subpath=%s", ALPINE, pathInCtr, pathToCheck), ALPINE, "ls"}

		run1Cmd := append(ctrCommand, pathToCheck)
		run1 := podmanTest.Podman(run1Cmd)
		run1.WaitWithDefaultTimeout()
		Expect(run1).Should(ExitCleanly())

		run2Cmd := append(ctrCommand, pathInCtr)
		run2 := podmanTest.Podman(run2Cmd)
		run2.WaitWithDefaultTimeout()
		Expect(run2).Should(ExitCleanly())

		Expect(run1.OutputToString()).Should(Equal(run2.OutputToString()))
	})

	It("podman run -v chowns multiple times on empty volume", func() {
		imgName := "testimg"
		dockerfile := fmt.Sprintf(`FROM %s
RUN addgroup -g 1234 test1
RUN addgroup -g 4567 test2
RUN addgroup -g 7890 test3
RUN adduser -D -u 1234 -G test1 test1
RUN adduser -D -u 4567 -G test2 test2
RUN adduser -D -u 7890 -G test3 test3
RUN mkdir /test1 /test2 /test3 /test4
RUN chown test1:test1 /test1
RUN chown test2:test2 /test2
RUN chown test3:test3 /test4
RUN chmod 755 /test1 /test2 /test3 /test4`, ALPINE)
		podmanTest.BuildImage(dockerfile, imgName, "false")

		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		mountVolumeAndCheckDirectory(volName, "/test1", "test1", imgName)
		mountVolumeAndCheckDirectory(volName, "/test2", "test2", imgName)
		mountVolumeAndCheckDirectory(volName, "/test3", "root", imgName)
		mountVolumeAndCheckDirectory(volName, "/test4", "root", imgName)
	})

	It("podman run -v chowns until copy-up on volume", func() {
		imgName := "testimg"
		dockerfile := fmt.Sprintf(`FROM %s
RUN addgroup -g 1234 test1
RUN addgroup -g 4567 test2
RUN addgroup -g 7890 test3
RUN adduser -D -u 1234 -G test1 test1
RUN adduser -D -u 4567 -G test2 test2
RUN adduser -D -u 7890 -G test3 test3
RUN mkdir /test1 /test2 /test3
RUN touch /test2/file1
RUN chown test1:test1 /test1
RUN chown -R test2:test2 /test2
RUN chown test3:test3 /test3
RUN chmod 755 /test1 /test2 /test3`, ALPINE)
		podmanTest.BuildImage(dockerfile, imgName, "false")

		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		mountVolumeAndCheckDirectory(volName, "/test1", "test1", imgName)
		mountVolumeAndCheckDirectory(volName, "/test2", "test2", imgName)
		mountVolumeAndCheckDirectory(volName, "/test3", "test2", imgName)
	})

	It("podman run -v chowns until volume has contents", func() {
		imgName := "testimg"
		dockerfile := fmt.Sprintf(`FROM %s
RUN addgroup -g 1234 test1
RUN addgroup -g 4567 test2
RUN addgroup -g 7890 test3
RUN adduser -D -u 1234 -G test1 test1
RUN adduser -D -u 4567 -G test2 test2
RUN adduser -D -u 7890 -G test3 test3
RUN mkdir /test1 /test2 /test3
RUN chown test1:test1 /test1
RUN chown test2:test2 /test2
RUN chown test3:test3 /test3
RUN chmod 755 /test1 /test2 /test3`, ALPINE)
		podmanTest.BuildImage(dockerfile, imgName, "false")

		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		mountVolumeAndCheckDirectory(volName, "/test1", "test1", imgName)
		mountVolumeAndCheckDirectory(volName, "/test2", "test2", imgName)

		session := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/test2", volName), imgName, "touch", "/test2/file1"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitCleanly())

		mountVolumeAndCheckDirectory(volName, "/test3", "test2", imgName)
	})

	It("podman run --mount type=volume,subpath=", func() {
		volName := "testvol"
		mkvol := podmanTest.Podman([]string{"volume", "create", volName})
		mkvol.WaitWithDefaultTimeout()
		Expect(mkvol).Should(ExitCleanly())

		subvol := "/test/test2/"
		pathInCtr := "/mnt"
		pathToCreate := filepath.Join(pathInCtr, subvol)
		popvol := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/mnt", volName), ALPINE, "sh", "-c", fmt.Sprintf("mkdir -p %s; touch %s; touch %s", pathToCreate, filepath.Join(pathToCreate, "foo"), filepath.Join(pathToCreate, "bar"))})
		popvol.WaitWithDefaultTimeout()
		Expect(popvol).Should(ExitCleanly())

		checkCtr := podmanTest.Podman([]string{"run", "--mount", fmt.Sprintf("type=volume,source=%s,target=%s,subpath=%s", volName, pathInCtr, subvol), ALPINE, "ls", pathInCtr})
		checkCtr.WaitWithDefaultTimeout()
		Expect(checkCtr).To(ExitCleanly())
		Expect(checkCtr.OutputToString()).To(ContainSubstring("foo"))
		Expect(checkCtr.OutputToString()).To(ContainSubstring("bar"))
	})

	It("user-specified overlay supersedes image volume", func() {
		err := podmanTest.RestoreArtifact(REDIS_IMAGE)
		Expect(err).ToNot(HaveOccurred())
		mountPath := filepath.Join(podmanTest.TempDir, "secrets")
		err = os.Mkdir(mountPath, 0o755)
		Expect(err).ToNot(HaveOccurred())
		testFile := filepath.Join(mountPath, "test1")
		f, err := os.Create(testFile)
		Expect(err).ToNot(HaveOccurred(), "os.Create(testfile)")
		f.Close()
		Expect(err).ToNot(HaveOccurred())
		session := podmanTest.Podman([]string{"run", "-v", fmt.Sprintf("%s:/data:O", mountPath), REDIS_IMAGE, "ls", "/data/test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("--mount flag defaults to volume if no type given", func() {
		volName := "testvol"
		podmanTest.PodmanExitCleanly("volume", "create", volName)

		podmanTest.PodmanExitCleanly("run", "--rm", "--mount", fmt.Sprintf("src=%s,dest=/mnt", volName), ALPINE, "touch", "/mnt/testfile")
		outTest := podmanTest.PodmanExitCleanly("run", "--rm", "--mount", fmt.Sprintf("type=volume,src=%s,dest=/mnt", volName), ALPINE, "ls", "/mnt")
		Expect(outTest.OutputToString()).To(ContainSubstring("testfile"))
	})

	It("podman run --tmpfs with noatime option", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--tmpfs", "/mytmpfs:noatime", ALPINE, "grep", "mytmpfs", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(ContainSubstring("noatime"))

		session = podmanTest.Podman([]string{"run", "--rm", "--tmpfs", "/mytmpfs", ALPINE, "grep", "mytmpfs", "/proc/self/mountinfo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output = session.OutputToString()
		Expect(output).ToNot(ContainSubstring("noatime"))
	})

	It("podman run -v with nocreate option fails when volume doesn't exist", func() {
		volName := "testvol-nocreate-nonexistent"
		// Ensure volume doesn't exist
		session := podmanTest.Podman([]string{"volume", "rm", "-f", volName})
		session.WaitWithDefaultTimeout()

		// Run with nocreate option should error
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/mnt:nocreate", volName), ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, fmt.Sprintf("volume %s does not exist", volName)))
	})

	It("podman run -v with nocreate option succeeds when volume exists", func() {
		volName := "testvol-nocreate-exists"
		// Create volume first
		session := podmanTest.Podman([]string{"volume", "create", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Run with nocreate option should succeed since volume exists
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/mnt:nocreate", volName), ALPINE, "touch", "/mnt/testfile"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Cleanup
		session = podmanTest.Podman([]string{"volume", "rm", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run --mount type=volume with nocreate option fails when volume doesn't exist", func() {
		volName := "testvol-mount-nocreate-nonexistent"
		// Ensure volume doesn't exist
		session := podmanTest.Podman([]string{"volume", "rm", "-f", volName})
		session.WaitWithDefaultTimeout()

		// Run with nocreate option should error
		session = podmanTest.Podman([]string{"run", "--rm", "--mount", fmt.Sprintf("type=volume,src=%s,dst=/mnt,nocreate", volName), ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, fmt.Sprintf("volume %s does not exist", volName)))
	})

	It("podman run --mount type=volume with nocreate option succeeds when volume exists", func() {
		volName := "testvol-mount-nocreate-exists"
		// Create volume first
		session := podmanTest.Podman([]string{"volume", "create", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Run with nocreate option should succeed since volume exists
		session = podmanTest.Podman([]string{"run", "--rm", "--mount", fmt.Sprintf("type=volume,src=%s,dst=/mnt,nocreate", volName), ALPINE, "touch", "/mnt/testfile"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Cleanup
		session = podmanTest.Podman([]string{"volume", "rm", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman run -v with nocreate combined with other options", func() {
		volName := "testvol-nocreate-combo"
		// Create volume first
		session := podmanTest.Podman([]string{"volume", "create", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Run with nocreate and ro options should succeed
		session = podmanTest.Podman([]string{"run", "--rm", "-v", fmt.Sprintf("%s:/mnt:ro,nocreate", volName), ALPINE, "ls", "/mnt"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// Cleanup
		session = podmanTest.Podman([]string{"volume", "rm", volName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("podman create -v with nocreate option fails when volume doesn't exist", func() {
		volName := "testvol-create-nocreate"
		// Ensure volume doesn't exist
		session := podmanTest.Podman([]string{"volume", "rm", "-f", volName})
		session.WaitWithDefaultTimeout()

		// Create with nocreate option should error
		session = podmanTest.Podman([]string{"create", "-v", fmt.Sprintf("%s:/mnt:nocreate", volName), ALPINE, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, fmt.Sprintf("volume %s does not exist", volName)))
	})
})
