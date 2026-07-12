//go:build !remote && (linux || freebsd)

package libpod

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	"go.podman.io/podman/v6/pkg/api/handlers/utils/apiutil"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/auth"
	"go.podman.io/podman/v6/pkg/channel"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	domainUtils "go.podman.io/podman/v6/pkg/domain/utils"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

func ManifestCreate(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Name        string            `schema:"name"`
		Images      []string          `schema:"images"`
		All         bool              `schema:"all"`
		Amend       bool              `schema:"amend"`
		Annotation  []string          `schema:"annotation"`
		Annotations map[string]string `schema:"annotations"`
	}{
		// Add defaults here once needed.
	}

	// Support 3.x API calls, alias image to images
	if image, ok := r.URL.Query()["image"]; ok {
		query.Images = image
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	// Support 4.x API calls, map query parameter to path
	if name, ok := mux.Vars(r)["name"]; ok {
		n, err := url.QueryUnescape(name)
		if err != nil {
			utils.Error(w, http.StatusBadRequest,
				fmt.Errorf("failed to parse name parameter %q: %w", name, err))
			return
		}
		query.Name = n
	}

	if _, err := reference.ParseNormalizedNamed(query.Name); err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("invalid image name %s: %w", query.Name, err))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	annotations := maps.Clone(query.Annotations)
	for _, annotation := range query.Annotation {
		k, v, ok := strings.Cut(annotation, "=")
		if !ok {
			utils.Error(w, http.StatusBadRequest,
				fmt.Errorf("invalid annotation %s", annotation))
			return
		}
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[k] = v
	}

	createOptions := entities.ManifestCreateOptions{All: query.All, Amend: query.Amend, Annotations: annotations}
	manID, err := imageEngine.ManifestCreate(r.Context(), query.Name, query.Images, createOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	status := http.StatusOK
	if _, err := utils.SupportedVersion(r, "< 4.0.0"); err == apiutil.ErrVersionNotSupported {
		status = http.StatusCreated
	}

	buffer, err := io.ReadAll(r.Body)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Treat \r\n as empty body
	if len(buffer) < 3 {
		utils.WriteResponse(w, status, entities.IDResponse{ID: manID})
		return
	}

	body := new(entities.ManifestModifyOptions)
	if err := json.Unmarshal(buffer, body); err != nil {
		utils.InternalServerError(w, fmt.Errorf("decoding modifications in request: %w", err))
		return
	}

	if len(body.IndexAnnotation) != 0 || len(body.IndexAnnotations) != 0 || body.IndexSubject != "" {
		manifestAnnotateOptions := entities.ManifestAnnotateOptions{
			IndexAnnotation:  body.IndexAnnotation,
			IndexAnnotations: body.IndexAnnotations,
			IndexSubject:     body.IndexSubject,
		}
		if _, err := imageEngine.ManifestAnnotate(r.Context(), manID, "", manifestAnnotateOptions); err != nil {
			utils.InternalServerError(w, err)
			return
		}
	}
	if len(body.Images) > 0 {
		if _, err := imageEngine.ManifestAdd(r.Context(), manID, body.Images, body.ManifestAddOptions); err != nil {
			utils.InternalServerError(w, err)
			return
		}
	}

	utils.WriteResponse(w, status, entities.IDResponse{ID: manID})
}

