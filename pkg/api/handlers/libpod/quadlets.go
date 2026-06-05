//go:build !remote && (linux || freebsd)

package libpod

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/systemd/quadlet"
	"go.podman.io/podman/v6/pkg/util"
	"go.podman.io/storage/pkg/chrootarchive"
)

func ListQuadlets(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.FiltersFromRequest(r)
	if err != nil {
		utils.Error(
			w, http.StatusInternalServerError,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err),
		)
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}

	quadlets, err := containerEngine.QuadletList(r.Context(), entities.QuadletListOptions{Filters: filterMap})
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, quadlets)
}

func GetQuadletPrint(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	containerEngine := abi.ContainerEngine{Libpod: runtime}

	quadletContents, err := containerEngine.QuadletPrint(r.Context(), name)
	if err != nil {
		utils.Error(w, http.StatusNotFound, fmt.Errorf("no such quadlet: %s: %w", name, err))
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(quadletContents)); err != nil {
		logrus.Errorf("Failed to write quadlet contents: %v", err)
		return
	}
}

// QuadletExists checks if a quadlet exists by name
func QuadletExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	containerEngine := abi.ContainerEngine{Libpod: runtime}

	report, err := containerEngine.QuadletExists(r.Context(), name)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if !report.Value {
		utils.Error(w, http.StatusNotFound, fmt.Errorf("no such quadlet: %s", name))
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

// extractQuadletFiles extracts quadlet files from tar archive to a temporary directory
func extractQuadletFiles(tempDir string, r io.ReadCloser) ([]string, error) {
	quadletDir := filepath.Join(tempDir, "quadlets")
	err := os.Mkdir(quadletDir, 0o700)
	if err != nil {
		return nil, err
	}

	err = chrootarchive.Untar(r, quadletDir, nil)
	if err != nil {
		return nil, err
	}

	// Collect all files from the extracted directory
	var filePaths []string
	err = filepath.Walk(quadletDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			filePaths = append(filePaths, path)
		}
		return nil
	})

	return filePaths, err
}

// processMultipartQuadlets processes multipart form data and saves files to temporary directory
func processMultipartQuadlets(tempDir string, r *http.Request) ([]string, error) {
	quadletDir := filepath.Join(tempDir, "quadlets")
	err := os.Mkdir(quadletDir, 0o700)
	if err != nil {
		return nil, err
	}

	reader, err := r.MultipartReader()
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart reader: %w", err)
	}

	var filePaths []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read multipart: %w", err)
		}
		defer part.Close()

		filename := part.FileName()
		if filename == "" {
			// Skip parts without filenames
			continue
		}

		// Create file in temp directory
		filePath := filepath.Join(quadletDir, filename)
		file, err := os.Create(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file %s: %w", filename, err)
		}
		defer file.Close()

		_, err = io.Copy(file, part)
		if err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", filename, err)
		}

		filePaths = append(filePaths, filePath)
	}

	return filePaths, nil
}

func InstallQuadlets(w http.ResponseWriter, r *http.Request) {
	// Create temporary directory for processing
	contextDirectory, err := os.MkdirTemp("", "libpod_quadlet")
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	defer func() {
		if err := os.RemoveAll(contextDirectory); err != nil {
			logrus.Warn(fmt.Errorf("failed to remove libpod_quadlet tmp directory %q: %w", contextDirectory, err))
		}
	}()

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	// Parse query parameters
	query := struct {
		Replace       bool `schema:"replace"`
		ReloadSystemd bool `schema:"reload-systemd"`
	}{
		Replace:       false,
		ReloadSystemd: true, // Default to true like CLI
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	multipart, err := utils.ValidateContentType(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	var filePaths []string
	if multipart {
		logrus.Debug("Processing multipart form data")
		filePaths, err = processMultipartQuadlets(contextDirectory, r)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
	} else {
		logrus.Debug("Processing tar archive")
		filePaths, err = extractQuadletFiles(contextDirectory, r.Body)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
	}

	if len(filePaths) == 0 {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("no files found in request"))
		return
	}

	countQuadletFiles := 0
	for _, filePath := range filePaths {
		isQuadletFile := quadlet.IsExtSupported(filePath)
		if isQuadletFile {
			countQuadletFiles++
		}
	}
	switch {
	case countQuadletFiles > 1:
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("only a single quadlet file is allowed per request"))
		return
	case countQuadletFiles == 0:
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("no quadlet files found in request"))
		return
	}

	// QuadletInstall expects the quadlet file to be first in the list.
	// filepath.Walk returns files in lexicographic order, so a non-quadlet
	// file (e.g. "Containerfile") may sort before the ".container" file.
	for i, filePath := range filePaths {
		if quadlet.IsExtSupported(filePath) {
			filePaths[0], filePaths[i] = filePaths[i], filePaths[0]
			break
		}
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	installOptions := entities.QuadletInstallOptions{
		Replace:       query.Replace,
		ReloadSystemd: query.ReloadSystemd,
	}

	installReport, err := containerEngine.QuadletInstall(r.Context(), filePaths, installOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if len(installReport.QuadletErrors) > 0 {
		var errs []error
		for path, err := range installReport.QuadletErrors {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
		}
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("errors occurred installing some Quadlets: %w", errors.Join(errs...)))
		return
	}

	utils.WriteResponse(w, http.StatusOK, installReport)
}

