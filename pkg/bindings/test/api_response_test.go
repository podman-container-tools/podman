package bindings_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
	"go.podman.io/podman/v6/pkg/domain/entities/types"
)

var _ = Describe("APIResponse Process method", func() {
	createMockResponse := func(jsonResponse []byte, statusCode int) *bindings.APIResponse {
		response := &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(bytes.NewBuffer(jsonResponse)),
			Header:     make(http.Header),
		}
		response.Header.Set("Content-Type", "application/json")
		return &bindings.APIResponse{Response: response}
	}

	It("unmarshal the response with ContainerPruneReport with error", func() {
		errorStr := `replacing mount point "/tmp/CI_7Qsy/podman-e2e/b33ae357af1/merged": directory not empty`
		responseReport := types.SystemPruneReport{
			ContainerPruneReports: []*reports.PruneReport{
				{Id: "aec04392e9b2fe7c4a3", Size: 8219},
				{
					Id:   "3d8a8789524a0c44a61",
					Size: 7238,
					Err:  errors.New(errorStr),
				},
				{Id: "e9ef46f3a3cd43c929b", Size: 0},
			},
			PodPruneReport:      nil,
			ImagePruneReports:   nil,
			NetworkPruneReports: nil,
			VolumePruneReports:  nil,
			ReclaimedSpace:      15457,
		}

		jsonResponse, err := json.Marshal(responseReport)
		Expect(err).ToNot(HaveOccurred())
		apiResponse := createMockResponse(jsonResponse, 200)

		var report types.SystemPruneReport
		err = apiResponse.Process(&report)
		Expect(err).ToNot(HaveOccurred())

		Expect(report.ContainerPruneReports).To(HaveLen(3))
		Expect(report.ReclaimedSpace).To(Equal(uint64(15457)))

		first := report.ContainerPruneReports[0]
		Expect(first.Id).To(Equal("aec04392e9b2fe7c4a3"))
		Expect(first.Size).To(Equal(uint64(8219)))
		Expect(first.Err).ToNot(HaveOccurred())

		second := report.ContainerPruneReports[1]
		Expect(second.Id).To(Equal("3d8a8789524a0c44a61"))
		Expect(second.Size).To(Equal(uint64(7238)))
		Expect(second.Err).To(HaveOccurred())
		Expect(second.Err.Error()).To(Equal(errorStr))

		third := report.ContainerPruneReports[2]
		Expect(third.Id).To(Equal("e9ef46f3a3cd43c929b"))
		Expect(third.Size).To(Equal(uint64(0)))
		Expect(third.Err).ToNot(HaveOccurred())
	})
})