// ManifestExists return true if manifest list exists.
func ManifestExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	imageEngine := abi.ImageEngine{Libpod: runtime}
	report, err := imageEngine.ManifestExists(r.Context(), name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !report.Value {
		utils.Error(w, http.StatusNotFound, errors.New("manifest not found"))
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func ManifestInspect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	name := utils.GetName(r)
	// Wrapper to support 3.x with 4.x libpod
	query := struct {
		TLSVerify bool `schema:"tlsVerify"`
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	_, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	opts := entities.ManifestInspectOptions{Authfile: authfile}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		opts.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	manifest, err := imageEngine.ManifestInspect(r.Context(), name, opts)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, manifest)
}

// ManifestAddV3 remove digest from manifest list
//
// As of 4.0.0 use ManifestModify instead
func ManifestAddV3(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	// Wrapper to support 3.x with 4.x libpod
	query := struct {
		entities.ManifestAddOptions
		TLSVerify bool `schema:"tlsVerify"`
	}{}
	if err := utils.ReadJSONFromBody(r, &query); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	authconf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)
	var username, password string
	if authconf != nil {
		username = authconf.Username
		password = authconf.Password
	}
	query.ManifestAddOptions.Authfile = authfile
	query.ManifestAddOptions.Username = username
	query.ManifestAddOptions.Password = password
	if sys := runtime.SystemContext(); sys != nil {
		query.ManifestAddOptions.CertDir = sys.DockerCertPath
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		query.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	name := utils.GetName(r)
	if _, err := runtime.LibimageRuntime().LookupManifestList(name); err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	newID, err := imageEngine.ManifestAdd(r.Context(), name, query.Images, query.ManifestAddOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, entities.IDResponse{ID: newID})
}

// ManifestRemoveDigestV3 remove digest from manifest list
//
// As of 4.0.0 use ManifestModify instead
func ManifestRemoveDigestV3(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Digest string `schema:"digest"`
	}{
		// Add defaults here once needed.
	}
	name := utils.GetName(r)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	manifestList, err := runtime.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}
	d, err := digest.Parse(query.Digest)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := manifestList.RemoveInstance(d); err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, entities.IDResponse{ID: manifestList.ID()})
}

// ManifestPushV3 push image to registry
//
// As of 4.0.0 use ManifestPush instead
func ManifestPushV3(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		All              bool   `schema:"all"`
		Destination      string `schema:"destination"`
		RemoveSignatures bool   `schema:"removeSignatures"`
		TLSVerify        bool   `schema:"tlsVerify"`
	}{
		// Add defaults here once needed.
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	if err := utils.IsRegistryReference(query.Destination); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	source := utils.GetName(r)
	authconf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)
	var username, password string
	if authconf != nil {
		username = authconf.Username
		password = authconf.Password
	}
	options := entities.ImagePushOptions{
		All:              query.All,
		Authfile:         authfile,
		Password:         password,
		RemoveSignatures: query.RemoveSignatures,
		Username:         username,
	}
	if sys := runtime.SystemContext(); sys != nil {
		options.CertDir = sys.DockerCertPath
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}
	imageEngine := abi.ImageEngine{Libpod: runtime}
	digest, err := imageEngine.ManifestPush(r.Context(), source, query.Destination, options)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("pushing image %q: %w", query.Destination, err))
		return
	}
	utils.WriteResponse(w, http.StatusOK, entities.IDResponse{ID: digest})
}

