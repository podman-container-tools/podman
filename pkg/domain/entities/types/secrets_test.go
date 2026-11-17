package types

import (
	"encoding/json"
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTypes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Types Suite")
}

var _ = Describe("SecretRmReport JSON", func() {
	Context("when marshaling and unmarshaling", func() {
		tests := []struct {
			name   string
			report *SecretRmReport
		}{
			{
				name: "report with error",
				report: &SecretRmReport{
					ID:  "newsecret",
					Err: errors.New("deleting secret newsecret: is a directory"),
				},
			},
			{
				name: "report without error",
				report: &SecretRmReport{
					ID:  "newsecret",
					Err: nil,
				},
			},
			{
				name: "empty report",
				report: &SecretRmReport{
					ID:  "",
					Err: nil,
				},
			},
			{
				name: "report with special characters in error",
				report: &SecretRmReport{
					ID:  "secret-special",
					Err: errors.New(`error with "quotes" and \backslashes\ and newlines`),
				},
			},
		}

		for _, tt := range tests {
			It("should handle "+tt.name, func() {
				jsonData, err := json.Marshal(tt.report)
				Expect(err).ToNot(HaveOccurred())

				var unmarshalled SecretRmReport
				err = json.Unmarshal(jsonData, &unmarshalled)
				Expect(err).ToNot(HaveOccurred())

				Expect(unmarshalled.ID).To(Equal(tt.report.ID))

				if tt.report.Err == nil {
					Expect(unmarshalled.Err).ToNot(HaveOccurred())
				} else {
					Expect(unmarshalled.Err).To(HaveOccurred())
					Expect(unmarshalled.Err.Error()).To(Equal(tt.report.Err.Error()))
				}
			})
		}
	})
})
