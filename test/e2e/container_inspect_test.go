//go:build linux || freebsd

package integration

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/annotations"
	. "go.podman.io/podman/v6/test/utils"
)

var _ = Describe("Podman container inspect", func() {
	It("podman inspect a container for the container manager annotation", func() {
		const testContainer = "container-inspect-test-1"
		setup := podmanTest.RunTopContainer(testContainer)
		setup.WaitWithDefaultTimeout()
		Expect(setup).Should(ExitCleanly())

		data := podmanTest.InspectContainer(testContainer)
		Expect(data[0].Config.Annotations[annotations.ContainerManager]).
			To(Equal(annotations.ContainerManagerLibpod))
	})

	It("podman inspect shows exposed ports", func() {
		name := "testcon"
		session := podmanTest.Podman([]string{"run", "-d", "--stop-timeout", "0", "--expose", "8787/udp", "--expose", "99/sctp", "--name", name, ALPINE, "sleep", "100"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		data := podmanTest.InspectContainer(name)

		Expect(data).To(HaveLen(1))
		Expect(data[0].NetworkSettings.Ports).
			To(Equal(map[string][]define.InspectHostPort{"8787/udp": nil, "99/sctp": nil}))
		Expect(data[0].Config.ExposedPorts).
			To(Equal(map[string]struct{}{"8787/udp": {}, "99/sctp": {}}))

		session = podmanTest.Podman([]string{"ps", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("99/sctp, 8787/udp"))
	})

	It("podman inspect shows exposed ports on image", func() {
		name := "testcon"
		session := podmanTest.Podman([]string{"run", "-d", "--expose", "8989", "--name", name, NGINX_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		data := podmanTest.InspectContainer(name)
		Expect(data).To(HaveLen(1))
		Expect(data[0].NetworkSettings.Ports).
			To(Equal(map[string][]define.InspectHostPort{"80/tcp": nil, "8989/tcp": nil}))

		session = podmanTest.Podman([]string{"ps", "--format", "{{.Ports}}"})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())
		Expect(session.OutputToString()).To(Equal("80/tcp, 8989/tcp"))
	})

	It("podman inspect exposed ports includes published ports", func() {
		hostPort := GetPort()
		c1 := "ctr1"
		c1s := podmanTest.Podman([]string{"run", "-d", "--expose", "22/tcp", "-p", fmt.Sprintf("%d:80/tcp", hostPort), "--name", c1, ALPINE, "top"})
		c1s.WaitWithDefaultTimeout()
		Expect(c1s).Should(ExitCleanly())

		c2 := "ctr2"
		c2s := podmanTest.Podman([]string{"run", "-d", "--net", fmt.Sprintf("container:%s", c1), "--name", c2, ALPINE, "top"})
		c2s.WaitWithDefaultTimeout()
		Expect(c2s).Should(ExitCleanly())

		data1 := podmanTest.InspectContainer(c1)
		Expect(data1).To(HaveLen(1))
		Expect(data1[0].Config.ExposedPorts).
			To(Equal(map[string]struct{}{"22/tcp": {}, "80/tcp": {}}))

		data2 := podmanTest.InspectContainer(c2)
		Expect(data2).To(HaveLen(1))
		Expect(data2[0].Config.ExposedPorts).To(BeNil())
	})

	It("podman inspect shows volumes-from with mount options", func() {
		ctr1 := "volfctr"
		ctr2 := "voltctr"
		vol1 := filepath.Join(podmanTest.TempDir, "vol-test1")
		volsctr := ctr1 + ":z,ro"

		err := os.MkdirAll(vol1, 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"create", "--name", ctr1, "-v", vol1, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		session = podmanTest.Podman([]string{"create", "--volumes-from", volsctr, "--name", ctr2, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		data := podmanTest.InspectContainer(ctr2)
		Expect(data).To(HaveLen(1))
		Expect(data[0].HostConfig.VolumesFrom).To(Equal([]string{volsctr}))
		Expect(data[0].Config.Annotations[define.VolumesFromAnnotation]).To(Equal(volsctr))
	})

	It("podman inspect hides secrets mounted to env", func() {
		secretName := "mysecret"

		secretFilePath := filepath.Join(podmanTest.TempDir, "secret")
		err := os.WriteFile(secretFilePath, []byte("mySecretValue"), 0o755)
		Expect(err).ToNot(HaveOccurred())

		session := podmanTest.Podman([]string{"secret", "create", secretName, secretFilePath})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		name := "testcon"
		session = podmanTest.Podman([]string{"run", "--secret", fmt.Sprintf("%s,type=env", secretName), "--name", name, CITEST_IMAGE})
		session.WaitWithDefaultTimeout()
		Expect(session).Should(ExitCleanly())

		data := podmanTest.InspectContainer(name)
		Expect(data).To(HaveLen(1))
		Expect(data[0].Config.Env).To(ContainElement(Equal(secretName + "=*******")))
	})
})