// ManifestPush push image to registry
//
// As of 4.0.0
func ManifestPush(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := struct {
		All                    bool     `schema:"all"`
		CompressionFormat      string   `schema:"compressionFormat"`
		CompressionLevel       *int     `schema:"compressionLevel"`
		ForceCompressionFormat bool     `schema:"forceCompressionFormat"`
		Format                 string   `schema:"format"`
		RemoveSignatures       bool     `schema:"removeSignatures"`
		Retry                  uint     `schema:"retry"`
		RetryDelay             string   `schema:"retryDelay"`
		TLSVerify              bool     `schema:"tlsVerify"`
		Quiet                  bool     `schema:"quiet"`
		AddCompression         []string `schema:"addCompression"`
	}{
		// Add defaults here once needed.
		TLSVerify: true,
		// #15210: older versions did not sent *any* data, so we need
		//         to be quiet by default to remain backwards compatible
		Quiet: true,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	destination := utils.GetVar(r, "destination")
	if err := utils.IsRegistryReference(destination); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	authconf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse registry header for %s: %w", r.URL.String(), err))
		return
	}
	defer auth.RemoveAuthfile(authfile)
	var username, password string
	if authconf != nil {
		username = authconf.Username
		password = authconf.Password
	}
	options := entities.ImagePushOptions{
		All:                    query.All,
		Authfile:               authfile,
		AddCompression:         query.AddCompression,
		CompressionFormat:      query.CompressionFormat,
		CompressionLevel:       query.CompressionLevel,
		ForceCompressionFormat: query.ForceCompressionFormat,
		Format:                 query.Format,
		Password:               password,
		Quiet:                  true,
		RemoveSignatures:       query.RemoveSignatures,
		Username:               username,
	}
	if _, found := r.URL.Query()["compressionFormat"]; found {
		if _, foundForceCompression := r.URL.Query()["forceCompressionFormat"]; !foundForceCompression {
			// If `compressionFormat` is set and no value for `forceCompressionFormat`
			// is selected then default has to be `true`.
			options.ForceCompressionFormat = true
		}
	}
	if sys := runtime.SystemContext(); sys != nil {
		options.CertDir = sys.DockerCertPath
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}
	if _, found := r.URL.Query()["retry"]; found {
		options.Retry = &query.Retry
	}
	options.RetryDelay = query.RetryDelay

	imageEngine := abi.ImageEngine{Libpod: runtime}
	source := utils.GetName(r)

	// Let's keep thing simple when running in quiet mode and push directly.
	if query.Quiet {
		digest, err := imageEngine.ManifestPush(r.Context(), source, destination, options)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("pushing image %q: %w", destination, err))
			return
		}
		utils.WriteResponse(w, http.StatusOK, entities.ManifestPushReport{ID: digest})
		return
	}

	writer := channel.NewWriter(make(chan []byte))
	defer writer.Close()
	options.Writer = writer

	pushCtx, pushCancel := context.WithCancel(r.Context())
	var digest string
	var pushError error
	go func() {
		defer pushCancel()
		digest, pushError = imageEngine.ManifestPush(pushCtx, source, destination, options)
	}()

	flush := func() {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	flush()

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	for {
		var report entities.ManifestPushReport
		select {
		case s := <-writer.Chan():
			report.Stream = string(s)
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to encode json: %v", err)
			}
			flush()
		case <-pushCtx.Done():
			if pushError != nil {
				report.Error = pushError.Error()
			} else {
				report.ID = digest
			}
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to encode json: %v", err)
			}
			flush()
			return
		case <-r.Context().Done():
			// Client has closed connection
			return
		}
	}
}

