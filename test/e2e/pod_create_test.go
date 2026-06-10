//go:build linux || freebsd

package integration

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/selinux/go-selinux"
	"go.podman.io/common/pkg/apparmor"
	"go.podman.io/common/pkg/seccomp"
	"go.podman.io/common/pkg/sysinfo"
	"go.podman.io/podman/v6/pkg/util"
	. "go.podman.io/podman/v6/test/utils"
	"go.podman.io/storage/pkg/stringid"
)

var _ = Describe("Podman pod create", func() {
	hostname, _ := os.Hostname()

	It("podman create pod", func() {
		_, ec, podID := podmanTest.CreatePod(nil)
		Expect(ec).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "-q", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring(podID))
		Expect(check.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman create pod with name", func() {
		name := "test"
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}})
		Expect(ec).To(Equal(0))

		check := podmanTest.Podman([]string{"pod", "ps", "--no-trunc"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToString()).To(ContainSubstring(name))
	})

	It("podman create pod with doubled name", func() {
		name := "test"
		_, ec, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}})
		Expect(ec).To(Equal(0))

		_, ec2, _ := podmanTest.CreatePod(map[string][]string{"--name": {name}})
		Expect(ec2).To(Not(Equal(0)))

		check := podmanTest.Podman([]string{"pod", "ps", "-q"})
		check.WaitWithDefaultTimeout()
		Expect(check.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman create pod without network portbindings", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		pod := session.OutputToString()

		webserver := podmanTest.Podman([]string{"run", "--pod", pod, "-dt", NGINX_IMAGE})
		webserver.WaitWithDefaultTimeout()
		Expect(webserver).Should(ExitCleanly())

		_, err := net.Dial("tcp", "localhost:80")
		Expect(err).To(HaveOccurred())
	})

	It("podman create pod with network portbindings", func() {
		name := "test"
		port := GetPort()
		session := podmanTest.Podman([]string{"pod", "create", "--name", name, "-p", fmt.Sprintf("%d:80", port)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		pod := session.OutputToString()

		webserver := podmanTest.Podman([]string{"run", "--pod", pod, "-dt", NGINX_IMAGE})
		webserver.WaitWithDefaultTimeout()
		Expect(webserver).Should(ExitCleanly())
		testPortConnection(port)
	})

	It("podman create pod with id file with network portbindings", func() {
		file := filepath.Join(podmanTest.TempDir, "pod.id")
		name := "test"
		port := GetPort()
		session := podmanTest.Podman([]string{"pod", "create", "--name", name, "--pod-id-file", file, "-p", fmt.Sprintf("%d:80", port)})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		webserver := podmanTest.Podman([]string{"run", "--pod-id-file", file, "-dt", NGINX_IMAGE})
		webserver.WaitWithDefaultTimeout()
		Expect(webserver).Should(ExitCleanly())
		testPortConnection(port)
	})

	It("podman create pod with no infra but portbindings should fail", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--infra=false", "--name", name, "-p", "80:80"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "you must have an infra container to publish port bindings to the host"))
	})

	It("podman create pod with --no-hosts", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--no-hosts", "--name", name})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		alpineResolvConf := podmanTest.Podman([]string{"run", "--rm", "--no-hosts", ALPINE, "cat", "/etc/hosts"})
		alpineResolvConf.WaitWithDefaultTimeout()
		Expect(alpineResolvConf).Should(ExitCleanly())

		podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "--rm", ALPINE, "cat", "/etc/hosts"})
		podResolvConf.WaitWithDefaultTimeout()
		Expect(podResolvConf).Should(ExitCleanly())
		Expect(podResolvConf.OutputToString()).To(Equal(alpineResolvConf.OutputToString()))
	})

	It("podman create pod with --no-hostname", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--no-hostname", "--name", name})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		alpineHostname := podmanTest.Podman([]string{"run", "--rm", "--no-hostname", ALPINE, "cat", "/etc/hostname"})
		alpineHostname.WaitWithDefaultTimeout()
		Expect(alpineHostname).Should(ExitCleanly())

		podHostname := podmanTest.Podman([]string{"run", "--pod", name, "--rm", ALPINE, "cat", "/etc/hostname"})
		podHostname.WaitWithDefaultTimeout()
		Expect(podHostname).Should(ExitCleanly())
		Expect(podHostname.OutputToString()).To(Equal(alpineHostname.OutputToString()))
	})

	It("podman create pod with --no-hosts and no infra should fail", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--no-hosts", "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "cannot specify --no-hosts without an infra container"))
	})

	It("podman create pod with --add-host", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--add-host", "test.example.com:12.34.56.78", "--name", name})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "--rm", ALPINE, "cat", "/etc/hosts"})
		podResolvConf.WaitWithDefaultTimeout()
		Expect(podResolvConf).Should(ExitCleanly())
		Expect(podResolvConf.OutputToString()).To(ContainSubstring("12.34.56.78 test.example.com"))
	})

	It("podman create pod with --add-host and no infra should fail", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--add-host", "test.example.com:12.34.56.78", "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "NoInfra and HostAdd are mutually exclusive pod options: invalid pod spec"))
	})

	It("podman create pod with --add-host and --no-hosts should fail", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--add-host", "test.example.com:12.34.56.78", "--name", name, "--no-hosts"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "--no-hosts and --add-host cannot be set together"))
	})

	Describe("podman create pod with --hosts-file", func() {
		BeforeEach(func() {
			buildDir := filepath.Join(podmanTest.TempDir, "build"+stringid.GenerateRandomID())
			err := os.Mkdir(buildDir, 0o755)
			Expect(err).ToNot(HaveOccurred())

			imageHosts := filepath.Join(buildDir, "pause_hosts")
			err = os.WriteFile(imageHosts, []byte("56.78.12.34 image.example.com"), 0o755)
			Expect(err).ToNot(HaveOccurred())

			configHosts := filepath.Join(podmanTest.TempDir, "hosts")
			err = os.WriteFile(configHosts, []byte("12.34.56.78 config.example.com"), 0o755)
			Expect(err).ToNot(HaveOccurred())

			confFile := filepath.Join(podmanTest.TempDir, "containers.conf")
			err = os.WriteFile(confFile, fmt.Appendf(nil, "[containers]\nbase_hosts_file=\"%s\"\n", configHosts), 0o755)
			Expect(err).ToNot(HaveOccurred())
			os.Setenv("CONTAINERS_CONF_OVERRIDE", confFile)
			if IsRemote() {
				podmanTest.RestartRemoteService()
			}

			containerfile := strings.Join([]string{
				`FROM ` + INFRA_IMAGE,
				`COPY pause_hosts /etc/hosts`,
			}, "\n")

			containerFilePath := filepath.Join(buildDir, "Containerfile")
			err = os.WriteFile(containerFilePath, []byte(containerfile), 0o644)
			Expect(err).ToNot(HaveOccurred())

			podmanTest.PodmanExitCleanly("build", "-q", "-t", "foobar.com/hosts_test_pause:latest", "--layers=false", "--no-hosts", buildDir)
		})

		It("--hosts-file=path", func() {
			hostsPath := filepath.Join(podmanTest.TempDir, "hosts")
			err := os.WriteFile(hostsPath, []byte("23.45.67.89 file.example.com"), 0o755)
			Expect(err).ToNot(HaveOccurred())

			podCreate := podmanTest.Podman([]string{"pod", "create", "--hostname", "hosts_test.dev", "--hosts-file=" + hostsPath, "--add-host=add.example.com:34.56.78.90", "--infra-image=foobar.com/hosts_test_pause:latest", "--infra-name=hosts_test_infra", "--name", "hosts_test_pod"})
			podCreate.WaitWithDefaultTimeout()
			Expect(podCreate).Should(ExitCleanly())

			session := podmanTest.Podman([]string{"run", "--pod", "hosts_test_pod", "--name", "hosts_test", "--rm", ALPINE, "cat", "/etc/hosts"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).ToNot(ContainSubstring("56.78.12.34 image.example.com"))
			Expect(session.OutputToString()).ToNot(ContainSubstring("12.34.56.78 config.example.com"))
			Expect(session.OutputToString()).To(ContainSubstring("23.45.67.89 file.example.com"))
			Expect(session.OutputToString()).To(ContainSubstring("34.56.78.90 add.example.com"))
			Expect(session.OutputToString()).To(ContainSubstring("127.0.0.1 localhost"))
			Expect(session.OutputToString()).To(ContainSubstring("::1 localhost"))
			Expect(session.OutputToString()).To(ContainSubstring("host.containers.internal host.docker.internal"))
			Expect(session.OutputToString()).To(ContainSubstring("hosts_test.dev hosts_test_infra"))
			Expect(session.OutputToString()).To(ContainSubstring("127.0.0.1 hosts_test"))
		})

		It("--hosts-file=image", func() {
			podCreate := podmanTest.Podman([]string{"pod", "create", "--hostname", "hosts_test.dev", "--hosts-file=image", "--add-host=add.example.com:34.56.78.90", "--infra-image=foobar.com/hosts_test_pause:latest", "--infra-name=hosts_test_infra", "--name", "hosts_test_pod"})
			podCreate.WaitWithDefaultTimeout()
			Expect(podCreate).Should(ExitCleanly())

			session := podmanTest.Podman([]string{"run", "--pod", "hosts_test_pod", "--name", "hosts_test", "--rm", ALPINE, "cat", "/etc/hosts"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).To(ContainSubstring("56.78.12.34 image.example.com"))
			Expect(session.OutputToString()).ToNot(ContainSubstring("12.34.56.78 config.example.com"))
			Expect(session.OutputToString()).To(ContainSubstring("34.56.78.90 add.example.com"))
			Expect(session.OutputToString()).To(ContainSubstring("127.0.0.1 localhost"))
			Expect(session.OutputToString()).To(ContainSubstring("::1 localhost"))
			Expect(session.OutputToString()).To(ContainSubstring("host.containers.internal host.docker.internal"))
			Expect(session.OutputToString()).To(ContainSubstring("hosts_test.dev hosts_test_infra"))
			Expect(session.OutputToString()).To(ContainSubstring("127.0.0.1 hosts_test"))
		})

		It("--hosts-file=none", func() {
			podCreate := podmanTest.Podman([]string{"pod", "create", "--hostname", "hosts_test.dev", "--hosts-file=none", "--add-host=add.example.com:34.56.78.90", "--infra-image=foobar.com/hosts_test_pause:latest", "--infra-name=hosts_test_infra", "--name", "hosts_test_pod"})
			podCreate.WaitWithDefaultTimeout()
			Expect(podCreate).Should(ExitCleanly())

			session := podmanTest.Podman([]string{"run", "--pod", "hosts_test_pod", "--name", "hosts_test", "--rm", ALPINE, "cat", "/etc/hosts"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).ToNot(ContainSubstring("56.78.12.34 image.example.com"))
			Expect(session.OutputToString()).ToNot(ContainSubstring("12.34.56.78 config.example.com"))
			Expect(session.OutputToString()).To(ContainSubstring("34.56.78.90 add.example.com"))
			Expect(session.OutputToString()).To(ContainSubstring("127.0.0.1 localhost"))
			Expect(session.OutputToString()).To(ContainSubstring("::1 localhost"))
			Expect(session.OutputToString()).To(ContainSubstring("host.containers.internal host.docker.internal"))
			Expect(session.OutputToString()).To(ContainSubstring("hosts_test.dev hosts_test_infra"))
			Expect(session.OutputToString()).To(ContainSubstring("127.0.0.1 hosts_test"))
		})

		It("--hosts-file= falls back to containers.conf", func() {
			podCreate := podmanTest.Podman([]string{"pod", "create", "--hostname", "hosts_test.dev", "--hosts-file=", "--add-host=add.example.com:34.56.78.90", "--infra-image=foobar.com/hosts_test_pause:latest", "--infra-name=hosts_test_infra", "--name", "hosts_test_pod"})
			podCreate.WaitWithDefaultTimeout()
			Expect(podCreate).Should(ExitCleanly())

			session := podmanTest.Podman([]string{"run", "--pod", "hosts_test_pod", "--name", "hosts_test", "--rm", ALPINE, "cat", "/etc/hosts"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			Expect(session.OutputToString()).ToNot(ContainSubstring("56.78.12.34 image.example.com"))
			Expect(session.OutputToString()).To(ContainSubstring("12.34.56.78 config.example.com"))
			Expect(session.OutputToString()).To(ContainSubstring("34.56.78.90 add.example.com"))
			Expect(session.OutputToString()).To(ContainSubstring("127.0.0.1 localhost"))
			Expect(session.OutputToString()).To(ContainSubstring("::1 localhost"))
			Expect(session.OutputToString()).To(ContainSubstring("host.containers.internal host.docker.internal"))
			Expect(session.OutputToString()).To(ContainSubstring("hosts_test.dev hosts_test_infra"))
			Expect(session.OutputToString()).To(ContainSubstring("127.0.0.1 hosts_test"))
		})
	})

	It("podman create pod with --hosts-file and no infra should fail", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--hosts-file=image", "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "NoInfra and HostsFile are mutually exclusive pod options: invalid pod spec"))
	})

	It("podman create pod with --hosts-file and --no-hosts should fail", func() {
		name := "test"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--hosts-file=image", "--name", name, "--no-hosts"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "--no-hosts and --hosts-file cannot be set together"))
	})

	It("podman create pod with DNS server set", func() {
		name := "test"
		server := "12.34.56.78"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns", server, "--name", name})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "--rm", ALPINE, "cat", "/etc/resolv.conf"})
		podResolvConf.WaitWithDefaultTimeout()
		Expect(podResolvConf).Should(ExitCleanly())
		Expect(podResolvConf.OutputToString()).To(ContainSubstring("nameserver %s", server))
	})

	It("podman create pod with DNS server set and no infra should fail", func() {
		name := "test"
		server := "12.34.56.78"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns", server, "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "NoInfra and DNSServer are mutually exclusive pod options: invalid pod spec"))
	})

	It("podman create pod with DNS option set", func() {
		name := "test"
		option := "attempts:5"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns-opt", option, "--name", name})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "--rm", ALPINE, "cat", "/etc/resolv.conf"})
		podResolvConf.WaitWithDefaultTimeout()
		Expect(podResolvConf).Should(ExitCleanly())
		Expect(podResolvConf.OutputToString()).To(ContainSubstring(fmt.Sprintf("options %s", option)))
	})

	It("podman create pod with DNS option set and no infra should fail", func() {
		name := "test"
		option := "attempts:5"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns-opt", option, "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "NoInfra and DNSOption are mutually exclusive pod options: invalid pod spec"))
	})

	It("podman create pod with DNS search domain set", func() {
		name := "test"
		search := "example.com"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns-search", search, "--name", name})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "--rm", ALPINE, "cat", "/etc/resolv.conf"})
		podResolvConf.WaitWithDefaultTimeout()
		Expect(podResolvConf).Should(ExitCleanly())
		Expect(podResolvConf.OutputToString()).To(ContainSubstring(fmt.Sprintf("search %s", search)))
	})

	It("podman create pod with DNS search domain set and no infra should fail", func() {
		name := "test"
		search := "example.com"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--dns-search", search, "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "NoInfo and DNSSearch are mutually exclusive pod options: invalid pod spec"))
	})

	It("podman create pod with IP address", func() {
		name := "test"
		ip := GetSafeIPAddress()
		podCreate := podmanTest.Podman([]string{"pod", "create", "--ip", ip, "--name", name})
		podCreate.WaitWithDefaultTimeout()
		// Rootless should error without network
		if isRootless() {
			Expect(podCreate).Should(ExitWithError(125, "invalid config provided: networks and static ip/mac address can only be used with Bridge mode networking"))
		} else {
			Expect(podCreate).Should(ExitCleanly())
			podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "--rm", ALPINE, "ip", "addr"})
			podResolvConf.WaitWithDefaultTimeout()
			Expect(podResolvConf).Should(ExitCleanly())
			Expect(podResolvConf.OutputToString()).To(ContainSubstring(ip))
		}
	})

	It("podman container in pod with IP address shares IP address", func() {
		SkipIfRootless("Rootless does not support --ip without network")
		podName := "test"
		ctrName := "testCtr"
		ip := GetSafeIPAddress()
		podCreate := podmanTest.Podman([]string{"pod", "create", "--ip", ip, "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		podCtr := podmanTest.Podman([]string{"run", "--name", ctrName, "--pod", podName, "-d", "-t", ALPINE, "top"})
		podCtr.WaitWithDefaultTimeout()
		Expect(podCtr).Should(ExitCleanly())
		ctrInspect := podmanTest.Podman([]string{"inspect", ctrName})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(ExitCleanly())
		ctrJSON := ctrInspect.InspectContainerToJSON()
		Expect(ctrJSON[0].NetworkSettings).To(HaveField("IPAddress", ip))
	})

	It("podman create pod with IP address and no infra should fail", func() {
		name := "test"
		ip := GetSafeIPAddress()
		podCreate := podmanTest.Podman([]string{"pod", "create", "--ip", ip, "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "cannot set --ip without infra container: invalid argument"))
	})

	It("podman create pod with MAC address", func() {
		name := "test"
		mac := "92:d0:c6:0a:29:35"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--mac-address", mac, "--name", name})
		podCreate.WaitWithDefaultTimeout()
		// Rootless should error
		if isRootless() {
			Expect(podCreate).Should(ExitWithError(125, "invalid config provided: networks and static ip/mac address can only be used with Bridge mode networking"))
		} else {
			Expect(podCreate).Should(ExitCleanly())
			podResolvConf := podmanTest.Podman([]string{"run", "--pod", name, "--rm", ALPINE, "ip", "addr"})
			podResolvConf.WaitWithDefaultTimeout()
			Expect(podResolvConf).Should(ExitCleanly())
			Expect(podResolvConf.OutputToString()).To(ContainSubstring(mac))
		}
	})

	It("podman create pod with MAC address and no infra should fail", func() {
		name := "test"
		mac := "92:d0:c6:0a:29:35"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--mac-address", mac, "--name", name, "--infra=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "cannot set --mac without infra container: invalid argument"))
	})

	It("podman create pod and print id to external file", func() {
		// Switch to temp dir and restore it afterwards
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred())
		Expect(os.Chdir(os.TempDir())).To(Succeed())

		targetFile := filepath.Join(podmanTest.TempDir, "idFile")
		defer Expect(os.RemoveAll(targetFile)).To(BeNil())
		defer Expect(os.Chdir(cwd)).To(BeNil())

		session := podmanTest.Podman([]string{"pod", "create", "--name=abc", "--pod-id-file", targetFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		id, _ := os.ReadFile(targetFile)
		check := podmanTest.Podman([]string{"pod", "inspect", "abc"})
		check.WaitWithDefaultTimeout()
		data := check.InspectPodToJSON()
		Expect(data).To(HaveField("ID", string(id)))
	})

	It("podman pod create --replace", func() {
		// Make sure we error out with --name.
		session := podmanTest.Podman([]string{"pod", "create", "--replace"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "cannot replace pod without --name being set"))

		// Create and replace 5 times in a row the "same" pod.
		podName := "testCtr"
		for range 5 {
			session = podmanTest.Podman([]string{"pod", "create", "--replace", "--name", podName})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
		}
	})

	It("podman create pod with defaults", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"pod", "inspect", name})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		data := check.InspectPodToJSON()

		check1 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Config.Entrypoint}}", data.Containers[0].ID})
		check1.WaitWithDefaultTimeout()
		Expect(check1).Should(ExitCleanly())
		Expect(check1.OutputToString()).To(Equal("[/catatonit -P]"))

		// check the Path and Args
		check2 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Path}}:{{.Args}}", data.Containers[0].ID})
		check2.WaitWithDefaultTimeout()
		Expect(check2).Should(ExitCleanly())
		Expect(check2.OutputToString()).To(Equal("/catatonit:[-P]"))
	})

	It("podman create pod with --infra-command", func() {
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--infra-command", "/pause1", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"pod", "inspect", name})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		data := check.InspectPodToJSON()

		check1 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Config.Entrypoint}}", data.Containers[0].ID})
		check1.WaitWithDefaultTimeout()
		Expect(check1).Should(ExitCleanly())
		Expect(check1.OutputToString()).To(Equal("[/pause1]"))

		// check the Path and Args
		check2 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Path}}:{{.Args}}", data.Containers[0].ID})
		check2.WaitWithDefaultTimeout()
		Expect(check2).Should(ExitCleanly())
		Expect(check2.OutputToString()).To(Equal("/pause1:[/pause1]"))
	})

	It("podman create pod with --infra-image", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
entrypoint ["/fromimage"]
`
		podmanTest.BuildImage(dockerfile, "localhost/infra", "false")
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--infra-image", "localhost/infra", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"pod", "inspect", name})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		data := check.InspectPodToJSON()

		check1 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Config.Entrypoint}}", data.Containers[0].ID})
		check1.WaitWithDefaultTimeout()
		Expect(check1).Should(ExitCleanly())
		Expect(check1.OutputToString()).To(Equal("[/fromimage]"))

		// check the Path and Args
		check2 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Path}}:{{.Args}}", data.Containers[0].ID})
		check2.WaitWithDefaultTimeout()
		Expect(check2).Should(ExitCleanly())
		Expect(check2.OutputToString()).To(Equal("/fromimage:[/fromimage]"))
	})

	It("podman create pod with --infra-command --infra-image", func() {
		dockerfile := `FROM quay.io/libpod/alpine:latest
entrypoint ["/fromimage"]
`
		podmanTest.BuildImage(dockerfile, "localhost/infra", "false")
		name := "test"
		session := podmanTest.Podman([]string{"pod", "create", "--infra-image", "localhost/infra", "--infra-command", "/fromcommand", "--name", name})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		check := podmanTest.Podman([]string{"pod", "inspect", name})
		check.WaitWithDefaultTimeout()
		Expect(check).Should(ExitCleanly())
		data := check.InspectPodToJSON()

		check1 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Config.Entrypoint}}", data.Containers[0].ID})
		check1.WaitWithDefaultTimeout()
		Expect(check1).Should(ExitCleanly())
		Expect(check1.OutputToString()).To(Equal("[/fromcommand]"))

		// check the Path and Args
		check2 := podmanTest.Podman([]string{"container", "inspect", "--format", "{{.Path}}:{{.Args}}", data.Containers[0].ID})
		check2.WaitWithDefaultTimeout()
		Expect(check2).Should(ExitCleanly())
		Expect(check2.OutputToString()).To(Equal("/fromcommand:[/fromcommand]"))
	})

	It("podman pod status test", func() {
		podName := "testpod"
		create := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())

		status1 := podmanTest.Podman([]string{"pod", "inspect", "--format", "{{ .State }}", podName})
		status1.WaitWithDefaultTimeout()
		Expect(status1).Should(ExitCleanly())
		Expect(status1.OutputToString()).To(ContainSubstring("Created"))

		ctr1 := podmanTest.Podman([]string{"run", "--pod", podName, "-d", ALPINE, "top"})
		ctr1.WaitWithDefaultTimeout()
		Expect(ctr1).Should(ExitCleanly())

		status2 := podmanTest.Podman([]string{"pod", "inspect", "--format", "{{ .State }}", podName})
		status2.WaitWithDefaultTimeout()
		Expect(status2).Should(ExitCleanly())
		Expect(status2.OutputToString()).To(ContainSubstring("Running"))

		ctr2 := podmanTest.Podman([]string{"create", "--pod", podName, ALPINE, "top"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitCleanly())

		status3 := podmanTest.Podman([]string{"pod", "inspect", "--format", "{{ .State }}", podName})
		status3.WaitWithDefaultTimeout()
		Expect(status3).Should(ExitCleanly())
		Expect(status3.OutputToString()).To(ContainSubstring("Degraded"))
	})

	It("podman create with unsupported network options", func() {
		podCreate := podmanTest.Podman([]string{"pod", "create", "--network", "container:doesnotmatter"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "pods presently do not support network mode container"))
		Expect(podCreate.ErrorToString()).To(ContainSubstring("pods presently do not support network mode container"))
	})

	It("podman pod create with namespace path networking", func() {
		SkipIfRootless("ip netns is not supported for rootless users")
		SkipIfContainerized("ip netns cannot be run within a container.")

		podName := "netnspod"
		netNsName := "test1"
		networkMode := fmt.Sprintf("ns:/var/run/netns/%s", netNsName)

		addNetns := SystemExec("ip", []string{"netns", "add", netNsName})
		Expect(addNetns).Should(ExitCleanly())
		defer func() {
			delNetns := SystemExec("ip", []string{"netns", "delete", netNsName})
			Expect(delNetns).Should(ExitCleanly())
		}()

		podCreate := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--network", networkMode})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		podStart := podmanTest.Podman([]string{"pod", "start", podName})
		podStart.WaitWithDefaultTimeout()
		Expect(podStart).Should(ExitCleanly())

		inspectPod := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspectPod.WaitWithDefaultTimeout()
		Expect(inspectPod).Should(ExitCleanly())
		inspectPodJSON := inspectPod.InspectPodToJSON()

		inspectInfraContainer := podmanTest.Podman([]string{"inspect", inspectPodJSON.InfraContainerID})
		inspectInfraContainer.WaitWithDefaultTimeout()
		Expect(inspectInfraContainer).Should(ExitCleanly())
		inspectInfraContainerJSON := inspectInfraContainer.InspectContainerToJSON()

		Expect(inspectInfraContainerJSON[0].HostConfig.NetworkMode).To(Equal(networkMode))
	})

	It("podman pod create with --net=none", func() {
		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--network", "none", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "ip", "-o", "-4", "addr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("inet 127.0.0.1/8 scope host lo"))
		Expect(session.OutputToStringArray()).To(HaveLen(1))
	})

	It("podman pod create --infra-image w/untagged image", func() {
		podmanTest.AddImageToRWStore(ALPINE)
		dockerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT ["sleep","99999"]
		`
		// This builds a none/none image
		iid := podmanTest.BuildImage(dockerfile, "", "true")

		create := podmanTest.Podman([]string{"pod", "create", "--infra-image", iid})
		create.WaitWithDefaultTimeout()
		Expect(create).Should(ExitCleanly())
	})

	It("podman pod create --cpus", func() {
		podName := "testPod"
		numCPU := float64(sysinfo.NumCPU())
		period, quota := util.CoresToPeriodAndQuota(numCPU)
		numCPUStr := strconv.Itoa(int(numCPU))
		podCreate := podmanTest.Podman([]string{"pod", "create", "--cpus", numCPUStr, "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		contCreate := podmanTest.Podman([]string{"container", "create", "--pod", podName, "alpine"})
		contCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(ExitCleanly())
		podJSON := podInspect.InspectPodToJSON()
		Expect(podJSON).To(HaveField("CPUPeriod", period))
		Expect(podJSON).To(HaveField("CPUQuota", quota))
	})

	It("podman pod create --cpuset-cpus", func() {
		podName := "testPod"
		ctrName := "testCtr"
		numCPU := float64(sysinfo.NumCPU()) - 1
		numCPUStr := strconv.Itoa(int(numCPU))
		in := "0-" + numCPUStr
		podCreate := podmanTest.Podman([]string{"pod", "create", "--cpuset-cpus", in, "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		contCreate := podmanTest.Podman([]string{"container", "create", "--name", ctrName, "--pod", podName, "alpine"})
		contCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(ExitCleanly())
		podJSON := podInspect.InspectPodToJSON()
		Expect(podJSON).To(HaveField("CPUSetCPUs", in))
	})

	It("podman pod create --pid", func() {
		podName := "pidPod"
		ns := "ns:/proc/self/ns/"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(ExitCleanly())
		podJSON := podInspect.InspectPodToJSON()
		Expect(podJSON.InfraConfig).To(HaveField("PidNS", ns))

		podName = "pidPod2"
		ns = "pod"

		podCreate = podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitWithError(125, "cannot use pod namespace as container is not joining a pod or pod has no infra container: invalid argument"))

		podName = "pidPod3"
		ns = "host"

		podCreate = podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		podInspect = podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(ExitCleanly())
		podJSON = podInspect.InspectPodToJSON()
		Expect(podJSON.InfraConfig).To(HaveField("PidNS", "host"))

		podName = "pidPod4"
		ns = "private"

		podCreate = podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		podInspect = podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(ExitCleanly())
		podJSON = podInspect.InspectPodToJSON()
		Expect(podJSON.InfraConfig).To(HaveField("PidNS", "private"))

		podName = "pidPod5"
		ns = "container:randomfakeid"

		podCreate = podmanTest.Podman([]string{"pod", "create", "--pid", ns, "--name", podName, "--share", "pid"})
		podCreate.WaitWithDefaultTimeout()
		// This can fail in two ways, depending on intricate SELinux specifics:
		// There are actually two different failure messages:
		//   container "randomfakeid" not found: no container with name ...
		//   looking up container to share pid namespace with: no container with name ...
		// Too complicated to differentiate in test context, so we ignore the first part
		// and just check for the "no container" substring, which is common to both.
		Expect(podCreate).Should(ExitWithError(125, `no container with name or ID "randomfakeid" found: no such container`))
	})

	It("podman pod create with --userns=keep-id", func() {
		if !isRootless() {
			Skip("Test only runs without root")
		}

		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--userns", "keep-id", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "id", "-u"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		uid := strconv.Itoa(os.Geteuid())
		Expect(session.OutputToString()).To(ContainSubstring(uid))

		// Check passwd
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "id", "-un"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		Expect(session.OutputToString()).To(Equal(u.Username))

		// root owns /usr
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "stat", "-c%u", "/usr"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("0"))

		// fail if --pod and --userns set together
		session = podmanTest.Podman([]string{"run", "--pod", podName, "--userns", "keep-id", ALPINE, "id", "-u"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitWithError(125, "cannot set user namespace mode when joining pod with infra container"))
	})

	It("podman pod create with --userns=keep-id can add users", func() {
		if !isRootless() {
			Skip("Test only runs without root")
		}

		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--userns", "keep-id", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		// NOTE: we need to use a Fedora image here since the
		// alpine/busybox versions are not capable of dealing with
		// --userns=keep-id and will just error out when not running as
		// "root"
		ctrName := "ctr-name"
		session := podmanTest.Podman([]string{"run", "--pod", podName, "-d", "--stop-signal", "9", "--name", ctrName, FEDORA_MINIMAL, "sleep", "600"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		u, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		// container inside pod inherits user from infra container if --user is not set
		// etc/passwd entry will look like USERNAME:*:1000:1000:Full User Name:/:/bin/sh
		exec1 := podmanTest.Podman([]string{"exec", ctrName, "id", "-un"})
		exec1.WaitWithDefaultTimeout()
		Expect(exec1).Should(ExitCleanly())
		Expect(exec1.OutputToString()).To(Equal(u.Username))

		exec2 := podmanTest.Podman([]string{"exec", ctrName, "useradd", "testuser"})
		exec2.WaitWithDefaultTimeout()
		Expect(exec2).Should(ExitCleanly())

		exec3 := podmanTest.Podman([]string{"exec", ctrName, "cat", "/etc/passwd"})
		exec3.WaitWithDefaultTimeout()
		Expect(exec3).Should(ExitCleanly())
		Expect(exec3.OutputToString()).To(ContainSubstring("testuser"))
	})

	It("podman pod create with --userns=auto", func() {
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

		m := make(map[string]string)
		for i := range 5 {
			podName := "testPod" + strconv.Itoa(i)
			podCreate := podmanTest.Podman([]string{"pod", "create", "--userns=auto", "--name", podName})
			podCreate.WaitWithDefaultTimeout()
			Expect(podCreate).Should(ExitCleanly())

			session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map"})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			l := session.OutputToString()
			Expect(l).To(ContainSubstring("1024"))
			m[l] = l
		}
		// check for no duplicates
		Expect(m).To(HaveLen(5))
	})

	It("podman pod create --userns=auto:size=%d", func() {
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

		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--userns=auto:size=500", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("500"))

		podName = "testPod-1"
		podCreate = podmanTest.Podman([]string{"pod", "create", "--userns=auto:size=3000", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("3000"))
	})

	It("podman pod create --userns=auto:uidmapping=", func() {
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

		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--userns=auto:uidmapping=0:0:1", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(MatchRegexp(`(^|\s)0\s+0\s+1(\s|$)`))

		podName = "testPod-1"
		podCreate = podmanTest.Podman([]string{"pod", "create", "--userns=auto:size=8192,uidmapping=0:0:1", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/uid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("8191"))
	})

	It("podman pod create --userns=auto:gidmapping=", func() {
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

		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--userns=auto:gidmapping=0:0:1", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		session := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/gid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		output := session.OutputToString()
		Expect(output).To(MatchRegexp(`(^|\s)0\s+0\s+1(\s|$)`))

		podName = "testPod-1"
		podCreate = podmanTest.Podman([]string{"pod", "create", "--userns=auto:size=8192,gidmapping=0:0:1", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/proc/self/gid_map"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("8191"))
	})

	It("podman pod create --volume", func() {
		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())
		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--volume", volName + ":/tmp1", "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(ExitCleanly())
		data := podInspect.InspectPodToJSON()
		Expect(data.Mounts[0]).To(HaveField("Name", volName))
		ctrName := "testCtr"
		ctrCreate := podmanTest.Podman([]string{"create", "--pod", podName, "--name", ctrName, ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(ExitCleanly())
		ctrInspect := podmanTest.Podman([]string{"inspect", ctrName})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(ExitCleanly())
		ctrData := ctrInspect.InspectContainerToJSON()
		Expect(ctrData[0].Mounts[0]).To(HaveField("Name", volName))

		ctr2 := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "sh", "-c", "echo hello >> " + "/tmp1/test"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitCleanly())

		ctr3 := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/tmp1/test"})
		ctr3.WaitWithDefaultTimeout()
		Expect(ctr3.OutputToString()).To(ContainSubstring("hello"))

		ctr4 := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "touch", "/tmp1/testing.txt"})
		ctr4.WaitWithDefaultTimeout()
		Expect(ctr4).Should(ExitCleanly())
	})

	It("podman pod create --device", func() {
		SkipIfRootless("Cannot create devices in /dev in rootless mode")
		// path must be unique to this test, not used anywhere else
		devdir := "/dev/devdirpodcreate"
		Expect(os.MkdirAll(devdir, os.ModePerm)).To(Succeed())
		defer os.RemoveAll(devdir)

		mknod := SystemExec("mknod", []string{devdir + "/null", "c", "1", "3"})
		mknod.WaitWithDefaultTimeout()
		Expect(mknod).Should(ExitCleanly())

		podName := "testPod"
		session := podmanTest.Podman([]string{"pod", "create", "--device", devdir + ":/dev/bar", "--name", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"run", "-q", "--pod", podName, ALPINE, "stat", "-c%t:%T", "/dev/bar/null"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("1:3"))
	})

	It("podman pod create --volumes-from", func() {
		volName := "testVol"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())
		ctrName := "testCtr"
		ctrCreate := podmanTest.Podman([]string{"create", "--volume", volName + ":/tmp1", "--name", ctrName, ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(ExitCleanly())
		ctrInspect := podmanTest.Podman([]string{"inspect", ctrName})
		ctrInspect.WaitWithDefaultTimeout()
		Expect(ctrInspect).Should(ExitCleanly())
		data := ctrInspect.InspectContainerToJSON()
		Expect(data[0].Mounts[0]).To(HaveField("Name", volName))
		podName := "testPod"
		podCreate := podmanTest.Podman([]string{"pod", "create", "--volumes-from", ctrName, "--name", podName})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(ExitCleanly())
		podData := podInspect.InspectPodToJSON()
		Expect(podData.Mounts[0]).To(HaveField("Name", volName))

		ctr2 := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "sh", "-c", "echo hello >> " + "/tmp1/test"})
		ctr2.WaitWithDefaultTimeout()
		Expect(ctr2).Should(ExitCleanly())

		ctr3 := podmanTest.Podman([]string{"run", "--pod", podName, ALPINE, "cat", "/tmp1/test"})
		ctr3.WaitWithDefaultTimeout()
		Expect(ctr3.OutputToString()).To(ContainSubstring("hello"))
	})

	It("podman pod create read network mode from config", func() {
		confPath, err := filepath.Abs("config/containers-netns.conf")
		Expect(err).ToNot(HaveOccurred())
		os.Setenv("CONTAINERS_CONF", confPath)
		if IsRemote() {
			podmanTest.RestartRemoteService()
		}

		pod := podmanTest.Podman([]string{"pod", "create", "--name", "test", "--infra-name", "test-infra"})
		pod.WaitWithDefaultTimeout()
		Expect(pod).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.HostConfig.NetworkMode}}", "test-infra"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).Should(Equal("host"))
	})

	It("podman pod create --security-opt", func() {
		if !selinux.GetEnabled() {
			Skip("SELinux not enabled")
		}
		podCreate := podmanTest.Podman([]string{"pod", "create", "--security-opt", "label=type:spc_t", "--security-opt", "seccomp=unconfined"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		ctrCreate := podmanTest.Podman([]string{"container", "create", "--pod", podCreate.OutputToString(), ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(ExitCleanly())

		ctrInspect := podmanTest.InspectContainer(ctrCreate.OutputToString())
		Expect(ctrInspect[0].HostConfig).To(HaveField("SecurityOpt", []string{"label=type:spc_t", "seccomp=unconfined"}))

		podCreate = podmanTest.Podman([]string{"pod", "create", "--security-opt", "label=disable"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		ctrCreate = podmanTest.Podman([]string{"container", "run", "--pod", podCreate.OutputToString(), ALPINE, "cat", "/proc/self/attr/current"})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(ExitCleanly())
		Expect(ctrCreate.OutputToString()).To(ContainSubstring("spc_t"))
	})

	It("podman pod create --security-opt seccomp", func() {
		if !seccomp.IsEnabled() {
			Skip("seccomp is not enabled")
		}
		podCreate := podmanTest.Podman([]string{"pod", "create", "--security-opt", "seccomp=unconfined"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		ctrCreate := podmanTest.Podman([]string{"container", "create", "--pod", podCreate.OutputToString(), ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(ExitCleanly())

		ctrInspect := podmanTest.InspectContainer(ctrCreate.OutputToString())
		Expect(ctrInspect[0].HostConfig).To(HaveField("SecurityOpt", []string{"seccomp=unconfined"}))
	})

	It("podman pod create --security-opt apparmor test", func() {
		if !apparmor.IsEnabled() {
			Skip("Apparmor is not enabled")
		}
		podCreate := podmanTest.Podman([]string{"pod", "create", "--security-opt", fmt.Sprintf("apparmor=%s", apparmor.Profile)})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		ctrCreate := podmanTest.Podman([]string{"container", "create", "--pod", podCreate.OutputToString(), ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(ExitCleanly())

		inspect := podmanTest.InspectContainer(ctrCreate.OutputToString())
		Expect(inspect[0]).To(HaveField("AppArmorProfile", apparmor.Profile))
	})

	It("podman pod create --sysctl test", func() {
		SkipIfRootless("Network sysctls are not available root rootless")
		podCreate := podmanTest.Podman([]string{"pod", "create", "--sysctl", "net.core.somaxconn=65535"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		session := podmanTest.Podman([]string{"run", "--pod", podCreate.OutputToString(), "--rm", ALPINE, "sysctl", "net.core.somaxconn"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("net.core.somaxconn = 65535"))

		// if not sharing the net NS, nothing should fail, but the sysctl should not be passed
		podCreate = podmanTest.Podman([]string{"pod", "create", "--share", "pid", "--sysctl", "net.core.somaxconn=65535"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"run", "--pod", podCreate.OutputToString(), "--rm", ALPINE, "sysctl", "net.core.somaxconn"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).NotTo(ContainSubstring("net.core.somaxconn = 65535"))

		// one other misc option
		podCreate = podmanTest.Podman([]string{"pod", "create", "--sysctl", "kernel.msgmax=65535"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"run", "--pod", podCreate.OutputToString(), "--rm", ALPINE, "sysctl", "kernel.msgmax"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring("kernel.msgmax = 65535"))

		podCreate = podmanTest.Podman([]string{"pod", "create", "--share", "pid", "--sysctl", "kernel.msgmax=65535"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"run", "--pod", podCreate.OutputToString(), "--rm", ALPINE, "sysctl", "kernel.msgmax"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).NotTo(ContainSubstring("kernel.msgmax = 65535"))
	})

	It("podman pod create --share-parent test", func() {
		podCreate := podmanTest.Podman([]string{"pod", "create", "--share-parent=false"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		ctrCreate := podmanTest.Podman([]string{"run", "-dt", "--pod", podCreate.OutputToString(), ALPINE})
		ctrCreate.WaitWithDefaultTimeout()
		Expect(ctrCreate).Should(ExitCleanly())

		inspectPod := podmanTest.Podman([]string{"pod", "inspect", podCreate.OutputToString()})
		inspectPod.WaitWithDefaultTimeout()
		Expect(inspectPod).Should(ExitCleanly())
		data := inspectPod.InspectPodToJSON()

		inspect := podmanTest.InspectContainer(ctrCreate.OutputToString())
		Expect(data.CgroupPath).To(BeEmpty())
		if podmanTest.CgroupManager == "cgroupfs" || !isRootless() {
			Expect(inspect[0].HostConfig.CgroupParent).To(BeEmpty())
		} else if podmanTest.CgroupManager == "systemd" {
			Expect(inspect[0].HostConfig).To(HaveField("CgroupParent", "user.slice"))
		}

		podCreate2 := podmanTest.Podman([]string{"pod", "create", "--share", "cgroup,ipc,net,uts", "--share-parent=false", "--infra-name", "cgroupCtr"})
		podCreate2.WaitWithDefaultTimeout()
		Expect(podCreate2).Should(ExitCleanly())

		ctrCreate2 := podmanTest.Podman([]string{"run", "-dt", "--pod", podCreate2.OutputToString(), ALPINE})
		ctrCreate2.WaitWithDefaultTimeout()
		Expect(ctrCreate2).Should(ExitCleanly())

		inspectInfra := podmanTest.InspectContainer("cgroupCtr")

		inspect2 := podmanTest.InspectContainer(ctrCreate2.OutputToString())

		Expect(inspect2[0].HostConfig.CgroupMode).To(ContainSubstring(inspectInfra[0].ID))

		podCreate3 := podmanTest.Podman([]string{"pod", "create", "--share", "cgroup"})
		podCreate3.WaitWithDefaultTimeout()
		Expect(podCreate3).ShouldNot(ExitCleanly())
	})

	It("podman pod create infra inheritance test", func() {
		volName := "testVol1"
		volCreate := podmanTest.Podman([]string{"volume", "create", volName})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"pod", "create", "-v", volName + ":/vol1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		volName2 := "testVol2"
		volCreate = podmanTest.Podman([]string{"volume", "create", volName2})
		volCreate.WaitWithDefaultTimeout()
		Expect(volCreate).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", session.OutputToString(), "-v", volName2 + ":/vol2", ALPINE, "mount"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).Should(ContainSubstring("/vol1"))
		Expect(session.OutputToString()).Should(ContainSubstring("/vol2"))
	})

	It("podman pod create --shm-size", func() {
		podCreate := podmanTest.Podman([]string{"pod", "create", "--shm-size", "10mb"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		run := podmanTest.Podman([]string{"run", "--pod", podCreate.OutputToString(), ALPINE, "mount"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		t, strings := run.GrepString("shm on /dev/shm type tmpfs")
		Expect(t).To(BeTrue(), "found /dev/shm")
		Expect(strings[0]).Should(ContainSubstring("size=10240k"))
	})

	It("podman pod create --shm-size and --ipc=host conflict", func() {
		podCreate := podmanTest.Podman([]string{"pod", "create", "--shm-size", "10mb"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		run := podmanTest.Podman([]string{"run", "-dt", "--pod", podCreate.OutputToString(), "--ipc", "host", ALPINE})
		run.WaitWithDefaultTimeout()
		Expect(run).ShouldNot(ExitCleanly())
	})

	It("podman pod create --uts test", func() {
		session := podmanTest.Podman([]string{"pod", "create", "--uts", "host"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"run", "--pod", session.OutputToString(), ALPINE, "printenv", "HOSTNAME"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(ContainSubstring(hostname))

		podName := "utsPod"
		ns := "ns:/proc/self/ns/"

		// just share uts with a custom path
		podCreate := podmanTest.Podman([]string{"pod", "create", "--uts", ns, "--name", podName, "--share", "uts"})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		podInspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		podInspect.WaitWithDefaultTimeout()
		Expect(podInspect).Should(ExitCleanly())
		podJSON := podInspect.InspectPodToJSON()
		Expect(podJSON.InfraConfig).To(HaveField("UtsNS", ns))
	})

	It("podman pod create --shm-size-systemd", func() {
		podName := "testShmSizeSystemd"
		session := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--shm-size-systemd", "10mb"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// add container to pod
		ctrRun := podmanTest.Podman([]string{"run", "-d", "--pod", podName, SYSTEMD_IMAGE, "/sbin/init"})
		ctrRun.WaitWithDefaultTimeout()
		Expect(ctrRun).Should(ExitCleanly())

		run := podmanTest.Podman([]string{"exec", ctrRun.OutputToString(), "mount"})
		run.WaitWithDefaultTimeout()
		Expect(run).Should(ExitCleanly())
		t, strings := run.GrepString("tmpfs on /run/lock")
		Expect(t).To(BeTrue(), "found /run/lock")
		Expect(strings[0]).Should(ContainSubstring("size=10240k"))
	})

	It("create pod with name subset of existing ID", func() {
		create1 := podmanTest.Podman([]string{"pod", "create"})
		create1.WaitWithDefaultTimeout()
		Expect(create1).Should(ExitCleanly())
		pod1ID := create1.OutputToString()

		pod2Name := pod1ID[:5]
		create2 := podmanTest.Podman([]string{"pod", "create", pod2Name})
		create2.WaitWithDefaultTimeout()
		Expect(create2).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"pod", "inspect", "--format", "{{.Name}}", pod2Name})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).Should(Equal(pod2Name))
	})

	It("podman pod create --restart set to default", func() {
		// When the --restart flag is not set, the default value is No
		// TODO: v5.0 change this so that the default value is Always
		podName := "mypod"
		testCtr := "ctr1"
		session := podmanTest.Podman([]string{"pod", "create", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// add container to pod
		ctrRun := podmanTest.Podman([]string{"run", "--name", testCtr, "-d", "--pod", podName, ALPINE, "echo", "hello"})
		ctrRun.WaitWithDefaultTimeout()
		Expect(ctrRun).Should(ExitCleanly())
		// Wait about 1 second, so we can check the number of restarts as default restart policy is set to No
		time.Sleep(1 * time.Second)
		ps := podmanTest.Podman([]string{"ps", "-a", "--filter", "name=" + testCtr, "--format", "{{.Restarts}}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		restarts, err := strconv.Atoi(ps.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		Expect(restarts).To(BeNumerically("==", 0))
		ps = podmanTest.Podman([]string{"ps", "-a", "--filter", "name=" + testCtr, "--format", "{{.Status}}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(ContainSubstring("Exited"))
	})

	It("podman pod create --restart=on-failure", func() {
		// Restart policy set to on-failure with max 2 retries
		podName := "mypod"
		runningCtr := "ctr1"
		testCtr := "ctr2"
		session := podmanTest.Podman([]string{"pod", "create", "--restart", "on-failure:2", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		// add container to pod
		ctrRun := podmanTest.Podman([]string{"run", "--name", runningCtr, "-d", "--pod", podName, ALPINE, "sleep", "100"})
		ctrRun.WaitWithDefaultTimeout()
		Expect(ctrRun).Should(ExitCleanly())
		ctrRun = podmanTest.Podman([]string{"run", "--name", testCtr, "-d", "--pod", podName, ALPINE, "sh", "-c", "echo hello && exit 1"})
		ctrRun.WaitWithDefaultTimeout()
		Expect(ctrRun).Should(ExitCleanly())
		// Wait about 2 seconds, so we can check the number of restarts after failure
		time.Sleep(2 * time.Second)
		ps := podmanTest.Podman([]string{"ps", "-a", "--filter", "name=" + testCtr, "--format", "{{.Restarts}}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		restarts, err := strconv.Atoi(ps.OutputToString())
		Expect(err).ToNot(HaveOccurred())
		Expect(restarts).To(BeNumerically("==", 2))
		ps = podmanTest.Podman([]string{"ps", "-a", "--filter", "name=" + testCtr, "--format", "{{.Status}}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(ContainSubstring("Exited"))
		ps = podmanTest.Podman([]string{"ps", "-a", "--filter", "name=" + runningCtr, "--format", "{{.Status}}"})
		ps.WaitWithDefaultTimeout()
		Expect(ps).Should(ExitCleanly())
		Expect(ps.OutputToString()).To(ContainSubstring("Up"))
	})

	It("podman pod create --restart=no/never", func() {
		// never and no are the same, just different words to do the same thing
		policy := []string{"no", "never"}
		for _, p := range policy {
			podName := "mypod-" + p
			runningCtr := "ctr1-" + p
			testCtr := "ctr2-" + p
			testCtr2 := "ctr3-" + p
			session := podmanTest.Podman([]string{"pod", "create", "--restart", p, podName})
			session.WaitWithDefaultTimeout()
			Expect(session).Should(ExitCleanly())
			// add container to pod
			ctrRun := podmanTest.Podman([]string{"run", "--name", runningCtr, "-d", "--pod", podName, ALPINE, "sleep", "100"})
			ctrRun.WaitWithDefaultTimeout()
			Expect(ctrRun).Should(ExitCleanly())
			ctrRun = podmanTest.Podman([]string{"run", "--name", testCtr, "-d", "--pod", podName, ALPINE, "echo", "hello"})
			ctrRun.WaitWithDefaultTimeout()
			Expect(ctrRun).Should(ExitCleanly())
			ctrRun = podmanTest.Podman([]string{"run", "--name", testCtr2, "-d", "--pod", podName, ALPINE, "sh", "-c", "echo hello && exit 1"})
			ctrRun.WaitWithDefaultTimeout()
			Expect(ctrRun).Should(ExitCleanly())
			// Wait 1 second, so we can check the number of restarts and make sure the container has actually ran
			time.Sleep(1 * time.Second)
			// check first test container - container exits with exit code 0
			ps := podmanTest.Podman([]string{"ps", "-a", "--filter", "name=" + testCtr, "--format", "{{.Restarts}}"})
			ps.WaitWithDefaultTimeout()
			Expect(ps).Should(ExitCleanly())
			restarts, err := strconv.Atoi(ps.OutputToString())
			Expect(err).ToNot(HaveOccurred())
			Expect(restarts).To(BeNumerically("==", 0))
			ps = podmanTest.Podman([]string{"ps", "-a", "--filter", "name=" + testCtr, "--format", "{{.Status}}"})
			ps.WaitWithDefaultTimeout()
			Expect(ps).Should(ExitCleanly())
			Expect(ps.OutputToString()).To(ContainSubstring("Exited"))
			// Check second test container - container exits with non-zero exit code
			ps = podmanTest.Podman([]string{"ps", "-a", "--filter", "name=" + testCtr2, "--format", "{{.Restarts}}"})
			ps.WaitWithDefaultTimeout()
			Expect(ps).Should(ExitCleanly())
			restarts, err = strconv.Atoi(ps.OutputToString())
			Expect(err).ToNot(HaveOccurred())
			Expect(restarts).To(BeNumerically("==", 0))
			ps = podmanTest.Podman([]string{"ps", "-a", "--filter", "name=" + testCtr2, "--format", "{{.Status}}"})
			ps.WaitWithDefaultTimeout()
			Expect(ps).Should(ExitCleanly())
			Expect(ps.OutputToString()).To(ContainSubstring("Exited"))
			ps = podmanTest.Podman([]string{"ps", "-a", "--filter", "name=" + runningCtr, "--format", "{{.Status}}"})
			ps.WaitWithDefaultTimeout()
			Expect(ps).Should(ExitCleanly())
			Expect(ps.OutputToString()).To(ContainSubstring("Up"))
		}
	})
})
