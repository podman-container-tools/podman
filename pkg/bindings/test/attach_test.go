package bindings_test

import (
	"bytes"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/specgen"
)

var _ = Describe("Podman containers attach", func() {
	var (
		bt *bindingTest
		s  *gexec.Session
	)

	BeforeEach(func() {
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("can run top in container", func() {
		name := "TopAttachTest"
		id, err := bt.RunTopContainer(&name, nil)
		Expect(err).ShouldNot(HaveOccurred())

		tickTock := time.NewTimer(2 * time.Second)
		go func() {
			<-tickTock.C
			timeout := uint(5)
			err := containers.Stop(bt.conn, id, new(containers.StopOptions).WithTimeout(timeout))
			if err != nil {
				_, writeErr := GinkgoWriter.Write([]byte(err.Error()))
				Expect(writeErr).ShouldNot(HaveOccurred())
			}
		}()

		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		go func() {
			defer GinkgoRecover()
			options := new(containers.AttachOptions).WithLogs(true).WithStream(true)
			err := containers.Attach(bt.conn, id, nil, stdout, stderr, nil, options)
			Expect(err).ShouldNot(HaveOccurred())
		}()

		time.Sleep(5 * time.Second)

		// First character/First line of top output
		Expect(stdout.String()).Should(ContainSubstring("Mem: "))
	})

	It("can echo data via cat in container", func() {
		s := specgen.NewSpecGenerator(alpine.name, false)
		s.Name = "CatAttachTest"
		localTrue := true
		s.Terminal = &localTrue
		s.Command = []string{"/bin/cat"}
		ctnr, err := containers.CreateWithSpec(bt.conn, s, nil)
		Expect(err).ShouldNot(HaveOccurred())

		err = containers.Start(bt.conn, ctnr.ID, nil)
		Expect(err).ShouldNot(HaveOccurred())

		wait := define.ContainerStateRunning
		_, err = containers.Wait(bt.conn, ctnr.ID, new(containers.WaitOptions).WithCondition([]define.ContainerStatus{wait}))
		Expect(err).ShouldNot(HaveOccurred())

		tickTock := time.NewTimer(2 * time.Second)
		go func() {
			<-tickTock.C
			err := containers.Stop(bt.conn, ctnr.ID, new(containers.StopOptions).WithTimeout(uint(5)))
			if err != nil {
				fmt.Fprint(GinkgoWriter, err.Error())
			}
		}()

		msg := "Hello, World"
		stdin := &bytes.Buffer{}
		stdin.WriteString(msg + "\n")

		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		go func() {
			defer GinkgoRecover()
			options := new(containers.AttachOptions).WithStream(true)
			err := containers.Attach(bt.conn, ctnr.ID, stdin, stdout, stderr, nil, options)
			Expect(err).ShouldNot(HaveOccurred())
		}()

		time.Sleep(5 * time.Second)
		// Tty==true so we get echo'ed stdin + expected output
		Expect(stdout.String()).Should(Equal(fmt.Sprintf("%[1]s\r\n%[1]s\r\n", msg)))
		Expect(stderr.String()).Should(BeEmpty())
	})
})
