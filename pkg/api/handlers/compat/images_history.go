//go:build !remote && (linux || freebsd)

package compat

import (
	"fmt"
	"net/http"

	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
)

func HistoryImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}

	newImage, _, err := runtime.LibimageRuntime().LookupImage(possiblyNormalizedName, nil)
	if err != nil {
		utils.ImageNotFound(w, possiblyNormalizedName, fmt.Errorf("failed to find image %s: %w", possiblyNormalizedName, err))
		return
	}
	history, err := newImage.History(r.Context())
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	allHistory := make([]handlers.HistoryResponse, 0, len(history))
	for _, h := range history {
		l := handlers.HistoryResponse{
			Created:   h.Created.Unix(),
			CreatedBy: h.CreatedBy,
			Tags:      h.Tags,
			Size:      h.Size,
			Comment:   h.Comment,
		}
		if utils.IsLibpodRequest(r) {
			l.ID = h.ID
		} else {
			l.ID = "sha256:" + h.ID
		}
		allHistory = append(allHistory, l)
	}
	utils.WriteResponse(w, http.StatusOK, allHistory)
}
