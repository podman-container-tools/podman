package reports

import (
	"encoding/json"
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestReports(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reports Suite")
}

var _ = Describe("PruneReport JSON", func() {
	Context("when marshaling and unmarshaling", func() {
		tests := []struct {
			name    string
			report  *PruneReport
			wantErr bool
		}{
			{
				name: "report with error",
				report: &PruneReport{
					Id:   "test-container-id",
					Err:  errors.New("test error message"),
					Size: 1024,
				},
			},
			{
				name: "report without error",
				report: &PruneReport{
					Id:   "test-container-id",
					Err:  nil,
					Size: 2048,
				},
			},
			{
				name: "empty report",
				report: &PruneReport{
					Id:   "",
					Err:  nil,
					Size: 0,
				},
			},
			{
				name: "report with complex error message from failing test case",
				report: &PruneReport{
					Id:   "3d8a8789524a0c44a61baa49ceedda7be069b0b3d01255b24013d2fb82168c7e",
					Err:  errors.New(`replacing mount point "/tmp/CI_7Qsy/podman-e2e-213135586/subtest-1767990215/p/root/overlay/d9f554276b923c07bf708858b5f35774f9d2924fa4094b1583e56b33ae357af1/merged": directory not empty`),
					Size: 7238,
				},
			},
			{
				name: "report with special characters in error",
				report: &PruneReport{
					Id:   "container-special",
					Err:  errors.New(`error with "quotes" and \backslashes\ and newlines`),
					Size: 512,
				},
			},
		}

		for _, tt := range tests {
			It("should handle "+tt.name, func() {
				jsonData, err := json.Marshal(tt.report)
				Expect(err).ToNot(HaveOccurred())

				var unmarshalled PruneReport
				err = json.Unmarshal(jsonData, &unmarshalled)
				Expect(err).ToNot(HaveOccurred())

				Expect(unmarshalled.Id).To(Equal(tt.report.Id))
				Expect(unmarshalled.Size).To(Equal(tt.report.Size))

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