// ManifestModify efficiently updates the named manifest list
func ManifestModify(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	imageEngine := abi.ImageEngine{Libpod: runtime}

	body := new(entities.ManifestModifyOptions)

	multireader, err := r.MultipartReader()
	if err != nil {
		multireader = nil
		// not multipart - request is just encoded JSON, nothing else
		if err := utils.ReadJSONFromBody(r, &body); err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
	} else {
		// multipart - request is encoded JSON in the first part, each artifact is its own part
		bodyPart, err := multireader.NextPart()
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("reading first part of multipart request: %w", err))
			return
		}
		err = json.NewDecoder(bodyPart).Decode(body)
		bodyPart.Close()
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("decoding modify request in multipart request: %w", err))
			return
		}
	}

	name := utils.GetName(r)
	manifestList, err := runtime.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	if len(body.ManifestAddOptions.Annotation) != 0 {
		if len(body.ManifestAddOptions.Annotations) != 0 {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("can not set both Annotation and Annotations"))
			return
		}
		annots, err := domainUtils.ParseAnnotations(body.ManifestAddOptions.Annotation)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		body.ManifestAddOptions.Annotations = annots
		body.ManifestAddOptions.Annotation = nil
	}
	if len(body.ManifestAddOptions.IndexAnnotation) != 0 {
		if len(body.ManifestAddOptions.IndexAnnotations) != 0 {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("can not set both IndexAnnotation and IndexAnnotations"))
			return
		}
		annots, err := domainUtils.ParseAnnotations(body.ManifestAddOptions.IndexAnnotation)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		body.ManifestAddOptions.IndexAnnotations = annots
		body.ManifestAddOptions.IndexAnnotation = nil
	}

	var artifactExtractionError error
	var artifactExtraction sync.WaitGroup
	if multireader != nil {
		// If the data was multipart, then save items from it into a
		// directory that will be removed along with this list,
		// whenever that happens.
		artifactExtraction.Go(func() {
			storageConfig := runtime.StorageConfig()
			// FIXME: knowing that this is the location of the
			// per-image-record-stuff directory is a little too
			// "inside storage"
			fileDir, err := os.MkdirTemp(filepath.Join(runtime.GraphRoot(), storageConfig.GraphDriverName+"-images", manifestList.ID()), "")
			if err != nil {
				artifactExtractionError = err
				return
			}
			// We'll be building a list of the names of files we
			// received as part of the request and setting it in
			// the request body before we're done.
			var contentFiles []string
			part, err := multireader.NextPart()
			if err != nil {
				artifactExtractionError = err
				return
			}
			for part != nil {
				partName := part.FormName()
				if filename := part.FileName(); filename != "" {
					partName = filename
				}
				if partName != "" {
					partName = path.Base(partName)
				}
				// Write the file in a scope that lets us close it as quickly
				// as possible.
				if err = func() error {
					defer part.Close()
					var f *os.File
					// Create the file.
					if partName != "" {
						// Try to use the supplied name.
						f, err = os.OpenFile(filepath.Join(fileDir, partName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
					} else {
						// No supplied name means they don't care.
						f, err = os.CreateTemp(fileDir, "upload")
					}
					if err != nil {
						return err
					}
					defer f.Close()
					// Write the file's contents.
					if _, err = io.Copy(f, part); err != nil {
						return err
					}
					contentFiles = append(contentFiles, f.Name())
					return nil
				}(); err != nil {
					break
				}
				part, err = multireader.NextPart()
			}
			// If we stowed all of the uploaded files without issue, we're all good.
			if err != nil && !errors.Is(err, io.EOF) {
				artifactExtractionError = err
				return
			}
			// Save the list of files that we created.
			body.ArtifactFiles = contentFiles
		})
	}

	if tlsVerify, ok := r.URL.Query()["tlsVerify"]; ok {
		tls, err := strconv.ParseBool(tlsVerify[len(tlsVerify)-1])
		if err != nil {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("tlsVerify param is not a bool: %w", err))
			return
		}
		body.SkipTLSVerify = types.NewOptionalBool(!tls)
	}

	authconf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)
	var username, password string
	if authconf != nil {
		username = authconf.Username
		password = authconf.Password
	}
	body.ManifestAddOptions.Authfile = authfile
	body.ManifestAddOptions.Username = username
	body.ManifestAddOptions.Password = password
	if sys := runtime.SystemContext(); sys != nil {
		body.ManifestAddOptions.CertDir = sys.DockerCertPath
	}

	report := entities.ManifestModifyReport{ID: manifestList.ID()}
	switch {
	case strings.EqualFold("update", body.Operation):
		if len(body.Images) > 0 {
			id, err := imageEngine.ManifestAdd(r.Context(), name, body.Images, body.ManifestAddOptions)
			if err != nil {
				report.Errors = append(report.Errors, err)
				break
			}
			report.ID = id
			report.Images = body.Images
		}
		if multireader != nil {
			// Wait for the extraction goroutine to finish
			// processing the message in the request body, so that
			// we know whether or not everything looked alright.
			artifactExtraction.Wait()
			if artifactExtractionError != nil {
				report.Errors = append(report.Errors, artifactExtractionError)
				artifactExtractionError = nil
				break
			}
			// Reconstruct a ManifestAddArtifactOptions from the corresponding
			// fields in the entities.ManifestModifyOptions that we decoded
			// the request struct into and then supplemented with the files list.
			// We waited until after the extraction goroutine finished to ensure
			// that we'd pick up its changes to the ArtifactFiles list.
			manifestAddArtifactOptions := entities.ManifestAddArtifactOptions{
				Type:          body.ArtifactType,
				LayerType:     body.ArtifactLayerType,
				ConfigType:    body.ArtifactConfigType,
				Config:        body.ArtifactConfig,
				ExcludeTitles: body.ArtifactExcludeTitles,
				Annotations:   body.ArtifactAnnotations,
				Subject:       body.ArtifactSubject,
				Files:         body.ArtifactFiles,
			}
			id, err := imageEngine.ManifestAddArtifact(r.Context(), name, body.ArtifactFiles, manifestAddArtifactOptions)
			if err != nil {
				report.Errors = append(report.Errors, err)
				break
			}
			report.ID = id
			report.Files = body.ArtifactFiles
		}
	case strings.EqualFold("remove", body.Operation):
		for _, image := range body.Images {
			id, err := imageEngine.ManifestRemoveDigest(r.Context(), name, image)
			if err != nil {
				report.Errors = append(report.Errors, err)
				continue
			}
			report.ID = id
			report.Images = append(report.Images, image)
		}
	case strings.EqualFold("annotate", body.Operation):
		options := body.ManifestAnnotateOptions
		images := []string{""}
		if len(body.Images) > 0 {
			images = body.Images
		}
		for _, image := range images {
			id, err := imageEngine.ManifestAnnotate(r.Context(), name, image, options)
			if err != nil {
				report.Errors = append(report.Errors, err)
				continue
			}
			report.ID = id
			if image != "" {
				report.Images = append(report.Images, image)
			}
		}
	default:
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("illegal operation %q for %q", body.Operation, r.URL.String()))
		return
	}

	// In case something weird happened, don't just let the goroutine go; make the
	// client at least wait for it.
	artifactExtraction.Wait()
	if artifactExtractionError != nil {
		report.Errors = append(report.Errors, artifactExtractionError)
	}

	statusCode := http.StatusOK
	switch {
	case len(report.Errors) > 0 && len(report.Images) > 0:
		statusCode = http.StatusConflict
	case len(report.Errors) > 0:
		statusCode = http.StatusBadRequest
	}
	utils.WriteResponse(w, statusCode, report)
}

