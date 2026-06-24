//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"go.podman.io/podman/v6/libpod/define"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/util"
	. "go.podman.io/podman/v6/test/utils"
	"sigs.k8s.io/yaml"
)

var _ = Describe("Podman kube generate", func() {
	It("pod on bogus object", func() {
		session := podmanTest.Podman([]string{"generate", "kube", "foobarpod"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `name or ID "foobarpod" not found`))
	})

	It("service on bogus object", func() {
		session := podmanTest.Podman([]string{"kube", "generate", "-s", "foobarservice"})
		session.WaitWithDefaultTimeout()
		Expect(session).To(ExitWithError(125, `name or ID "foobarservice" not found`))
	})

	It("on container", func() {
		session := podmanTest.RunTopContainer("top")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", false))
		Expect(pod.Spec.SecurityContext).To(BeNil())
		Expect(pod.Spec.DNSConfig).To(BeNil())
		Expect(pod.Spec.Containers[0]).To(HaveField("WorkingDir", ""))
		Expect(pod.Spec.Containers[0].SecurityContext).To(BeNil())
		Expect(pod.Spec.Containers[0].Env).To(BeNil())
		Expect(pod).To(HaveField("Name", "top-pod"))
		Expect(pod.Spec.TerminationGracePeriodSeconds).To(BeNil())

		numContainers := 0
		for range pod.Spec.Containers {
			numContainers++
		}
		Expect(numContainers).To(Equal(1))
	})

	It("service on container with --security-opt level", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test", "--security-opt", "label=level:s0:c100,c200", CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "test"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(kube.OutputToString()).To(ContainSubstring("level: s0:c100,c200"))
	})

	It("service kube on container with --security-opt disable", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test-disable", "--security-opt", "label=disable", CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "test-disable"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(kube.OutputToString()).To(ContainSubstring("type: spc_t"))
	})

	It("service kube on container with --security-opt type", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test", "--security-opt", "label=type:foo_bar_t", CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "test"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(kube.OutputToString()).To(ContainSubstring("type: foo_bar_t"))
	})

	It("service kube on container - targetPort should match port name", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test-ctr", "-p", "3890:3890", CITEST_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "-s", "test-ctr"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Separate out the Service and Pod yaml
		arr := strings.Split(string(kube.Out.Contents()), "---")
		Expect(arr).To(HaveLen(2))

		svc := new(v1.Service)
		err := yaml.Unmarshal([]byte(arr[0]), svc)
		Expect(err).ToNot(HaveOccurred())
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].TargetPort.IntValue()).To(Equal(3890))

		pod := new(v1.Pod)
		err = yaml.Unmarshal([]byte(arr[1]), pod)
		Expect(err).ToNot(HaveOccurred())
	})

	It("on container with and without service", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test-ctr", "-p", "3890:3890", CITEST_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "-s", "test-ctr"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Separate out the Service and Pod yaml
		arr := strings.Split(string(kube.Out.Contents()), "---")
		Expect(arr).To(HaveLen(2))

		svc := new(v1.Service)
		err := yaml.Unmarshal([]byte(arr[0]), svc)
		Expect(err).ToNot(HaveOccurred())
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].TargetPort.IntValue()).To(Equal(3890))

		pod := new(v1.Pod)
		err = yaml.Unmarshal([]byte(arr[1]), pod)
		Expect(err).ToNot(HaveOccurred())
		// Since hostPort will not be set in the yaml, when we unmarshal it it will have a value of 0
		Expect(pod.Spec.Containers[0].Ports[0].HostPort).To(Equal(int32(0)))

		// Now do kube generate without the --service flag
		kube = podmanTest.Podman([]string{"kube", "generate", "test-ctr"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// The hostPort in the pod yaml should be set to 3890
		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers[0].Ports[0].HostPort).To(Equal(int32(3890)))
	})

	It("on pod", func() {
		_, rc, _ := podmanTest.CreatePod(map[string][]string{"--name": {"toppod"}})
		Expect(rc).To(Equal(0))

		session := podmanTest.RunTopContainerInPod("topcontainer", "toppod")
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "toppod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", false))
		Expect(pod.Annotations).To(HaveLen(1))

		numContainers := 0
		for range pod.Spec.Containers {
			numContainers++
		}
		Expect(numContainers).To(Equal(1))
	})

	It("multiple pods", func() {
		pod1 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod1", CITEST_IMAGE, "top"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1).Should(ExitCleanly())

		pod2 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod2", CITEST_IMAGE, "top"})
		pod2.WaitWithDefaultTimeout()
		Expect(pod2).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "pod1", "pod2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		Expect(string(kube.Out.Contents())).To(ContainSubstring(`name: pod1`))
		Expect(string(kube.Out.Contents())).To(ContainSubstring(`name: pod2`))
	})

	It("on pod with init containers", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:toppod", "--init-ctr", "always", CITEST_IMAGE, "echo", "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", "toppod", "--init-ctr", "always", CITEST_IMAGE, "echo", "world"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", "toppod", CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "toppod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", false))

		numContainers := len(pod.Spec.Containers) + len(pod.Spec.InitContainers)
		Expect(numContainers).To(Equal(3))

		// Init container should be in the generated kube yaml if created with "once" type and the pod has not been started
		session = podmanTest.Podman([]string{"create", "--pod", "new:toppod-2", "--init-ctr", "once", CITEST_IMAGE, "echo", "using once type"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", "toppod-2", CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube = podmanTest.Podman([]string{"kube", "generate", "toppod-2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", false))

		numContainers = len(pod.Spec.Containers) + len(pod.Spec.InitContainers)
		Expect(numContainers).To(Equal(2))

		// Init container should not be in the generated kube yaml if created with "once" type and the pod has been started
		session = podmanTest.Podman([]string{"create", "--pod", "new:toppod-3", "--init-ctr", "once", CITEST_IMAGE, "echo", "using once type"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", "toppod-3", CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "start", "toppod-3"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube = podmanTest.Podman([]string{"kube", "generate", "toppod-3"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", false))

		numContainers = len(pod.Spec.Containers) + len(pod.Spec.InitContainers)
		Expect(numContainers).To(Equal(1))
	})

	It("on pod with user namespace", func() {
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
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", "testPod", "--userns=auto"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"create", "--name", "topcontainer", "--pod", "testPod", CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "testPod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		expected := false
		Expect(pod.Spec).To(HaveField("HostUsers", &expected))
	})

	It("on pod with host network", func() {
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", "testHostNetwork", "--network", "host"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"create", "--name", "topcontainer", "--pod", "testHostNetwork", "--network", "host", CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "testHostNetwork"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", true))
	})

	It("on container with host network", func() {
		session := podmanTest.RunTopContainerWithArgs("topcontainer", []string{"--network", "host"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "topcontainer"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec).To(HaveField("HostNetwork", true))
	})

	It("on pod with hostAliases", func() {
		podName := "testHost"
		testIP := "127.0.0.1"
		podSession := podmanTest.Podman([]string{
			"pod", "create", "--name", podName,
			"--add-host", "test1.podman.io" + ":" + testIP,
			"--add-host", "test2.podman.io" + ":" + testIP,
		})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, CITEST_IMAGE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(ExitCleanly())

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName, CITEST_IMAGE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.HostAliases).To(HaveLen(2))
		Expect(pod.Spec.HostAliases[0]).To(HaveField("IP", testIP))
		Expect(pod.Spec.HostAliases[1]).To(HaveField("IP", testIP))
	})

	It("with network sharing", func() {
		// Expect error with default sharing options as Net namespace is shared
		podName := "testPod"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		ctrSession := podmanTest.Podman([]string{"create", "--name", "testCtr", "--pod", podName, "-p", "9000:8000", CITEST_IMAGE, "top"})
		ctrSession.WaitWithDefaultTimeout()
		Expect(ctrSession).Should(ExitWithError(125, "invalid config provided: published or exposed ports must be defined when the pod is created: network cannot be configured when it is shared with a pod"))

		// Ports without Net sharing should work with ports being set for each container in the generated kube yaml
		podName = "testNet"
		podSession = podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", "ipc"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, "-p", "9000:8000", CITEST_IMAGE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(ExitCleanly())

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName, "-p", "6000:5000", CITEST_IMAGE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers).To(HaveLen(2))
		Expect(pod.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(8000)))
		Expect(pod.Spec.Containers[1].Ports[0].ContainerPort).To(Equal(int32(5000)))
		Expect(pod.Spec.Containers[0].Ports[0].HostPort).To(Equal(int32(9000)))
		Expect(pod.Spec.Containers[1].Ports[0].HostPort).To(Equal(int32(6000)))
	})

	It("with and without hostname", func() {
		// Expect error with default sharing options as UTS namespace is shared
		podName := "testPod"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		ctrSession := podmanTest.Podman([]string{"create", "--name", "testCtr", "--pod", podName, "--hostname", "test-hostname", CITEST_IMAGE, "top"})
		ctrSession.WaitWithDefaultTimeout()
		Expect(ctrSession).Should(ExitWithError(125, "invalid config provided: cannot set hostname when joining the pod UTS namespace: invalid configuration"))

		// Hostname without uts sharing should work, but generated kube yaml will have pod hostname
		// set to the hostname of the first container
		podName = "testHostname"
		podSession = podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", "ipc"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		ctr1Name := "ctr1"
		ctr1HostName := "ctr1-hostname"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, "--hostname", ctr1HostName, CITEST_IMAGE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(ExitCleanly())

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName, CITEST_IMAGE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers).To(HaveLen(2))
		Expect(pod.Spec.Hostname).To(Equal(ctr1HostName))

		// No hostname

		podName = "testNoHostname"
		podSession = podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", "ipc"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		ctr3Name := "ctr3"
		ctr3Session := podmanTest.Podman([]string{"create", "--name", ctr3Name, "--pod", podName, CITEST_IMAGE, "top"})
		ctr3Session.WaitWithDefaultTimeout()
		Expect(ctr3Session).Should(ExitCleanly())

		kube = podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers).To(HaveLen(1))
		Expect(pod.Spec.Hostname).To(BeEmpty())
	})

	It("service kube on pod", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:test-pod", "-p", "4000:4000/udp", CITEST_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "-s", "test-pod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Separate out the Service and Pod yaml
		arr := strings.Split(string(kube.Out.Contents()), "---")
		Expect(arr).To(HaveLen(2))

		svc := new(v1.Service)
		err := yaml.Unmarshal([]byte(arr[0]), svc)
		Expect(err).ToNot(HaveOccurred())
		Expect(svc.Spec.Ports).To(HaveLen(1))
		Expect(svc.Spec.Ports[0].TargetPort.IntValue()).To(Equal(4000))
		Expect(svc.Spec.Ports[0]).To(HaveField("Protocol", v1.ProtocolUDP))

		pod := new(v1.Pod)
		err = yaml.Unmarshal([]byte(arr[1]), pod)
		Expect(err).ToNot(HaveOccurred())
	})

	It("on pod with restartPolicy set for container in a pod", func() {
		// TODO: v5.0 - change/remove test once we block --restart on container when it is in a pod
		// podName,  set,  expect
		testSli := [][]string{
			{"testPod1", "", ""}, // some pod create from cmdline, so set it to an empty string and let k8s default it to Always
			{"testPod2", "always", "Always"},
			{"testPod3", "on-failure", "OnFailure"},
			{"testPod4", "no", "Never"},
			{"testPod5", "never", "Never"},
		}

		for k, v := range testSli {
			podName := v[0]
			// Need to set --restart during pod creation as gen kube only picks up the pod's restart policy
			podSession := podmanTest.Podman([]string{"pod", "create", "--restart", v[1], "--name", podName})
			podSession.WaitWithDefaultTimeout()
			Expect(podSession).Should(ExitCleanly())

			ctrName := "ctr" + strconv.Itoa(k)
			ctr1Session := podmanTest.Podman([]string{
				"create", "--name", ctrName, "--pod", podName,
				"--restart", v[1], CITEST_IMAGE, "top",
			})
			ctr1Session.WaitWithDefaultTimeout()
			Expect(ctr1Session).Should(ExitCleanly())

			kube := podmanTest.Podman([]string{"kube", "generate", podName})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(ExitCleanly())

			pod := new(v1.Pod)
			err := yaml.Unmarshal(kube.Out.Contents(), pod)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(pod.Spec.RestartPolicy)).To(Equal(v[2]))
		}
	})

	It("on pod with restartPolicy", func() {
		// podName,  set,  expect
		testSli := [][]string{
			{"testPod1", "", ""},
			{"testPod2", "always", "Always"},
			{"testPod3", "on-failure", "OnFailure"},
			{"testPod4", "no", "Never"},
			{"testPod5", "never", "Never"},
		}

		for k, v := range testSli {
			podName := v[0]
			podSession := podmanTest.Podman([]string{"pod", "create", "--restart", v[1], podName})
			podSession.WaitWithDefaultTimeout()
			Expect(podSession).Should(ExitCleanly())

			ctrName := "ctr" + strconv.Itoa(k)
			ctr1Session := podmanTest.Podman([]string{"create", "--name", ctrName, "--pod", podName, CITEST_IMAGE, "top"})
			ctr1Session.WaitWithDefaultTimeout()
			Expect(ctr1Session).Should(ExitCleanly())

			kube := podmanTest.Podman([]string{"kube", "generate", podName})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(ExitCleanly())

			pod := new(v1.Pod)
			err := yaml.Unmarshal(kube.Out.Contents(), pod)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(pod.Spec.RestartPolicy)).To(Equal(v[2]))
		}
	})

	It("on ctr with restartPolicy", func() {
		// podName,  set,  expect
		testSli := [][]string{
			{"", ""}, // some ctr created from cmdline, set it to "" and let k8s default it to Always
			{"always", "Always"},
			{"on-failure", "OnFailure"},
			{"no", "Never"},
			{"never", "Never"},
		}

		for k, v := range testSli {
			ctrName := "ctr" + strconv.Itoa(k)
			ctrSession := podmanTest.Podman([]string{"create", "--restart", v[0], "--name", ctrName, CITEST_IMAGE, "top"})
			ctrSession.WaitWithDefaultTimeout()
			Expect(ctrSession).Should(ExitCleanly())

			kube := podmanTest.Podman([]string{"kube", "generate", ctrName})
			kube.WaitWithDefaultTimeout()
			Expect(kube).Should(ExitCleanly())

			pod := new(v1.Pod)
			err := yaml.Unmarshal(kube.Out.Contents(), pod)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(pod.Spec.RestartPolicy)).To(Equal(v[1]))
		}
	})

	It("on pod with memory limit", func() {
		podName := "testMemoryLimit"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, "--memory", "10M", CITEST_IMAGE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		for _, ctr := range pod.Spec.Containers {
			memoryLimit, _ := ctr.Resources.Limits.Memory().AsInt64()
			Expect(memoryLimit).To(Equal(int64(10 * 1024 * 1024)))
		}
	})

	It("on pod with cpu limit", func() {
		podName := "testCpuLimit"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{
			"create", "--name", ctr1Name, "--pod", podName,
			"--cpus", "0.5", CITEST_IMAGE, "top",
		})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(ExitCleanly())

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{
			"create", "--name", ctr2Name, "--pod", podName,
			"--cpu-period", "100000", "--cpu-quota", "50000", CITEST_IMAGE, "top",
		})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		for _, ctr := range pod.Spec.Containers {
			cpuLimit := ctr.Resources.Limits.Cpu().MilliValue()
			Expect(cpuLimit).To(Equal(int64(500)))
		}
	})

	It("on pod with ports", func() {
		podName := "test"

		lock4 := GetPortLock("4008")
		defer lock4.Unlock()
		lock5 := GetPortLock("5008")
		defer lock5.Unlock()
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName, "-p", "4008:4000", "-p", "5008:5000"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		ctr1Name := "ctr1"
		ctr1Session := podmanTest.Podman([]string{"create", "--name", ctr1Name, "--pod", podName, CITEST_IMAGE, "top"})
		ctr1Session.WaitWithDefaultTimeout()
		Expect(ctr1Session).Should(ExitCleanly())

		ctr2Name := "ctr2"
		ctr2Session := podmanTest.Podman([]string{"create", "--name", ctr2Name, "--pod", podName, CITEST_IMAGE, "top"})
		ctr2Session.WaitWithDefaultTimeout()
		Expect(ctr2Session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		foundPort400x := 0
		foundPort500x := 0
		foundOtherPort := 0
		for _, ctr := range pod.Spec.Containers {
			for _, port := range ctr.Ports {
				// Since we are using tcp here, the generated kube yaml shouldn't
				// have anything for protocol under the ports as tcp is the default
				// for k8s
				Expect(port.Protocol).To(BeEmpty())
				switch port.HostPort {
				case 4008:
					foundPort400x++
				case 5008:
					foundPort500x++
				default:
					foundOtherPort++
				}
			}
		}
		Expect(foundPort400x).To(Equal(1))
		Expect(foundPort500x).To(Equal(1))
		Expect(foundOtherPort).To(Equal(0))

		// Create container with UDP port and check the generated kube yaml
		ctrWithUDP := podmanTest.Podman([]string{"create", "--pod", "new:test-pod", "-p", "6666:66/udp", CITEST_IMAGE, "top"})
		ctrWithUDP.WaitWithDefaultTimeout()
		Expect(ctrWithUDP).Should(ExitCleanly())

		kube = podmanTest.Podman([]string{"kube", "generate", "test-pod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers := pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0].Ports).To(HaveLen(1))
		Expect(containers[0].Ports[0]).To(HaveField("Protocol", v1.ProtocolUDP))
	})

	It("generate and reimport kube on pod", func() {
		podName := "toppod"
		_, rc, _ := podmanTest.CreatePod(map[string][]string{"--name": {podName}})
		Expect(rc).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", podName, "--name", "test1", CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session2 := podmanTest.Podman([]string{"create", "--pod", podName, "--name", "test2", CITEST_IMAGE, "top"})
		session2.WaitWithDefaultTimeout()
		Expect(session2).Should(ExitCleanly())

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"kube", "generate", "-f", outputFile, podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		session3 := podmanTest.Podman([]string{"pod", "rm", "-af"})
		session3.WaitWithDefaultTimeout()
		Expect(session3).Should(ExitCleanly())

		session4 := podmanTest.Podman([]string{"kube", "play", outputFile})
		session4.WaitWithDefaultTimeout()
		Expect(session4).Should(ExitCleanly())

		session5 := podmanTest.Podman([]string{"pod", "ps"})
		session5.WaitWithDefaultTimeout()
		Expect(session5).Should(ExitCleanly())
		Expect(session5.OutputToString()).To(ContainSubstring(podName))

		session6 := podmanTest.Podman([]string{"ps", "-a"})
		session6.WaitWithDefaultTimeout()
		Expect(session6).Should(ExitCleanly())
		psOut := session6.OutputToString()
		Expect(psOut).To(ContainSubstring("test1"))
		Expect(psOut).To(ContainSubstring("test2"))
	})

	It("generate with user and reimport kube on pod", func() {
		podName := "toppod"
		_, rc, _ := podmanTest.CreatePod(map[string][]string{"--name": {podName}})
		Expect(rc).To(Equal(0))

		session := podmanTest.Podman([]string{"create", "--pod", podName, "--name", "test1", "--user", "100:200", CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", "--format", "{{.Config.User}}", "test1"})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring("100:200"))

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"kube", "generate", "-f", outputFile, podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"pod", "rm", "-af"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		podmanTest.AddImageToRWStore(CITEST_IMAGE)
		session = podmanTest.Podman([]string{"kube", "play", outputFile})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		// container name in pod is <podName>-<ctrName>
		inspect1 := podmanTest.Podman([]string{"inspect", "--format", "{{.Config.User}}", "toppod-test1"})
		inspect1.WaitWithDefaultTimeout()
		Expect(inspect1).Should(ExitCleanly())
		Expect(inspect1.OutputToString()).To(ContainSubstring(inspect.OutputToString()))
	})

	It("with volume", func() {
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		err := os.MkdirAll(vol1, 0o755)
		Expect(err).ToNot(HaveOccurred())

		// we need a container name because IDs don't persist after rm/play
		ctrName := "test-ctr"
		ctrNameInKubePod := "test1-test-ctr"

		session1 := podmanTest.Podman([]string{"run", "-d", "--pod", "new:test1", "--name", ctrName, "-v", vol1 + ":/volume/:z", CITEST_IMAGE, "top"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"kube", "generate", "test1", "-f", outputFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		b, err := os.ReadFile(outputFile)
		Expect(err).ShouldNot(HaveOccurred())
		pod := new(v1.Pod)
		err = yaml.Unmarshal(b, pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.BindMountPrefix, vol1+":"+"z"))

		rm := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", "test1"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())

		play := podmanTest.Podman([]string{"kube", "play", outputFile})
		play.WaitWithDefaultTimeout()
		Expect(play).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", ctrNameInKubePod})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(vol1))
	})

	It("with subpath volume", func() {
		// We want to verify that generating Volume's subPath is working properly
		// https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath
		// by creating a volume with two directories and mounthing them separately
		volName := "vol-test1"
		pvcName := fmt.Sprintf("%s-pvc", volName)
		podmanTest.PodmanExitCleanly("volume", "create", volName)

		mountPoint := "/mnt"

		etcFile := "etcfile"
		etcDirPath := filepath.Join(mountPoint, "etc")
		etcSubPath := filepath.Join("/etc", etcFile)
		etcMountSubPath := filepath.Join(mountPoint, etcSubPath)
		populateEtcCmd := fmt.Sprintf("mkdir -p %s; touch %s", etcDirPath, etcMountSubPath)

		varFile := "varfile"
		varDirPath := filepath.Join(mountPoint, "var")
		varSubPath := filepath.Join("/var", varFile)
		varMountSubPath := filepath.Join(mountPoint, varSubPath)
		populateVarCmd := fmt.Sprintf("mkdir -p %s; touch %s", varDirPath, varMountSubPath)

		cmd := fmt.Sprintf("%s; %s", populateEtcCmd, populateVarCmd)
		vol := fmt.Sprintf("%s:%s", volName, mountPoint)
		podmanTest.PodmanExitCleanly("run", "-v", vol, ALPINE, "sh", "-c", cmd)

		etcTargetPath := filepath.Join(mountPoint, etcFile)
		varTargetPath := filepath.Join(mountPoint, varFile)
		mountTemplate := "type=volume,source=%s,volume-subpath=%s,target=%s"
		podmanTest.PodmanExitCleanly("run", "-d", "--pod", "new:test1", "--name", "test-ctr",
			"--mount", fmt.Sprintf(mountTemplate, volName, etcSubPath, etcTargetPath),
			"--mount", fmt.Sprintf(mountTemplate, volName, varSubPath, varTargetPath),
			CITEST_IMAGE, "top")

		kube := podmanTest.PodmanExitCleanly("kube", "generate", "test1")

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers[0].VolumeMounts).To(ContainElements(v1.VolumeMount{
			Name:      pvcName,
			MountPath: etcTargetPath,
			SubPath:   etcSubPath,
		}, v1.VolumeMount{
			Name:      pvcName,
			MountPath: varTargetPath,
			SubPath:   varSubPath,
		}),
		)
	})

	It("when bind-mounting '/' and '/root' at the same time ", func() {
		// Fixes https://github.com/containers/podman/issues/9764

		ctrName := "mount-root-ctr"
		session1 := podmanTest.Podman([]string{
			"run", "-d", "--pod", "new:mount-root-conflict", "--name", ctrName,
			"-v", "/:/volume1/",
			"-v", "/root:/volume2/",
			CITEST_IMAGE, "top",
		})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "mount-root-conflict"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.Volumes).To(HaveLen(2))
	})

	It("with persistent volume claim", func() {
		vol := "vol-test-persistent-volume-claim"

		// we need a container name because IDs don't persist after rm/play
		ctrName := "test-persistent-volume-claim"
		ctrNameInKubePod := "test1-test-persistent-volume-claim"

		session := podmanTest.Podman([]string{"run", "-d", "--pod", "new:test1", "--name", ctrName, "-v", vol + ":/volume/:z", CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"kube", "generate", "test1", "-f", outputFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		rm := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", "test1"})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())

		play := podmanTest.Podman([]string{"kube", "play", outputFile})
		play.WaitWithDefaultTimeout()
		Expect(play).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", ctrNameInKubePod})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(vol))
	})

	It("sharing pid namespace", func() {
		podName := "test"
		podSession := podmanTest.Podman([]string{"pod", "create", "--name", podName, "--share", "pid"})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"create", "--pod", podName, "--name", "test1", CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"kube", "generate", podName, "-f", outputFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		rm := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", podName})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())

		play := podmanTest.Podman([]string{"kube", "play", outputFile})
		play.WaitWithDefaultTimeout()
		Expect(play).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"pod", "inspect", podName})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring(`"pid"`))
	})

	It("with pods and containers", func() {
		pod1 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod1", CITEST_IMAGE, "top"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1).Should(ExitCleanly())

		pod2 := podmanTest.Podman([]string{"run", "-dt", "--name", "top", CITEST_IMAGE, "top"})
		pod2.WaitWithDefaultTimeout()
		Expect(pod2).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "pod1", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())
	})

	It("with containers in a pod should fail", func() {
		pod1 := podmanTest.Podman([]string{"pod", "create", "--name", "pod1"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1).Should(ExitCleanly())

		con := podmanTest.Podman([]string{"run", "-dt", "--pod", "pod1", "--name", "top", CITEST_IMAGE, "top"})
		con.WaitWithDefaultTimeout()
		Expect(con).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, " is associated with pod "))
	})

	It("with multiple containers", func() {
		con1 := podmanTest.Podman([]string{"run", "-dt", "--name", "con1", CITEST_IMAGE, "top"})
		con1.WaitWithDefaultTimeout()
		Expect(con1).Should(ExitCleanly())

		con2 := podmanTest.Podman([]string{"run", "-dt", "--name", "con2", CITEST_IMAGE, "top"})
		con2.WaitWithDefaultTimeout()
		Expect(con2).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "con1", "con2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())
	})

	It("multiple containers with same user namespace configuration", func() {
		name1 := "c1"
		name2 := "c2"
		_ = podmanTest.PodmanExitCleanly("run", "--userns", "auto:size=30", "-dt", "--name", name1, ALPINE, "top")
		_ = podmanTest.PodmanExitCleanly("run", "--userns", "auto:size=30", "-dt", "--name", name2, ALPINE, "top")

		gen := podmanTest.PodmanExitCleanly("kube", "generate", name1, name2)
		Expect(gen.OutputToString()).To(ContainSubstring("io.podman.annotations.userns: auto:size=10"))
	})

	It("multiple containers with differing user namespace configuration", func() {
		name1 := "c1"
		name2 := "c2"
		_ = podmanTest.PodmanExitCleanly("run", "--userns", "auto:size=30", "-dt", "--name", name1, ALPINE, "top")
		_ = podmanTest.PodmanExitCleanly("run", "--userns", "auto:size=40", "-dt", "--name", name2, ALPINE, "top")

		gen := podmanTest.Podman([]string{"kube", "generate", name1, name2})
		gen.WaitWithDefaultTimeout()
		Expect(gen).Should(ExitWithError(125, "two or more containers have differing user namespace configuration, cannot place in same Kubernetes pod: invalid argument"))
	})

	It("with containers in pods should fail", func() {
		pod1 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod1", "--name", "top1", CITEST_IMAGE, "top"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1).Should(ExitCleanly())

		pod2 := podmanTest.Podman([]string{"run", "-dt", "--pod", "new:pod2", "--name", "top2", CITEST_IMAGE, "top"})
		pod2.WaitWithDefaultTimeout()
		Expect(pod2).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "top1", "top2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).To(ExitWithError(125, " is associated with pod "))
	})

	It("on a container with dns options", func() {
		top := podmanTest.Podman([]string{"run", "-dt", "--name", "top", "--dns", "8.8.8.8", "--dns-search", "foobar.com", "--dns-option", "color:blue", CITEST_IMAGE, "top"})
		top.WaitWithDefaultTimeout()
		Expect(top).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.DNSConfig.Nameservers).To(ContainElement("8.8.8.8"))
		Expect(pod.Spec.DNSConfig.Searches).To(ContainElement("foobar.com"))
		Expect(pod.Spec.DNSConfig.Options).ToNot(BeEmpty())
		Expect(pod.Spec.DNSConfig.Options[0]).To(HaveField("Name", "color"))
		s := "blue"
		Expect(pod.Spec.DNSConfig.Options[0]).To(HaveField("Value", &s))
	})

	It("multiple container dns servers and options are cumulative", func() {
		top1 := podmanTest.Podman([]string{"run", "-dt", "--name", "top1", "--dns", "8.8.8.8", "--dns-search", "foobar.com", CITEST_IMAGE, "top"})
		top1.WaitWithDefaultTimeout()
		Expect(top1).Should(ExitCleanly())

		top2 := podmanTest.Podman([]string{"run", "-dt", "--name", "top2", "--dns", "8.7.7.7", "--dns-search", "homer.com", CITEST_IMAGE, "top"})
		top2.WaitWithDefaultTimeout()
		Expect(top2).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "top1", "top2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.DNSConfig.Nameservers).To(ContainElement("8.8.8.8"))
		Expect(pod.Spec.DNSConfig.Nameservers).To(ContainElement("8.7.7.7"))
		Expect(pod.Spec.DNSConfig.Searches).To(ContainElement("foobar.com"))
		Expect(pod.Spec.DNSConfig.Searches).To(ContainElement("homer.com"))
	})

	It("on a pod with dns options", func() {
		top := podmanTest.Podman([]string{"run", "--pod", "new:pod1", "-dt", "--name", "top", "--dns", "8.8.8.8", "--dns-search", "foobar.com", "--dns-opt", "color:blue", CITEST_IMAGE, "top"})
		top.WaitWithDefaultTimeout()
		Expect(top).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "pod1"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.DNSConfig.Nameservers).To(ContainElement("8.8.8.8"))
		Expect(pod.Spec.DNSConfig.Searches).To(ContainElement("foobar.com"))
		Expect(pod.Spec.DNSConfig.Options).ToNot(BeEmpty())
		Expect(pod.Spec.DNSConfig.Options[0]).To(HaveField("Name", "color"))
		s := "blue"
		Expect(pod.Spec.DNSConfig.Options[0]).To(HaveField("Value", &s))
	})

	It("- set entrypoint as command", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:testpod", "--entrypoint", "/bin/sleep", CITEST_IMAGE, "10s"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Now make sure that the container's command is set to the
		// entrypoint and it's arguments to "10s".
		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers := pod.Spec.Containers
		Expect(containers).To(HaveLen(1))

		Expect(containers[0]).To(HaveField("Command", []string{"/bin/sleep"}))
		Expect(containers[0]).To(HaveField("Args", []string{"10s"}))
	})

	It("- use command from image unless explicitly set in the podman command", func() {
		session := podmanTest.Podman([]string{"create", "--name", "test", CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "test"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Now make sure that the container's command in the kube yaml is not set to the
		// image command.
		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers := pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0].Command).To(BeEmpty())

		cmd := []string{"echo", "hi"}
		session = podmanTest.Podman(append([]string{"create", "--name", "test1", CITEST_IMAGE}, cmd...))
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube = podmanTest.Podman([]string{"kube", "generate", "test1"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Now make sure that the container's command in the kube yaml is set to the
		// command passed via the cli to podman create.
		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers = pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0]).To(HaveField("Command", cmd))
	})

	It("- use entrypoint from image unless --entrypoint is set", func() {
		// Build an image with an entrypoint.
		containerfile := `FROM quay.io/libpod/alpine:latest
ENTRYPOINT ["sleep"]`

		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0o644)
		Expect(err).ToNot(HaveOccurred())

		image := "generatekube:test"
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", containerfilePath, "-t", image})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", "new:testpod", image, "10s"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Now make sure that the container's command in the kube yaml is NOT set to the
		// entrypoint but the arguments should be set to "10s".
		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers := pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0]).To(HaveField("Args", []string{"10s"}))

		session = podmanTest.Podman([]string{"create", "--pod", "new:testpod-2", "--entrypoint", "echo", image, "hello"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube = podmanTest.Podman([]string{"kube", "generate", "testpod-2"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Now make sure that the container's command in the kube yaml is set to the
		// entrypoint defined by the --entrypoint flag and the arguments should be set to "hello".
		pod = new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers = pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0]).To(HaveField("Command", []string{"echo"}))
		Expect(containers[0]).To(HaveField("Args", []string{"hello"}))
	})

	It("- image has positive integer user set", func() {
		// Build an image with user=1000.
		containerfile := `FROM quay.io/libpod/alpine:latest
USER 1000`

		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0o644)
		Expect(err).ToNot(HaveOccurred())

		image := "generatekube:test"
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", containerfilePath, "-t", image})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", "new:testpod", image, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Now make sure that the container's securityContext has runAsNonRoot=true
		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers := pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		trueBool := true
		Expect(containers[0]).To(HaveField("SecurityContext.RunAsNonRoot", &trueBool))
	})

	It("- --privileged container", func() {
		session := podmanTest.Podman([]string{"create", "--pod", "new:testpod", "--privileged", CITEST_IMAGE, "ls"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Now make sure that the capabilities aren't set.
		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		containers := pod.Spec.Containers
		Expect(containers).To(HaveLen(1))
		Expect(containers[0].SecurityContext.Capabilities).To(BeNil())

		// Now make sure we can also `play` it.
		kubeFile := filepath.Join(podmanTest.TempDir, "kube.yaml")

		kube = podmanTest.Podman([]string{"kube", "generate", "testpod", "-f", kubeFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// Remove the pod so play can recreate it.
		kube = podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		kube = podmanTest.Podman([]string{"kube", "play", kubeFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())
	})

	It("based on user in container", func() {
		// Build an image with an entrypoint.
		containerfile := `FROM quay.io/libpod/alpine:latest
RUN adduser -u 10001 -S test1
USER test1`

		containerfilePath := filepath.Join(podmanTest.TempDir, "Containerfile")
		err = os.WriteFile(containerfilePath, []byte(containerfile), 0o644)
		Expect(err).ToNot(HaveOccurred())

		image := "generatekube:test"
		session := podmanTest.Podman([]string{"build", "--pull-never", "-f", containerfilePath, "-t", image})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", "new:testpod", image, "test1"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "testpod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers[0].SecurityContext.RunAsUser).To(BeNil())
	})

	It("on named volume", func() {
		vol := "simple-named-volume"

		session := podmanTest.Podman([]string{"volume", "create", vol})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", vol})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pvc := new(v1.PersistentVolumeClaim)
		err := yaml.Unmarshal(kube.Out.Contents(), pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc).To(HaveField("Name", vol))
		Expect(pvc.Spec.AccessModes[0]).To(Equal(v1.ReadWriteOnce))
		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))
	})

	It("on named volume with options", func() {
		vol := "complex-named-volume"
		volDevice := define.TypeTmpfs
		volType := define.TypeTmpfs
		volOpts := "nodev,noexec"

		session := podmanTest.Podman([]string{"volume", "create", "--opt", "device=" + volDevice, "--opt", "type=" + volType, "--opt", "o=" + volOpts, vol})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", vol})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pvc := new(v1.PersistentVolumeClaim)
		err := yaml.Unmarshal(kube.Out.Contents(), pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(pvc).To(HaveField("Name", vol))
		Expect(pvc.Spec.AccessModes[0]).To(Equal(v1.ReadWriteOnce))
		Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))

		for k, v := range pvc.Annotations {
			switch k {
			case util.VolumeDeviceAnnotation:
				Expect(v).To(Equal(volDevice))
			case util.VolumeTypeAnnotation:
				Expect(v).To(Equal(volType))
			case util.VolumeMountOptsAnnotation:
				Expect(v).To(Equal(volOpts))
			}
		}
	})

	It("on container with auto update labels", func() {
		top := podmanTest.Podman([]string{"run", "-dt", "--name", "top", "--label", "io.containers.autoupdate=local", CITEST_IMAGE, "top"})
		top.WaitWithDefaultTimeout()
		Expect(top).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "top"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Annotations).To(HaveKeyWithValue("io.containers.autoupdate/top", "local"))
	})

	It("on pod with auto update labels in all containers", func() {
		pod1 := podmanTest.Podman([]string{"pod", "create", "--name", "pod1"})
		pod1.WaitWithDefaultTimeout()
		Expect(pod1).Should(ExitCleanly())

		top1 := podmanTest.Podman([]string{"run", "-dt", "--name", "top1", "--pod", "pod1", "--label", "io.containers.autoupdate=registry", "--label", "io.containers.autoupdate.authfile=/some/authfile.json", CITEST_IMAGE, "top"})
		top1.WaitWithDefaultTimeout()
		Expect(top1).Should(ExitCleanly())

		top2 := podmanTest.Podman([]string{"run", "-dt", "--name", "top2", "--workdir", "/root", "--pod", "pod1", "--label", "io.containers.autoupdate=registry", "--label", "io.containers.autoupdate.authfile=/some/authfile.json", CITEST_IMAGE, "top"})
		top2.WaitWithDefaultTimeout()
		Expect(top2).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "pod1"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Spec.Containers[0]).To(HaveField("WorkingDir", ""))
		Expect(pod.Spec.Containers[1]).To(HaveField("WorkingDir", "/root"))

		for _, ctr := range []string{"top1", "top2"} {
			Expect(pod.Annotations).To(HaveKeyWithValue("io.containers.autoupdate/"+ctr, "registry"))
			Expect(pod.Annotations).To(HaveKeyWithValue("io.containers.autoupdate.authfile/"+ctr, "/some/authfile.json"))
		}
	})

	It("can export env variables correctly", func() {
		// Fixes https://github.com/containers/podman/issues/12647
		// PR https://github.com/containers/podman/pull/12648

		ctrName := "gen-kube-env-ctr"
		podName := "gen-kube-env"
		// In proxy environment, this test needs to the --http-proxy=false option (#16684)
		session1 := podmanTest.Podman([]string{
			"run", "-d", "--http-proxy=false", "--pod", "new:" + podName, "--name", ctrName,
			"-e", "FOO=bar",
			"-e", "HELLO=WORLD",
			CITEST_IMAGE, "top",
		})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.Containers[0].Env).To(HaveLen(2))
	})

	It("omit secret if empty", func() {
		dir := GinkgoT().TempDir()

		podCreate := podmanTest.Podman([]string{"run", "-d", "--pod", "new:" + "noSecretsPod", "--name", "noSecretsCtr", "--volume", dir + ":/foobar", CITEST_IMAGE})
		podCreate.WaitWithDefaultTimeout()
		Expect(podCreate).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "noSecretsPod"})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		Expect(kube.OutputToString()).ShouldNot(ContainSubstring("secret"))

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.Volumes[0].Secret).To(BeNil())
	})

	It("with default ulimits", func() {
		ctrName := "ulimit-ctr"
		session := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, CITEST_IMAGE, "sleep", "1000"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"kube", "generate", ctrName, "-f", outputFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		b, err := os.ReadFile(outputFile)
		Expect(err).ShouldNot(HaveOccurred())
		pod := new(v1.Pod)
		err = yaml.Unmarshal(b, pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(Not(HaveKey(define.UlimitAnnotation)))
	})

	It("with --ulimit set", func() {
		ctrName := "ulimit-ctr"
		ctrNameInKubePod := ctrName + "-pod-" + ctrName
		session1 := podmanTest.Podman([]string{"run", "-d", "--name", ctrName, "--ulimit", "nofile=1231:3123", CITEST_IMAGE, "sleep", "1000"})
		session1.WaitWithDefaultTimeout()
		Expect(session1).Should(ExitCleanly())

		outputFile := filepath.Join(podmanTest.RunRoot, "pod.yaml")
		kube := podmanTest.Podman([]string{"kube", "generate", ctrName, "-f", outputFile})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		b, err := os.ReadFile(outputFile)
		Expect(err).ShouldNot(HaveOccurred())
		pod := new(v1.Pod)
		err = yaml.Unmarshal(b, pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKey(define.UlimitAnnotation))
		Expect(pod.Annotations[define.UlimitAnnotation]).To(ContainSubstring("nofile=1231:3123"))

		rm := podmanTest.Podman([]string{"pod", "rm", "-t", "0", "-f", ctrName})
		rm.WaitWithDefaultTimeout()
		Expect(rm).Should(ExitCleanly())

		play := podmanTest.Podman([]string{"kube", "play", outputFile})
		play.WaitWithDefaultTimeout()
		Expect(play).Should(ExitCleanly())

		inspect := podmanTest.Podman([]string{"inspect", ctrNameInKubePod})
		inspect.WaitWithDefaultTimeout()
		Expect(inspect).Should(ExitCleanly())
		Expect(inspect.OutputToString()).To(ContainSubstring("RLIMIT_NOFILE"))
		Expect(inspect.OutputToString()).To(ContainSubstring("1231"))
		Expect(inspect.OutputToString()).To(ContainSubstring("3123"))
	})

	It("on pod with --type=deployment", func() {
		podName := "test-pod"
		session := podmanTest.Podman([]string{"pod", "create", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", podName, CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"create", "--pod", podName, CITEST_IMAGE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--type", "deployment", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		dep := new(v1.Deployment)
		err := yaml.Unmarshal(kube.Out.Contents(), dep)
		Expect(err).ToNot(HaveOccurred())
		Expect(dep.Name).To(Equal(podName + "-deployment"))
		Expect(dep.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app", podName))
		Expect(dep.Spec.Template.Name).To(Equal(podName))

		numContainers := 0
		for range dep.Spec.Template.Spec.Containers {
			numContainers++
		}
		Expect(numContainers).To(Equal(2))
	})

	It("on ctr with --type=deployment and --replicas=3", func() {
		ctrName := "test-ctr"
		session := podmanTest.Podman([]string{"create", "--name", ctrName, CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--type", "deployment", "--replicas", "3", ctrName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		dep := new(v1.Deployment)
		err := yaml.Unmarshal(kube.Out.Contents(), dep)
		Expect(err).ToNot(HaveOccurred())
		Expect(dep.Name).To(Equal(ctrName + "-pod-deployment"))
		Expect(dep.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app", ctrName+"-pod"))
		Expect(dep.Spec.Template.Name).To(Equal(ctrName + "-pod"))
		Expect(int(*dep.Spec.Replicas)).To(Equal(3))

		numContainers := 0
		for range dep.Spec.Template.Spec.Containers {
			numContainers++
		}
		Expect(numContainers).To(Equal(1))
	})

	It("on ctr with --type=pod and --replicas=3 should fail", func() {
		ctrName := "test-ctr"
		session := podmanTest.Podman([]string{"create", "--name", ctrName, CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--type", "pod", "--replicas", "3", ctrName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "--replicas can only be set when --type is set to deployment"))
	})

	It("on pod with --type=deployment and --restart=no should fail", func() {
		podName := "test-pod"
		session := podmanTest.Podman([]string{"pod", "create", "--restart", "no", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", podName, CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--type", "deployment", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "k8s Deployments can only have restartPolicy set to Always"))
	})

	It("on pod with --type=job", func() {
		podName := "test-pod"
		session := podmanTest.Podman([]string{"pod", "create", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", podName, CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"create", "--pod", podName, CITEST_IMAGE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--type", "job", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		dep := new(v1.Job)
		err := yaml.Unmarshal(kube.Out.Contents(), dep)
		Expect(err).ToNot(HaveOccurred())
		Expect(dep.Name).To(Equal(podName + "-job"))
		Expect(dep.Spec.Template.Name).To(Equal(podName))
		var intone int32 = 1
		Expect(dep.Spec.Parallelism).To(Equal(&intone))
		Expect(dep.Spec.Completions).To(Equal(&intone))

		numContainers := 0
		for range dep.Spec.Template.Spec.Containers {
			numContainers++
		}
		Expect(numContainers).To(Equal(2))
	})

	It("on pod with --type=job and --restart=always should fail", func() {
		podName := "test-pod"
		session := podmanTest.Podman([]string{"pod", "create", "--restart", "always", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", podName, CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--type", "job", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "k8s Jobs can not have restartPolicy set to Always; only Never and OnFailure policies allowed"))
	})

	It("on pod with invalid name", func() {
		podName := "test_pod"
		session := podmanTest.Podman([]string{"pod", "create", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", podName, "--restart", "no", CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		// The pod name should no longer have _ and there should be no hostname in the generated yaml
		Expect(pod.Name).To(Equal("testpod"))
		Expect(pod.Spec.Hostname).To(Equal(""))
	})

	It("--podman-only on container with --volumes-from", func() {
		ctr1 := "ctr1"
		ctr2 := "ctr2"
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")

		err := os.MkdirAll(vol1, 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"create", "--name", ctr1, "-v", vol1, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--volumes-from", ctr1, "--name", ctr2, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--podman-only", ctr2})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.VolumesFromAnnotation+"/"+ctr2, ctr1))
	})

	It("pod volumes-from annotation with semicolon as field separator", func() {
		// Assert that volumes-from annotation for multiple source
		// containers along with their mount options are getting
		// generated with semicolon as the field separator.

		srcctr1, srcctr2, tgtctr := "srcctr1", "srcctr2", "tgtctr"
		frmopt1, frmopt2 := srcctr1+":ro", srcctr2+":ro"
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		vol2 := filepath.Join(podmanTest.TempDir, "vol-test2")

		err1 := os.MkdirAll(vol1, 0o755)
		Expect(err1).ToNot(HaveOccurred())

		err2 := os.MkdirAll(vol2, 0o755)
		Expect(err2).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"create", "--name", srcctr1, "-v", vol1, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--name", srcctr2, "-v", vol2, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--volumes-from", frmopt1, "--volumes-from", frmopt2, "--name", tgtctr, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--podman-only", tgtctr})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err3 := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err3).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.VolumesFromAnnotation+"/"+tgtctr, frmopt1+";"+frmopt2))
	})

	It("--podman-only on container with --rm", func() {
		ctr := "ctr"

		session := podmanTest.Podman([]string{"create", "--rm", "--name", ctr, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--podman-only", ctr})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.InspectAnnotationAutoremove+"/"+ctr, define.InspectResponseTrue))
	})

	It("--podman-only on container with --privileged", func() {
		ctr := "ctr"

		session := podmanTest.Podman([]string{"create", "--privileged", "--name", ctr, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--podman-only", ctr})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.InspectAnnotationPrivileged+"/"+ctr, define.InspectResponseTrue))
	})

	It("--podman-only on container with --init", func() {
		ctr := "ctr"

		session := podmanTest.Podman([]string{"create", "--init", "--name", ctr, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--podman-only", ctr})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.InspectAnnotationInit+"/"+ctr, define.InspectResponseTrue))
	})

	It("--podman-only on container with --cidfile", func() {
		ctr := "ctr"
		cidFile := filepath.Join(podmanTest.TempDir, RandomString(10)+".txt")

		session := podmanTest.Podman([]string{"create", "--cidfile", cidFile, "--name", ctr, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--podman-only", ctr})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.InspectAnnotationCIDFile+"/"+ctr, cidFile))
	})

	It("--podman-only on container with --security-opt seccomp=unconfined", func() {
		ctr := "ctr"

		session := podmanTest.Podman([]string{"create", "--security-opt", "seccomp=unconfined", "--name", ctr, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--podman-only", ctr})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.InspectAnnotationSeccomp+"/"+ctr, "unconfined"))
	})

	It("--podman-only on container with --security-opt apparmor=unconfined", func() {
		ctr := "ctr"

		session := podmanTest.Podman([]string{"create", "--security-opt", "apparmor=unconfined", "--name", ctr, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--podman-only", ctr})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.InspectAnnotationApparmor+"/"+ctr, "unconfined"))
	})

	It("--podman-only on container with --security-opt label=level:s0", func() {
		ctr := "ctr"

		session := podmanTest.Podman([]string{"create", "--security-opt", "label=level:s0", "--name", ctr, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--podman-only", ctr})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.InspectAnnotationLabel+"/"+ctr, "level:s0"))
	})

	It("--podman-only on container with --publish-all", func() {
		podmanTest.AddImageToRWStore(CITEST_IMAGE)
		dockerfile := fmt.Sprintf(`FROM %s
EXPOSE 2002
EXPOSE 2001-2003
EXPOSE 2004-2005/tcp`, CITEST_IMAGE)
		imageName := "testimg"
		podmanTest.BuildImage(dockerfile, imageName, "false")

		// Verify that the buildah is just passing through the EXPOSE keys
		inspect := podmanTest.Podman([]string{"inspect", imageName})
		inspect.WaitWithDefaultTimeout()
		image := inspect.InspectImageJSON()
		Expect(image).To(HaveLen(1))
		Expect(image[0].Config.ExposedPorts).To(HaveLen(3))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2002/tcp"))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2001-2003/tcp"))
		Expect(image[0].Config.ExposedPorts).To(HaveKey("2004-2005/tcp"))

		ctr := "ctr"
		session := podmanTest.Podman([]string{"create", "--publish-all", "--name", ctr, imageName, "true"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--podman-only", ctr})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err = yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.InspectAnnotationPublishAll+"/"+ctr, define.InspectResponseTrue))
	})

	It("on pod with --infra-name set", func() {
		infraName := "infra-ctr"
		podName := "test-pod"
		podSession := podmanTest.Podman([]string{"pod", "create", "--infra-name", infraName, podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"create", "--pod", podName, CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveKeyWithValue(define.InfraNameAnnotation, infraName))
	})

	It("on pod without --infra-name set", func() {
		podName := "test-pod"
		podSession := podmanTest.Podman([]string{"pod", "create", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"create", "--pod", podName, CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// There should be no infra name annotation set if the --infra-name flag wasn't set during pod creation
		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Annotations).To(HaveLen(1))
	})

	It("on pod with --stop-timeout set for ctr", func() {
		podName := "test-pod"
		podSession := podmanTest.Podman([]string{"pod", "create", podName})
		podSession.WaitWithDefaultTimeout()
		Expect(podSession).Should(ExitCleanly())

		session := podmanTest.Podman([]string{"create", "--pod", podName, "--stop-timeout", "20", CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		// TerminationGracePeriodSeconds should be set to 20
		pod := new(v1.Pod)
		err := yaml.Unmarshal(kube.Out.Contents(), pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(int(*pod.Spec.TerminationGracePeriodSeconds)).To(Equal(20))
	})

	It("on pod with --type=daemonset", func() {
		podName := "test-pod"
		session := podmanTest.Podman([]string{"pod", "create", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", podName, CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		session = podmanTest.Podman([]string{"create", "--pod", podName, CITEST_IMAGE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--type", "daemonset", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitCleanly())

		dep := new(v1.DaemonSet)
		err := yaml.Unmarshal(kube.Out.Contents(), dep)
		Expect(err).ToNot(HaveOccurred())
		Expect(dep.Name).To(Equal(podName + "-daemonset"))
		Expect(dep.Spec.Selector.MatchLabels).To(HaveKeyWithValue("app", podName))
		Expect(dep.Spec.Template.Name).To(Equal(podName))
		Expect(dep.Spec.Template.Spec.Containers).To(HaveLen(2))
	})

	It("on ctr with --type=daemonset and --replicas=3 should fail", func() {
		ctrName := "test-ctr"
		session := podmanTest.Podman([]string{"create", "--name", ctrName, CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--type", "daemonset", "--replicas", "3", ctrName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "--replicas can only be set when --type is set to deployment"))
	})

	It("on pod with --type=daemonset and --restart=no should fail", func() {
		podName := "test-pod"
		session := podmanTest.Podman([]string{"pod", "create", "--restart", "no", podName})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--pod", podName, CITEST_IMAGE, "top"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		kube := podmanTest.Podman([]string{"kube", "generate", "--type", "daemonset", podName})
		kube.WaitWithDefaultTimeout()
		Expect(kube).Should(ExitWithError(125, "k8s DaemonSets can only have restartPolicy set to Always"))
	})
})
