package bindings_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.podman.io/podman/v6/pkg/bindings/containers"
)

var _ = Describe("Podman API Bindings", func() {
	boxedTrue, boxedFalse := new(bool), new(bool)
	*boxedTrue = true
	*boxedFalse = false

	It("verify simple setters", func() {
		boxedString := new(string)
		*boxedString = "Test"

		actual := new(containers.AttachOptions).
			WithDetachKeys("Test").WithLogs(true).WithStream(false)

		Expect(*actual).To(MatchAllFields(Fields{
			"DetachKeys": Equal(boxedString),
			"Logs":       Equal(boxedTrue),
			"Stream":     Equal(boxedFalse),
		}))

		Expect(actual.GetDetachKeys()).To(Equal("Test"))
		Expect(actual.GetLogs()).To(BeTrue())
		Expect(actual.GetStream()).To(BeFalse())
	})

	It("verify composite setters", func() {
		boxedInt := new(int)
		*boxedInt = 50

		actual := new(containers.ListOptions).
			WithFilters(map[string][]string{"Test": {"Test Filter"}}).
			WithLast(50)

		Expect(*actual).To(MatchAllFields(Fields{
			"All":       BeNil(),
			"External":  BeNil(),
			"Filters":   HaveKeyWithValue("Test", []string{"Test Filter"}),
			"Last":      Equal(boxedInt),
			"Namespace": BeNil(),
			"Size":      BeNil(),
			"Sync":      BeNil(),
		}))
	})
})
