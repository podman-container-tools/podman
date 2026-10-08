//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/libpod/define"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Verify podman containers.conf usage", func() {
	BeforeEach(func() {
		confPath, err := filepath.Abs("config/containers.conf")
		Expect(err).ToNot(HaveOccurred())
		os.Setenv("CONTAINERS_CONF", confPath)
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
	})

	It("limits test", func() {
		// containers.conf is set to "nofile=500:500"
		session := podmanTest.Podman([]string{"run", "--rm", FEDORA_MINIMAL, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("500"))

		session = podmanTest.Podman([]string{"run", "--rm", "--ulimit", "nofile=2048:2048", FEDORA_MINIMAL, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("2048"))

		// Reset CONTAINERS_CONF to "/dev/null"
		// Local should go back to defaults but remote should be set on server side
		os.Setenv("CONTAINERS_CONF", "/dev/null")
		session = podmanTest.Podman([]string{"run", "--rm", FEDORA_MINIMAL, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		if IsRemote() {
			Expect(session.OutputToString()).To(ContainSubstring("500"))
		} else {
			Expect(session.OutputToString()).To(Not(Equal("500")))
		}
	})

	It("oom-score-adj", func() {
		// containers.conf is set to "oom_score_adj=999"
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "cat", "/proc/self/oom_score_adj"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("999"))

		raw, err := os.ReadFile("/proc/self/oom_score_adj")
		Expect(err).ToNot(HaveOccurred())

		rawS := strings.TrimSuffix(string(raw), "\n")

		// Reset CONTAINERS_CONF to "/dev/null"
		// Local should go back to defaults but remote should be set on server side
		os.Setenv("CONTAINERS_CONF", "/dev/null")
		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "cat", "/proc/self/oom_score_adj"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		if IsRemote() {
			Expect(session.OutputToString()).To(Equal("999"))
		} else {
			if isRootless() {
				Expect(session.OutputToString()).To(ContainSubstring(rawS))
			} else {
				Expect(session.OutputToString()).To(ContainSubstring("0"))
			}
		}
	})

	It("cgroup_conf in containers.conf", func() {
		// FIXME: Needs crun-1.8.2-2 to allow this with --cgroup-manager=cgroupfs, once this is available remove the skip below.
		SkipIfRootless("--cgroup-manager=cgoupfs and --cgroup-conf not supported in rootless mode with crun")
		conffile := filepath.Join(podmanTest.TempDir, "container.conf")

		err := os.WriteFile(conffile, []byte("[containers]\ncgroup_conf = [\"pids.max=1234\",]\n"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("CONTAINERS_CONF_OVERRIDE", conffile)
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		// containers.conf is set to "pids.max=1234"
		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "cat", "/sys/fs/cgroup/pids.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("1234"))

		session = podmanTest.Podman([]string{"run", "--rm", "--cgroup-conf", "pids.max=400", ALPINE, "cat", "/sys/fs/cgroup/pids.max"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("400"))
	})

	It("having additional env", func() {
		// containers.conf default env includes foo
		session := podmanTest.Podman([]string{"run", ALPINE, "printenv"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("foo=bar"))
	})

	It("having env_merge from containers.conf", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
	ENV PODMAN_TEST_VAR=original
	`
		podmanTest.BuildImage(dockerfile, "test-env-merge", "false")

		session := podmanTest.Podman([]string{"run", "--rm", "test-env-merge", "printenv", "PODMAN_TEST_VAR"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("original-merged"))
	})

	It("additional devices", func() {
		// containers.conf devices includes notone
		session := podmanTest.Podman([]string{"run", "--device", "/dev/null:/dev/bar", ALPINE, "ls", "/dev"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(
			And(
				ContainSubstring("bar"),
				ContainSubstring("notone"),
			))
	})

	It("shm-size", func() {
		// containers.conf default sets shm-size=201k, which ends up as 200k
		session := podmanTest.Podman([]string{"run", ALPINE, "grep", "shm", "/proc/self/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("size=200k"))

		session = podmanTest.Podman([]string{"run", "--shm-size", "1g", ALPINE, "grep", "shm", "/proc/self/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("size=1048576k"))
	})

	It("add capabilities", func() {
		cap := podmanTest.Podman([]string{"run", ALPINE, "grep", "CapEff", "/proc/self/status"})
		cap.WaitWithDefaultTimeout()
		Expect(cap).Should(ExitCleanly())

		os.Setenv("CONTAINERS_CONF", "config/containers-ns.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		session := podmanTest.Podman([]string{"run", BB, "grep", "CapEff", "/proc/self/status"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).ToNot(Equal(cap.OutputToString()))
	})

	It("regular capabilities", func() {
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		result := podmanTest.Podman([]string{"top", "test1", "capeff"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(
			And(
				ContainSubstring("FOWNER"),
				ContainSubstring("SETFCAP"),
			))
	})

	It("drop capabilities", func() {
		os.Setenv("CONTAINERS_CONF", "config/containers-caps.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		setup := podmanTest.RunTopContainer("test1")
		setup.WaitWithDefaultTimeout()
		result := podmanTest.Podman([]string{"container", "top", "test1", "capeff"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).ToNot(
			And(
				ContainSubstring("SETUID"),
				ContainSubstring("FOWNER"),
			))
	})

	verifyNSHandling := func(nspath, option string) {
		os.Setenv("CONTAINERS_CONF", "config/containers-ns.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		// containers.conf default ipcns to default to host
		session := podmanTest.Podman([]string{"run", ALPINE, "ls", "-l", nspath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		fields := strings.Split(session.OutputToString(), " ")
		ctrNS := strings.TrimSuffix(fields[len(fields)-1], "\n")

		cmd := exec.Command("ls", "-l", nspath)
		res, err := cmd.Output()
		Expect(err).ToNot(HaveOccurred())
		fields = strings.Split(string(res), " ")
		hostNS := strings.TrimSuffix(fields[len(fields)-1], "\n")
		Expect(hostNS).To(Equal(ctrNS))

		session = podmanTest.Podman([]string{"run", option, "private", ALPINE, "ls", "-l", nspath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		fields = strings.Split(session.OutputToString(), " ")
		ctrNS = fields[len(fields)-1]
		Expect(hostNS).ToNot(Equal(ctrNS))
	}

	It("netns", func() {
		verifyNSHandling("/proc/self/ns/net", "--network")
	})

	It("ipcns", func() {
		verifyNSHandling("/proc/self/ns/ipc", "--ipc")
	})

	It("utsns", func() {
		verifyNSHandling("/proc/self/ns/uts", "--uts")
	})

	It("pidns", func() {
		verifyNSHandling("/proc/self/ns/pid", "--pid")
	})

	It("cgroupns", func() {
		verifyNSHandling("/proc/self/ns/cgroup", "--cgroupns")
	})

	It("using journald for container with container log_tag", func() {
		SkipIfJournaldUnavailable()
		os.Setenv("CONTAINERS_CONF", "config/containers-journald.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		logc := podmanTest.Podman([]string{"run", "-d", ALPINE, "sh", "-c", "echo podman; sleep 0.1; echo podman; sleep 0.1; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).Should(ExitCleanly())
		cid := logc.OutputToString()

		wait := podmanTest.Podman([]string{"wait", cid})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(ExitCleanly())

		// Flake prevention: journalctl makes no timeliness guarantees.
		time.Sleep(1 * time.Second)
		cmd := exec.Command("journalctl", "--no-pager", "-o", "json", "--output-fields=CONTAINER_TAG", fmt.Sprintf("CONTAINER_ID_FULL=%s", cid))
		out, err := cmd.CombinedOutput()
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(ContainSubstring("alpine"))
	})

	It("using journald for container with container log_tag override", func() {
		SkipIfJournaldUnavailable()
		os.Setenv("CONTAINERS_CONF", "config/containers-journald.conf")
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		logc := podmanTest.Podman([]string{"run", "-d", "--log-opt", "tag=OverriddenTag", ALPINE, "sh", "-c", "echo podman; sleep 0.1; echo podman; sleep 0.1; echo podman"})
		logc.WaitWithDefaultTimeout()
		Expect(logc).Should(ExitCleanly())
		cid := logc.OutputToString()

		wait := podmanTest.Podman([]string{"wait", cid})
		wait.WaitWithDefaultTimeout()
		Expect(wait).Should(ExitCleanly())

		// Flake prevention: journalctl makes no timeliness guarantees.
		time.Sleep(1 * time.Second)
		cmd := exec.Command("journalctl", "--no-pager", "-o", "json", "--output-fields=CONTAINER_TAG", fmt.Sprintf("CONTAINER_ID_FULL=%s", cid))
		out, err := cmd.CombinedOutput()
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(ContainSubstring("OverriddenTag"))
	})

	It("add volumes", func() {
		conffile := filepath.Join(podmanTest.TempDir, "container.conf")

		volume := filepath.Join(podmanTest.TempDir, "vol")
		err = os.MkdirAll(volume, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		err := os.WriteFile(conffile, fmt.Appendf(nil, "[containers]\nvolumes=[\"%s:%s:Z\",]\n", volume, volume), 0o755)
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("CONTAINERS_CONF", conffile)
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		result := podmanTest.Podman([]string{"run", ALPINE, "ls", volume})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("sysctl test", func() {
		// containers.conf is set to   "net.ipv4.ping_group_range=0 1000"
		session := podmanTest.Podman([]string{"run", "--rm", FEDORA_MINIMAL, "cat", "/proc/sys/net/ipv4/ping_group_range"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("1000"))

		// Ignore containers.conf setting if --net=host
		session = podmanTest.Podman([]string{"run", "--rm", "--net", "host", FEDORA_MINIMAL, "cat", "/proc/sys/net/ipv4/ping_group_range"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).ToNot(ContainSubstring("1000"))
	})

	It("search domain", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search foobar.com")))
	})

	It("add dns server", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("nameserver 1.2.3.4")))
	})

	It("add dns option", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("options debug")))
	})

	It("remove all search domain", func() {
		session := podmanTest.Podman([]string{"run", "--dns-search=.", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(Not(ContainElement(HavePrefix("search"))))
	})

	It("add search domain", func() {
		session := podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search")))
		Expect(session.OutputToString()).To(
			And(
				ContainSubstring("foobar.com"),
				ContainSubstring("1.2.3.4"),
				ContainSubstring("debug"),
			))
	})

	It("add timezone", func() {
		// containers.conf timezone set to Pacific/Honolulu
		session := podmanTest.Podman([]string{"run", "--tz", "", ALPINE, "date", "+'%H %Z'"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("HST"))

		// verify flag still overrides
		// Arizona does not observe DST so this command is safe at all times of the year
		session = podmanTest.Podman([]string{"run", "--tz", "America/Phoenix", ALPINE, "date", "+'%H %Z'"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("MST"))
	})

	It("add umask", func() {
		// containers.conf umask set to 0002
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			Skip("Test only works on crun")
		}

		session := podmanTest.Podman([]string{"run", "--rm", ALPINE, "sh", "-c", "umask"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("0002"))
	})

	It("podman-remote test localcontainers.conf", func() {
		SkipIfNotRemote("this test is only for remote")

		os.Setenv("CONTAINERS_CONF", "config/containers-remote.conf")
		// Configuration that comes from remote server
		// env
		session := podmanTest.Podman([]string{"run", ALPINE, "printenv", "foo"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("bar"))

		// dns-search, server, options
		session = podmanTest.Podman([]string{"run", ALPINE, "cat", "/etc/resolv.conf"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToStringArray()).To(ContainElement(HavePrefix("search")))
		Expect(session.OutputToString()).To(
			And(
				ContainSubstring("foobar.com"),
				ContainSubstring("1.2.3.4"),
				ContainSubstring("debug"),
			))

		// sysctls
		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "cat", "/proc/sys/net/ipv4/ping_group_range"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("1000"))

		// shm-size
		session = podmanTest.Podman([]string{"run", ALPINE, "grep", "shm", "/proc/self/mounts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("size=200k"))

		// ulimits
		session = podmanTest.Podman([]string{"run", "--rm", FEDORA_MINIMAL, "ulimit", "-n"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("500"))

		// Configuration that comes from remote client
		// Timezone
		session = podmanTest.Podman([]string{"run", ALPINE, "date", "+'%H %Z'"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(
			Or(
				ContainSubstring("EST"),
				ContainSubstring("EDT"),
			))

		// Umask
		session = podmanTest.Podman([]string{"run", "--rm", ALPINE, "sh", "-c", "umask"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("0022"))
	})

	It("add annotations", func() {
		// containers.conf is set to "run.oci.keep_original_groups=1"
		session := podmanTest.Podman([]string{"create", "--rm", "--name", "test", FEDORA_MINIMAL})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{ .Config.Annotations }}", "test"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(ContainSubstring(fmt.Sprintf("%s:1", define.RunOCIKeepOriginalGroups)))
	})

	It("--add-host and no-hosts=true fails", func() {
		session := podmanTest.Podman([]string{"run", "-dt", "--add-host", "test1:127.0.0.1", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, "--no-hosts and --add-host cannot be set together"))

		session = podmanTest.Podman([]string{"run", "-dt", "--add-host", "test1:127.0.0.1", "--no-hosts=false", ALPINE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
	})

	It("no-hosts=true /etc/hosts does not include hostname", func() {
		session := podmanTest.Podman([]string{"run", "--rm", "--name", "test", ALPINE, "cat", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).ToNot(ContainSubstring("test"))

		session = podmanTest.Podman([]string{"run", "--rm", "--name", "test", "--no-hosts=false", ALPINE, "cat", "/etc/hosts"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("test"))
	})

	Describe("base_hosts_file in containers.conf", func() {
		var baseHostsFile string
		var session *PodmanSessionIntegration

		JustBeforeEach(func() {
			conffile := filepath.Join(podmanTest.TempDir, "containers.conf")
			err = os.WriteFile(conffile, fmt.Appendf(nil, "[containers]\nbase_hosts_file=\"%s\"\nno_hosts=false\n", baseHostsFile), 0o755)
			Expect(err).ToNot(HaveOccurred())
			os.Setenv("CONTAINERS_CONF_OVERRIDE", conffile)
			if IsRemote() {
				podmanTest.RestartRemoteService()
			}

			dockerfile := strings.Join([]string{
				`FROM quay.io/libpod/alpine:latest`,
				`RUN echo '56.78.12.34 image.example.com' > /etc/hosts`,
			}, "\n")
			podmanTest.BuildImage(dockerfile, "foobar.com/hosts_test:latest", "false", "--no-hosts")

			session = podmanTest.Podman([]string{"run", "--name", "hosts_test", "--hostname", "hosts_test.dev", "--rm", "foobar.com/hosts_test:latest", "cat", "/etc/hosts"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
		})

		Describe("base_hosts_file=path", func() {
			BeforeEach(func() {
				hostsPath := filepath.Join(podmanTest.TempDir, "hosts")
				err := os.WriteFile(hostsPath, []byte("12.34.56.78 file.example.com"), 0o755)
				Expect(err).ToNot(HaveOccurred())
				baseHostsFile = hostsPath
			})

			It("should use the hosts file from the file path", func() {
				Expect(session.OutputToString()).ToNot(ContainSubstring("56.78.12.34 image.example.com"))
				Expect(session.OutputToString()).To(ContainSubstring("12.34.56.78 file.example.com"))
				Expect(session.OutputToString()).To(ContainSubstring("127.0.0.1 localhost"))
				Expect(session.OutputToString()).To(ContainSubstring("::1 localhost"))
				Expect(session.OutputToString()).To(ContainSubstring("host.containers.internal host.docker.internal"))
				Expect(session.OutputToString()).To(ContainSubstring("hosts_test.dev hosts_test"))
			})
		})

		Describe("base_hosts_file=image", func() {
			BeforeEach(func() {
				baseHostsFile = "image"
			})

			It("should use the hosts file from the container image", func() {
				Expect(session.OutputToString()).To(ContainSubstring("56.78.12.34 image.example.com"))
				Expect(session.OutputToString()).ToNot(ContainSubstring("12.34.56.78 file.example.com"))
				Expect(session.OutputToString()).To(ContainSubstring("127.0.0.1 localhost"))
				Expect(session.OutputToString()).To(ContainSubstring("::1 localhost"))
				Expect(session.OutputToString()).To(ContainSubstring("host.containers.internal host.docker.internal"))
				Expect(session.OutputToString()).To(ContainSubstring("hosts_test.dev hosts_test"))
			})
		})

		Describe("base_hosts_file=none", func() {
			BeforeEach(func() {
				baseHostsFile = "none"
			})

			It("should not use any hosts files", func() {
				Expect(session.OutputToString()).ToNot(ContainSubstring("56.78.12.34 image.example.com"))
				Expect(session.OutputToString()).ToNot(ContainSubstring("12.34.56.78 file.example.com"))
				Expect(session.OutputToString()).To(ContainSubstring("127.0.0.1 localhost"))
				Expect(session.OutputToString()).To(ContainSubstring("::1 localhost"))
				Expect(session.OutputToString()).To(ContainSubstring("host.containers.internal host.docker.internal"))
				Expect(session.OutputToString()).To(ContainSubstring("hosts_test.dev hosts_test"))
			})
		})
	})

	It("seccomp profile path", func() {
		configPath := filepath.Join(podmanTest.TempDir, "containers.conf")
		os.Setenv("CONTAINERS_CONF", configPath)

		profile := filepath.Join(podmanTest.TempDir, "seccomp.json")
		containersConf := fmt.Appendf(nil, "[containers]\nseccomp_profile=\"%s\"", profile)
		err = os.WriteFile(configPath, containersConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session := podmanTest.Podman([]string{"info", "--format", "{{.Host.Security.SECCOMPProfilePath}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal(profile))
	})

	It("add image_copy_tmp_dir", func() {
		// Prevents overwriting of TMPDIR environment
		if cacheDir, found := os.LookupEnv("TMPDIR"); found {
			defer os.Setenv("TMPDIR", cacheDir)
			os.Unsetenv("TMPDIR")
		} else {
			defer os.Unsetenv("TMPDIR")
		}
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session := podmanTest.Podman([]string{"info", "--format", "{{.Store.ImageCopyTmpDir}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("/var/tmp"))

		storagePath := filepath.Join(podmanTest.TempDir, "storage")
		configPath := filepath.Join(podmanTest.TempDir, "containers.conf")
		os.Setenv("CONTAINERS_CONF", configPath)

		containersConf := []byte("[engine]\nimage_copy_tmp_dir=\"/foobar\"")
		err = os.WriteFile(configPath, containersConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session = podmanTest.Podman([]string{"info", "--format", "{{.Store.ImageCopyTmpDir}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("/foobar"))

		containersConf = fmt.Appendf(nil, "[engine]\nimage_copy_tmp_dir=%q", storagePath)
		err = os.WriteFile(configPath, containersConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		session = podmanTest.Podman([]string{"info", "--format", "{{.Store.ImageCopyTmpDir}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(storagePath))

		containersConf = []byte("[engine]\nimage_copy_tmp_dir=\"storage1\"")
		err = os.WriteFile(configPath, containersConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		if !IsRemote() {
			session = podmanTest.Podman([]string{"info", "--format", "{{.Store.ImageCopyTmpDir}}"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitWithError(125, `invalid image_copy_tmp_dir value "storage1" (relative paths are not accepted)`))

			os.Setenv("TMPDIR", "/hoge")
			session = podmanTest.Podman([]string{"info", "--format", "{{.Store.ImageCopyTmpDir}}"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(Equal("/hoge"))
			os.Unsetenv("TMPDIR")
		}
	})

	// FIXME not sure why this is here
	It("system service --help shows (default 20)", func() {
		SkipIfRemote("system service is not supported on clients")

		result := podmanTest.Podman([]string{"system", "service", "--help"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring("(default 1234)"))
	})

	It("bad infra_image name", func() {
		infra1 := "i.do/not/exist:image"
		infra2 := "i.still.do/not/exist:image"
		errorString := "initializing source docker://" + infra1
		error2String := "initializing source docker://" + infra2
		configPath := filepath.Join(podmanTest.TempDir, "containers.conf")
		os.Setenv("CONTAINERS_CONF", configPath)

		containersConf := []byte("[engine]\ninfra_image=\"" + infra1 + "\"")
		err = os.WriteFile(configPath, containersConf, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		result := podmanTest.Podman([]string{"pod", "create", "--infra-image", infra2})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, error2String))

		result = podmanTest.Podman([]string{"pod", "create"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, errorString))

		result = podmanTest.Podman([]string{"create", "--pod", "new:pod1", ALPINE})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitWithError(125, errorString))
	})

	It("set .engine.remote=true", func() {
		SkipIfRemote("only meaningful when running ABI/local")

		// Need to restore CONTAINERS_CONF or AfterEach() will fail
		if path, found := os.LookupEnv("CONTAINERS_CONF"); found {
			defer os.Setenv("CONTAINERS_CONF", path)
		}

		configPath := filepath.Join(podmanTest.TempDir, "containers-engine-remote.conf")
		os.Setenv("CONTAINERS_CONF", configPath)
		defer os.Remove(configPath)

		err := os.WriteFile(configPath, []byte("[engine]\nremote=true"), os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		// podmanTest.Podman() cannot be used as it was initialized remote==false
		cmd := exec.Command(podmanTest.PodmanBinary, "info", "--format", "{{.Host.ServiceIsRemote}}")
		session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())

		description := "Should have failed as there is no running remote API service available."
		Eventually(session, DefaultWaitTimeout).Should(Exit(125), description)
		Expect(session.Err).Should(Say("Error: unable to connect to Podman socket"))
	})

	It("podman containers.conf cgroups=disabled", func() {
		if !strings.Contains(podmanTest.OCIRuntime, "crun") {
			// Assume this will never be fixed in runc
			Skip("NoCgroups requires crun")
		}

		conffile := filepath.Join(podmanTest.TempDir, "container.conf")
		err := os.WriteFile(conffile, []byte("[containers]\ncgroups=\"disabled\"\n"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		result := podmanTest.Podman([]string{"create", ALPINE, "true"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{ .HostConfig.Cgroups }}", result.OutputToString()})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).ToNot(Equal("disabled"))

		os.Setenv("CONTAINERS_CONF", conffile)
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}
		result = podmanTest.Podman([]string{"create", ALPINE, "true"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())

		inspect = podmanTest.Podman([]string{"inspect", "--format", "{{ .HostConfig.Cgroups }}", result.OutputToString()})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect.OutputToString()).To(Equal("disabled"))

		// Check we can also create a pod when cgroups=disabled
		result = podmanTest.Podman([]string{"pod", "create"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
	})

	It("podman containers.conf runtime", func() {
		SkipIfRemote("--runtime option is not available for remote commands")
		conffile := filepath.Join(podmanTest.TempDir, "container.conf")
		err := os.WriteFile(conffile, []byte("[engine]\nruntime=\"testruntime\"\n"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		os.Setenv("CONTAINERS_CONF", conffile)
		result := podmanTest.Podman([]string{"--help"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		Expect(result.OutputToString()).To(ContainSubstring("Path to the OCI-compatible binary used to run containers. (default \"testruntime\")"))
	})

	It("podman default_rootless_network_cmd", func() {
		SkipIfNotRootless("default_rootless_network_cmd is only used rootless")

		for _, mode := range []string{"pasta", "invalid"} {
			conffile := filepath.Join(podmanTest.TempDir, "container.conf")
			content := "[network]\ndefault_rootless_network_cmd=\"" + mode + "\"\n"
			err := os.WriteFile(conffile, []byte(content), 0o755)
			Expect(err).ToNot(HaveOccurred())

			os.Setenv("CONTAINERS_CONF_OVERRIDE", conffile)
			if IsRemote() {
				podmanTest.RestartRemoteService()
			}

			podman := podmanTest.Podman([]string{"create", "--name", mode, ALPINE, "ip", "addr"})
			podman.WaitWithDefaultTimeout()

			if mode == "invalid" {
				Expect(podman).Should(ExitWithError(125, `invalid default_rootless_network_cmd option "invalid"`))
				continue
			}
			Expect(podman).Should(ExitCleanly())

			inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.HostConfig.NetworkMode}}", mode})
			inspect.WaitWithDefaultTimeout()
			Expect(inspect).Should(ExitCleanly())
			Expect(inspect.OutputToString()).Should(Equal(mode))
		}
	})

	startContainer := func(params ...string) string {
		args := []string{"run", "-d"}
		if slices.Contains(params, "--name") {
			args = append(args, "--replace")
		}
		args = append(args, params...)
		args = append(args, ALPINE, "true")

		result := podmanTest.Podman(args)
		result.WaitWithDefaultTimeout()
		Expect(result).Should(ExitCleanly())
		containerID := result.OutputToString()

		return containerID
	}

	getContainerConfig := func(containerID string, formatParam string) string {
		inspect := podmanTest.Podman([]string{"inspect", "--format", formatParam, containerID})
		inspect.WaitWithDefaultTimeout()
		value := inspect.OutputToString()
		return value
	}

	It("podman containers.conf container_name_as_hostname", func() {
		// With default containers.conf

		// Start container with no options
		containerID := startContainer()
		hostname := getContainerConfig(containerID, "{{ .Config.Hostname }}")
		// Hostname should be the first 12 characters of the containerID
		Expect(hostname).To(Equal(containerID[:12]))

		// Start container with name
		containerID = startContainer("--name", "cname1")
		hostname = getContainerConfig(containerID, "{{ .Config.Hostname }}")
		// Hostname should still be the first 12 characters of the containerID
		Expect(hostname).To(Equal(containerID[:12]))

		// Start container with just hostname
		containerID = startContainer("--hostname", "cname1.dev")
		hostname = getContainerConfig(containerID, "{{ .Config.Hostname }}")
		// Hostname should now be "cname1.dev"
		Expect(hostname).To(Equal("cname1.dev"))

		// Start container with name and hostname
		containerID = startContainer("--name", "cname1", "--hostname", "cname1.dev")
		hostname = getContainerConfig(containerID, "{{ .Config.Hostname }}")
		// Hostname should now be "cname1.dev"
		Expect(hostname).To(Equal("cname1.dev"))

		// Create containers.conf override with container_name_as_hostname=true
		conffile := filepath.Join(podmanTest.TempDir, "container.conf")
		err := os.WriteFile(conffile, []byte("[containers]\ncontainer_name_as_hostname=true\n"), 0o755)
		Expect(err).ToNot(HaveOccurred())
		os.Setenv("CONTAINERS_CONF_OVERRIDE", conffile)
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		// Start container with no options
		containerID = startContainer()
		hostname = getContainerConfig(containerID, "{{ .Config.Hostname }}")
		name := getContainerConfig(containerID, "{{ .Name }}")
		// Hostname should be the auto generated container name with '_' removed
		Expect(hostname).To(Equal(strings.ReplaceAll(name, "_", "")))

		// Start container with name
		containerID = startContainer("--name", "cname1")
		hostname = getContainerConfig(containerID, "{{ .Config.Hostname }}")
		// Hostname should be the container name
		Expect(hostname).To(Equal("cname1"))

		// Start container with name containing '_'
		containerID = startContainer("--name", "cname1_2_3")
		hostname = getContainerConfig(containerID, "{{ .Config.Hostname }}")
		// Hostname should be the set container name with all '_' removed
		Expect(hostname).To(Equal("cname123"))

		// Start container with just hostname
		containerID = startContainer("--hostname", "cname1.dev")
		hostname = getContainerConfig(containerID, "{{ .Config.Hostname }}")
		// Hostname should now be "cname1.dev"
		Expect(hostname).To(Equal("cname1.dev"))

		// Start container with name and hostname
		containerID = startContainer("--name", "cname1", "--hostname", "cname1.dev")
		hostname = getContainerConfig(containerID, "{{ .Config.Hostname }}")
		// Hostname should still be "cname1.dev"
		Expect(hostname).To(Equal("cname1.dev"))

		// Start container with name = 260 characters
		longHostname := "cnabcdefghijklmnopqrstuvwxyz1234567890.abcdefghijklmnopqrstuvwxyz1234567890.abcdefghijklmnopqrstuvwxyz1234567890.abcdefghijklmnopqrstuvwxyz1234567890.abcdefghijklmnopqrstuvwxyz1234567890.abcdefghijklmnopqrstuvwxyz1234567890.abcdefghijklmnopqrstuvwxyz1234567890"
		containerID = startContainer("--name", longHostname)
		hostname = getContainerConfig(containerID, "{{ .Config.Hostname }}")
		name = getContainerConfig(containerID, "{{ .Name }}")
		// Double check that name actually got set correctly
		Expect(name).To(Equal(longHostname))
		// Hostname should be the container name truncated to 64 characters
		Expect(hostname).To(Equal(name[:64]))
	})
})