// RemoveQuadlet handles DELETE /libpod/quadlets/{name} to remove a quadlet file
func RemoveQuadlet(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := struct {
		Force         bool `schema:"force"`
		Ignore        bool `schema:"ignore"`
		ReloadSystemd bool `schema:"reload-systemd"`
	}{
		ReloadSystemd: true, // Default to true like CLI
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	name := utils.GetName(r)
	if name == "" {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("quadlet name must be provided"))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	removeOptions := entities.QuadletRemoveOptions{
		Force:         query.Force,
		Ignore:        query.Ignore,
		ReloadSystemd: query.ReloadSystemd,
	}

	removeReport, err := containerEngine.QuadletRemove(r.Context(), []string{name}, removeOptions)
	if err != nil {
		// For systemd connection errors and other internal errors
		utils.InternalServerError(w, err)
		return
	}

	// Check if there are errors in the report for this specific quadlet
	if err, ok := removeReport.Errors[name]; ok {
		// If ignore=false and quadlet not found, return 404
		if !query.Ignore && strings.Contains(err.Error(), "no such") {
			utils.Error(w, http.StatusNotFound, fmt.Errorf("no such quadlet: %s: %w", name, err))
			return
		}
		// If force=false and quadlet is running, return 400
		if !query.Force && errors.Is(err, define.ErrQuadletRunning) {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("quadlet %s is running and force is not set, refusing to remove: %w", name, err))
			return
		}
	}

	utils.WriteResponse(w, http.StatusOK, removeReport)
}

// RemoveQuadlets handles DELETE /libpod/quadlets to remove quadlet files (batch operation)
func RemoveQuadlets(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := struct {
		All           bool     `schema:"all"`
		Force         bool     `schema:"force"`
		Ignore        bool     `schema:"ignore"`
		ReloadSystemd bool     `schema:"reload-systemd"`
		Quadlets      []string `schema:"quadlets"`
	}{
		ReloadSystemd: true, // Default to true like CLI
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	// Validate that either all=true OR at least one quadlet name is provided
	if !query.All && len(query.Quadlets) == 0 {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("must provide at least 1 quadlet to remove or set all=true"))
		return
	}

	// Validate that both all and quadlets are not provided together
	if query.All && len(query.Quadlets) > 0 {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("when setting all=true, you may not pass any quadlet names"))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	removeOptions := entities.QuadletRemoveOptions{
		Force:         query.Force,
		All:           query.All,
		Ignore:        query.Ignore,
		ReloadSystemd: query.ReloadSystemd,
	}

	removeReport, err := containerEngine.QuadletRemove(r.Context(), query.Quadlets, removeOptions)
	if err != nil {
		// Check if it's a "must provide at least 1 quadlet" error (shouldn't happen due to validation above, but handle it)
		if strings.Contains(err.Error(), "must provide at least 1 quadlet") {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		// For systemd connection errors and other internal errors
		utils.InternalServerError(w, err)
		return
	}

	// Return 200 with the report containing errors (if any)
	// The CLI behavior returns success even with partial errors
	utils.WriteResponse(w, http.StatusOK, removeReport)
}
