package bindings_test

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/bindings/kube"
	"go.podman.io/podman/v6/pkg/bindings/manifests"
)

var _ = Describe("Binding types", func() {
	It("serialize image pull options", func() {
		var writer bytes.Buffer
		opts := new(images.PullOptions).WithOS("foo").WithProgressWriter(&writer).WithSkipTLSVerify(true).
			WithAuthfile("/tmp/auth.json").WithUsername("user").WithPassword("pass")
		params, err := opts.ToParams()
		Expect(err).ToNot(HaveOccurred())
		Expect(params.Get("os")).To(Equal("foo"))
		Expect(params.Has("progresswriter")).To(BeFalse())
		Expect(params.Has("skiptlsverify")).To(BeFalse())
		Expect(params.Has("authfile")).To(BeFalse())
		Expect(params.Has("username")).To(BeFalse())
		Expect(params.Has("password")).To(BeFalse())
	})

	It("serialize image push options", func() {
		var writer bytes.Buffer
		opts := new(images.PushOptions).WithAll(true).WithProgressWriter(&writer).WithSkipTLSVerify(true).
			WithAuthfile("/tmp/auth.json").WithUsername("user").WithPassword("pass")
		params, err := opts.ToParams()
		Expect(err).ToNot(HaveOccurred())
		Expect(params.Get("all")).To(Equal("true"))
		Expect(params.Has("progresswriter")).To(BeFalse())
		Expect(params.Has("skiptlsverify")).To(BeFalse())
		Expect(params.Has("authfile")).To(BeFalse())
		Expect(params.Has("username")).To(BeFalse())
		Expect(params.Has("password")).To(BeFalse())
	})

	It("serialize image search options", func() {
		opts := new(images.SearchOptions).WithLimit(123).WithSkipTLSVerify(true).
			WithAuthfile("/tmp/auth.json").WithUsername("user").WithPassword("pass")
		params, err := opts.ToParams()
		Expect(err).ToNot(HaveOccurred())
		Expect(params.Get("limit")).To(Equal("123"))
		Expect(params.Has("skiptlsverify")).To(BeFalse())
		Expect(params.Has("authfile")).To(BeFalse())
		Expect(params.Has("username")).To(BeFalse())
		Expect(params.Has("password")).To(BeFalse())
	})

	It("serialize image scp options", func() {
		// The names here are what the libpod ImageScp handler decodes, so a
		// rename on either side silently drops compression on the remote client.
		format := "zstd"
		level := 3
		opts := &images.ScpOptions{CompressionFormat: &format, CompressionLevel: &level}
		params, err := opts.ToParams()
		Expect(err).ToNot(HaveOccurred())
		Expect(params.Get("compressionformat")).To(Equal("zstd"))
		Expect(params.Get("compressionlevel")).To(Equal("3"))
	})

	It("serialize image scp options without compression", func() {
		// An unset level must not be sent as 0: the server would reject it.
		opts := &images.ScpOptions{}
		params, err := opts.ToParams()
		Expect(err).ToNot(HaveOccurred())
		Expect(params.Has("compressionformat")).To(BeFalse())
		Expect(params.Has("compressionlevel")).To(BeFalse())
	})

	It("serialize manifest modify options", func() {
		opts := new(manifests.ModifyOptions).WithOS("foo").WithSkipTLSVerify(true).
			WithAuthfile("/tmp/auth.json").WithUsername("user").WithPassword("pass")
		params, err := opts.ToParams()
		Expect(err).ToNot(HaveOccurred())
		Expect(params.Get("os")).To(Equal("foo"))
		Expect(params.Has("skiptlsverify")).To(BeFalse())
		Expect(params.Has("authfile")).To(BeFalse())
		Expect(params.Has("username")).To(BeFalse())
		Expect(params.Has("password")).To(BeFalse())
	})

	It("serialize manifest add options", func() {
		opts := new(manifests.AddOptions).WithAll(true).WithOS("foo").WithSkipTLSVerify(true).
			WithAuthfile("/tmp/auth.json").WithUsername("user").WithPassword("pass")
		params, err := opts.ToParams()
		Expect(err).ToNot(HaveOccurred())
		Expect(params.Get("all")).To(Equal("true"))
		Expect(params.Get("os")).To(Equal("foo"))
		Expect(params.Has("skiptlsverify")).To(BeFalse())
		Expect(params.Has("authfile")).To(BeFalse())
		Expect(params.Has("username")).To(BeFalse())
		Expect(params.Has("password")).To(BeFalse())
	})

	It("serialize kube play options", func() {
		opts := new(kube.PlayOptions).WithQuiet(true).WithSkipTLSVerify(true).
			WithAuthfile("/tmp/auth.json").WithUsername("user").WithPassword("pass")
		params, err := opts.ToParams()
		Expect(err).ToNot(HaveOccurred())
		Expect(params.Get("quiet")).To(Equal("true"))
		Expect(params.Has("skiptlsverify")).To(BeFalse())
		Expect(params.Has("authfile")).To(BeFalse())
		Expect(params.Has("username")).To(BeFalse())
		Expect(params.Has("password")).To(BeFalse())
	})
	It("serialize manifest inspect options", func() {
		opts := new(manifests.InspectOptions).WithAuthfile("/tmp/auth.json").WithSkipTLSVerify(true)
		params, err := opts.ToParams()
		Expect(err).ToNot(HaveOccurred())
		Expect(params.Has("authfile")).To(BeFalse())
		Expect(params.Has("skiptlsverify")).To(BeFalse())
	})
})
