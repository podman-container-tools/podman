//go:build linux

package bindings_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/bindings/pods"
	"go.podman.io/podman/v6/pkg/bindings/system"
)

var _ = Describe("Verify Podman resources", func() {
	var (
		bt *bindingTest
		s  *Session
	)

	BeforeEach(func() {
		bt = newBindingTest()
		s = bt.startAPIService()
		err := bt.NewConnection()
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("no leaked connections", func() {
		conn, err := bindings.NewConnection(context.Background(), bt.sock)
		Expect(err).ShouldNot(HaveOccurred())

		// Record details on open file descriptors before using API
		buffer := lsof()

		// Record open fd from /proc
		start, err := readProc()
		Expect(err).ShouldNot(HaveOccurred())

		// Run some operations
		_, err = system.Info(conn, nil)
		Expect(err).ShouldNot(HaveOccurred())
		_, err = images.List(conn, nil)
		Expect(err).ShouldNot(HaveOccurred())
		_, err = containers.List(conn, nil)
		Expect(err).ShouldNot(HaveOccurred())
		_, err = pods.List(conn, nil)
		Expect(err).ShouldNot(HaveOccurred())

		podman, _ := bindings.GetClient(conn)
		podman.Client.CloseIdleConnections()

		// Record open fd from /proc
		finished, err := readProc()
		Expect(err).ShouldNot(HaveOccurred())

		// CloseIdleConnections drops idle keep-alive connections, including the
		// connection opened before the first snapshot. A closed fd is not
		// a leak. A descriptor that is still open afterwards and was not
		// open before is.
		before := make(map[string]struct{}, len(start))
		for _, fd := range start {
			before[fd] = struct{}{}
		}
		var leaked []string
		for _, fd := range finished {
			if _, ok := before[fd]; !ok {
				leaked = append(leaked, fd)
			}
		}
		if len(leaked) > 0 {
			fmt.Fprintf(GinkgoWriter, "Open FDs:\nlsof Before:\n%s\n", buffer)

			// Record details on open file descriptors after using API
			buffer := lsof()
			fmt.Fprintf(GinkgoWriter, "lsof After:\n%s\n", buffer)

			// We know test has failed. Easier to let ginkgo format output.
			Expect(leaked).Should(BeEmpty())
		}
	})
})

func lsof() string {
	lsof := exec.Command("lsof", "+E", "-p", strconv.Itoa(os.Getpid()))
	buffer, err := lsof.Output()
	Expect(err).ShouldNot(HaveOccurred())
	return string(buffer)
}

func readProc() ([]string, error) {
	syscall.Sync()

	names := make([]string, 0)
	err := filepath.WalkDir(fmt.Sprintf("/proc/%d/fd", os.Getpid()),
		func(path string, d fs.DirEntry, err error) error {
			name := path + " -> "

			switch {
			case d.IsDir():
				return nil
			case err != nil:
				name += err.Error()
			case d.Type()&fs.ModeSymlink != 0:
				n, err := os.Readlink(path)
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
				// An fd can close between readdir and readlink.
				if n == "" {
					return nil
				}
				name += n
			}
			names = append(names, name)
			return nil
		})
	return names, err
}
