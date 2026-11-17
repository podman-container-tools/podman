package bindings_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/system"
)

var _ = Describe("Podman connection", func() {
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
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("request on cancelled context results in error", func() {
		ctx, cancel := context.WithCancel(bt.conn)
		cancel()
		_, err := system.Version(ctx, nil)
		Expect(err).To(MatchError(ctx.Err()))
	})

	It("cancel request in flight reports cancelled context", func() {
		name := "top"
		_, err := bt.RunTopContainer(&name, nil)
		Expect(err).ToNot(HaveOccurred())

		errChan := make(chan error)
		ctx, cancel := context.WithCancel(bt.conn)

		go func() {
			defer close(errChan)
			_, err := containers.Wait(ctx, name, nil)
			errChan <- err
		}()

		// Wait for the goroutine to fire the request
		time.Sleep(1 * time.Second)

		cancel()

		select {
		case err, ok := <-errChan:
			Expect(ok).To(BeTrue())
			Expect(err).To(MatchError(ctx.Err()))
		case <-time.NewTimer(1 * time.Second).C:
			Fail("cancelled request did not return in less than 1 second")
		}
	})
})
