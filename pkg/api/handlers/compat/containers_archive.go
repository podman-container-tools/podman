//go:build !remote && (linux || freebsd)

package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/copy"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
)

func Archive(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	switch r.Method {
	case http.MethodPut:
		handlePut(w, r, decoder, runtime)
	case http.MethodHead, http.MethodGet:
		handleHeadAndGet(w, r, decoder, runtime)
	default:
		utils.Error(w, http.StatusNotImplemented, fmt.Errorf("unsupported method: %v", r.Method))
	}
}

func handleHeadAndGet(w http.ResponseWriter, r *http.Request, decoder *schema.Decoder, runtime *libpod.Runtime) {
	query := struct {
		Path string `schema:"path"`
	}{}

	err := decoder.Decode(&query, r.URL.Query())
	if err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("couldn't decode the query: %w", err))
		return
	}

	if query.Path == "" {
		utils.Error(w, http.StatusBadRequest, errors.New("missing `path` parameter"))
		return
	}

	containerName := utils.GetName(r)
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	statReport, err := containerEngine.ContainerStat(r.Context(), containerName, query.Path)

	// NOTE
	// The statReport may actually be set even in case of an error.  That's
	// the case when we're looking at a symlink pointing to nirvana.  In
	// such cases, we really need the FileInfo but we also need the error.
	if statReport != nil {
		statHeader, err := copy.EncodeFileInfo(&statReport.FileInfo)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Add(copy.XDockerContainerPathStatHeader, statHeader)
	}

	if errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, copy.ErrENOENT) {
		// 404 is returned for an absent container and path.  The
		// clients must deal with it accordingly.
		utils.Error(w, http.StatusNotFound, err)
		return
	} else if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	// Our work is done when the user is interested in the header only.
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	copyFunc, err := containerEngine.ContainerCopyToArchive(r.Context(), containerName, query.Path, w)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/x-tar")
	w.WriteHeader(http.StatusOK)
	if err := copyFunc(); err != nil {
		logrus.Error(err.Error())
	}
}

func handlePut(w http.ResponseWriter, r *http.Request, decoder *schema.Decoder, runtime *libpod.Runtime) {
	query := struct {
		Path                 string `schema:"path"`
		Chown                bool   `schema:"copyUIDGID"`
		Rename               string `schema:"rename"`
		NoOverwriteDirNonDir bool   `schema:"noOverwriteDirNonDir"`
	}{
		Chown: utils.IsLibpodRequest(r), // backward compatibility
	}

	err := decoder.Decode(&query, r.URL.Query())
	if err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("couldn't decode the query: %w", err))
		return
	}

	var rename map[string]string
	if query.Rename != "" {
		if err := json.Unmarshal([]byte(query.Rename), &rename); err != nil {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("couldn't decode the query field 'rename': %w", err))
			return
		}
	}

	containerName := utils.GetName(r)
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	copyFunc, err := containerEngine.ContainerCopyFromArchive(r.Context(), containerName, query.Path, r.Body,
		entities.CopyOptions{
			Chown:                query.Chown,
			NoOverwriteDirNonDir: query.NoOverwriteDirNonDir,
			Rename:               rename,
		})
	if err != nil {
		switch {
		case errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, os.ErrNotExist):
			// 404 is returned for an absent container and path.  The
			// clients must deal with it accordingly.
			utils.Error(w, http.StatusNotFound, fmt.Errorf("the container does not exist: %w", err))
		case strings.Contains(err.Error(), "copier: put: error creating file"):
			// Not the best test but need to break this out for compatibility
			// See vendor/github.com/containers/buildah/copier/copier.go:1585
			utils.Error(w, http.StatusBadRequest, err)
		default:
			utils.Error(w, http.StatusInternalServerError, err)
		}
		return
	}

	if err := copyFunc(); err != nil {
		logrus.Error(err.Error())
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
