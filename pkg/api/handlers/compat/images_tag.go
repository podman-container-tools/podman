//go:build !remote && (linux || freebsd)

package compat

import (
	"errors"
	"fmt"
	"net/http"

	"go.podman.io/common/libimage"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
)

func TagImage(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	name := utils.GetName(r)
	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}

	// Allow tagging manifest list instead of resolving instances from manifest
	lookupOptions := &libimage.LookupImageOptions{ManifestList: true}
	newImage, _, err := runtime.LibimageRuntime().LookupImage(possiblyNormalizedName, lookupOptions)
	if err != nil {
		utils.ImageNotFound(w, name, fmt.Errorf("failed to find image %s: %w", name, err))
		return
	}

	tag := "latest"
	if len(r.Form.Get("tag")) > 0 {
		tag = r.Form.Get("tag")
	}
	if len(r.Form.Get("repo")) < 1 {
		utils.Error(w, http.StatusBadRequest, errors.New("repo parameter is required to tag an image"))
		return
	}
	repo := r.Form.Get("repo")
	tagName := fmt.Sprintf("%s:%s", repo, tag)

	possiblyNormalizedTag, err := utils.NormalizeToDockerHub(r, tagName)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}

	if err := newImage.Tag(possiblyNormalizedTag); err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusCreated, "")
}