// ManifestDelete removes a manifest list from storage
func ManifestDelete(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	imageEngine := abi.ImageEngine{Libpod: runtime}

	query := struct {
		Ignore bool `schema:"ignore"`
	}{
		// Add defaults here once needed.
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	opts := entities.ImageRemoveOptions{
		Ignore: query.Ignore,
	}

	name := utils.GetName(r)
	rmReport, rmErrors := imageEngine.ManifestRm(r.Context(), []string{name}, opts)
	// In contrast to batch-removal, where we're only setting the exit
	// code, we need to have another closer look at the errors here and set
	// the appropriate http status code.

	switch rmReport.ExitCode {
	case 0:
		report := handlers.LibpodImagesRemoveReport{ImageRemoveReport: *rmReport, Errors: []string{}}
		utils.WriteResponse(w, http.StatusOK, report)
	case 1:
		// 404 - no such image
		utils.Error(w, http.StatusNotFound, errorhandling.JoinErrors(rmErrors))
	case 2:
		// 409 - conflict error (in use by containers)
		utils.Error(w, http.StatusConflict, errorhandling.JoinErrors(rmErrors))
	default:
		// 500 - internal error
		utils.Error(w, http.StatusInternalServerError, errorhandling.JoinErrors(rmErrors))
	}
}
