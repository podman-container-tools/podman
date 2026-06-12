//go:build !remote && (linux || freebsd)

package utils

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/go-units"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/config"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/docker/reference"
	storageTransport "go.podman.io/image/v5/storage"
	"go.podman.io/image/v5/transports/alltransports"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils/apiutil"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/errorhandling"
	"go.podman.io/storage"
)

// NormalizeToDockerHub normalizes the specified nameOrID to Docker Hub if the
// request is for the compat API and if containers.conf set the specific mode.
// If nameOrID is a (short) ID for a local image, the full ID will be returned.
func NormalizeToDockerHub(r *http.Request, nameOrID string) (string, error) {
	cfg, err := config.Default()
	if err != nil {
		return "", err
	}
	if IsLibpodRequest(r) || !cfg.Engine.CompatAPIEnforceDockerHub {
		return nameOrID, nil
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	// The candidate may resolve to a local non-Docker Hub image, such as
	// 'busybox' -> 'registry.com/busybox'.
	img, candidate, err := runtime.LibimageRuntime().LookupImage(nameOrID, nil)
	if err != nil {
		if !errors.Is(err, storage.ErrImageUnknown) {
			return "", fmt.Errorf("normalizing name for compat API: %v", err)
		}
		// If the image could not be resolved locally, set the
		// candidate back to the input.
		candidate = nameOrID
	} else if strings.HasPrefix(img.ID(), strings.TrimPrefix(nameOrID, "sha256:")) {
		return img.ID(), nil
	}

	// No ID, so we can normalize.
	named, err := reference.ParseNormalizedNamed(candidate)
	if err != nil {
		return "", fmt.Errorf("normalizing name %q (orig: %q) for compat API: %v", candidate, nameOrID, err)
	}

	return named.String(), nil
}

// PossiblyEnforceDockerHub sets fields in the system context to enforce
// resolving short names to Docker Hub if the request is for the compat API and
// if containers.conf set the specific mode.
func PossiblyEnforceDockerHub(r *http.Request, sys *types.SystemContext) error {
	cfg, err := config.Default()
	if err != nil {
		return err
	}
	if IsLibpodRequest(r) || !cfg.Engine.CompatAPIEnforceDockerHub {
		return nil
	}
	sys.PodmanOnlyShortNamesIgnoreRegistriesConfAndForceDockerHub = true
	return nil
}

// IsRegistryReference checks if the specified name points to the "docker://"
// transport.  If it points to no supported transport, we'll assume a
// non-transport reference pointing to an image (e.g., "fedora:latest").
func IsRegistryReference(name string) error {
	imageRef, err := alltransports.ParseImageName(name)
	if err != nil {
		// No supported transport -> assume a docker-stype reference.
		return nil //nolint: nilerr
	}
	if imageRef.Transport().Name() == docker.Transport.Name() {
		return nil
	}
	return fmt.Errorf("unsupported transport %s in %q: only docker transport is supported", imageRef.Transport().Name(), name)
}

// ParseStorageReference parses the specified image name to a
// `types.ImageReference` and enforces it to refer to a
// containers-storage-transport reference.
func ParseStorageReference(name string) (types.ImageReference, error) {
	storagePrefix := storageTransport.Transport.Name()
	imageRef, err := alltransports.ParseImageName(name)
	if err == nil && imageRef.Transport().Name() != docker.Transport.Name() {
		return nil, fmt.Errorf("reference %q must be a storage reference", name)
	} else if err != nil {
		origErr := err
		imageRef, err = alltransports.ParseImageName(fmt.Sprintf("%s:%s", storagePrefix, name))
		if err != nil {
			return nil, fmt.Errorf("reference %q must be a storage reference: %w", name, origErr)
		}
	}
	return imageRef, nil
}

func GetImage(r *http.Request, name string) (*libimage.Image, error) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	image, _, err := runtime.LibimageRuntime().LookupImage(name, nil)
	if err != nil {
		return nil, err
	}
	return image, err
}

type pullResult struct {
	images []*libimage.Image
	err    error
}

// This function is inherited from moby/moby and licensed as Apache-2.0.
// It is used to keep backward compatibility with older docker clients.
func jsonProgressToString(p *jsonstream.Progress) string {
	var (
		pbBox      string
		numbersBox string
	)
	if p.Current <= 0 && p.Total <= 0 {
		return ""
	}
	if p.Total <= 0 {
		switch p.Units {
		case "":
			return fmt.Sprintf("%8v", units.HumanSize(float64(p.Current)))
		default:
			return fmt.Sprintf("%d %s", p.Current, p.Units)
		}
	}

	percentage := min(int(float64(p.Current)/float64(p.Total)*100)/2, 50)

	numSpaces := max(50-percentage, 0)
	pbBox = fmt.Sprintf("[%s>%s] ", strings.Repeat("=", percentage), strings.Repeat(" ", numSpaces))

	switch {
	case p.HideCounts:
	case p.Units == "": // no units, use bytes
		current := units.HumanSize(float64(p.Current))
		total := units.HumanSize(float64(p.Total))

		numbersBox = fmt.Sprintf("%8v/%v", current, total)

		if p.Current > p.Total {
			// remove total display if the reported current is wonky.
			numbersBox = fmt.Sprintf("%8v", current)
		}
	default:
		numbersBox = fmt.Sprintf("%d/%d %s", p.Current, p.Total, p.Units)

		if p.Current > p.Total {
			// remove total display if the reported current is wonky.
			numbersBox = fmt.Sprintf("%d %s", p.Current, p.Units)
		}
	}

	// Show approximation of remaining time if there's enough width.
	var timeLeftBox string
	if p.Current > 0 && p.Start > 0 && percentage < 50 {
		fromStart := time.Now().UTC().Sub(time.Unix(p.Start, 0))
		perEntry := fromStart / time.Duration(p.Current)
		left := time.Duration(p.Total-p.Current) * perEntry
		timeLeftBox = " " + left.Round(time.Second).String()
	}
	return pbBox + numbersBox + timeLeftBox
}

func CompatPull(r *http.Request, w http.ResponseWriter, runtime *libpod.Runtime, reference string, pullPolicy config.PullPolicy, pullOptions *libimage.PullOptions) {
	ctx := r.Context()
	progress := make(chan types.ProgressProperties)
	pullOptions.Progress = progress

	pullResChan := make(chan pullResult)
	go func() {
		pulledImages, err := runtime.LibimageRuntime().Pull(ctx, reference, pullPolicy, pullOptions)
		pullResChan <- pullResult{images: pulledImages, err: err}
	}()

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)

	flush := func() {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	statusWritten := false
	writeStatusCode := func(code int) {
		if !statusWritten {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			flush()
			statusWritten = true
		}
	}
	progressSent := false

loop: // break out of for/select infinite loop
	for {
		report := handlers.LegacyJSONMessage{} //nolint:staticcheck // LegacyJSONMessage is deprecated but kept for Docker API compat < v1.52
		report.Progress = &jsonstream.Progress{}
		select {
		case e := <-progress:
			writeStatusCode(http.StatusOK)
			progressSent = true
			switch e.Event {
			case types.ProgressEventNewArtifact:
				report.Status = "Pulling fs layer"
			case types.ProgressEventRead:
				report.Status = "Downloading"
				report.Progress.Current = int64(e.Offset)
				report.Progress.Total = e.Artifact.Size
				// Deprecated field, but because consumers might still read it keep it.
				if _, err := apiutil.SupportedVersion(r, "<1.52.0"); err == nil {
					report.ProgressMessage = jsonProgressToString(report.Progress)
				}
			case types.ProgressEventSkipped:
				report.Status = "Already exists"
			case types.ProgressEventDone:
				report.Status = "Download complete"
			}
			report.ID = e.Artifact.Digest.Encoded()[0:12]
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to json encode error %q", err.Error())
			}
			flush()
		case pullRes := <-pullResChan:
			err := pullRes.err
			if err != nil {
				writeStatusCode(HTTPStatusFromRegistryError(err))
				msg := err.Error()
				report.Error = &jsonstream.Error{
					Message: msg,
				}
				if _, err := apiutil.SupportedVersion(r, "<1.52.0"); err == nil {
					report.ErrorMessage = msg //nolint:staticcheck // deprecated field
				}
			} else {
				pulledImages := pullRes.images
				if len(pulledImages) > 0 {
					img := pulledImages[0].ID()
					report.Status = "Download complete"
					report.ID = img[0:12]
				} else {
					msg := "internal error: no images pulled"
					report.Error = &jsonstream.Error{
						Message: msg,
					}
					if _, err := apiutil.SupportedVersion(r, "<1.52.0"); err == nil {
						report.ErrorMessage = msg //nolint:staticcheck // deprecated field
					}
					writeStatusCode(http.StatusInternalServerError)
				}
			}

			// We need to check if no progress was sent previously. In that case, we should only return the base error message.
			// This is necessary for compatibility with the Docker API.
			if err != nil && !progressSent {
				msg := errorhandling.Cause(err).Error()
				message := jsonstream.Error{
					Message: msg,
				}
				if err := enc.Encode(message); err != nil {
					logrus.Warnf("Failed to json encode error %q", err.Error())
				}
			} else {
				if err := enc.Encode(report); err != nil {
					logrus.Warnf("Failed to json encode error %q", err.Error())
				}
			}
			flush()
			break loop // break out of for/select infinite loop
		}
	}
}

func HTTPStatusFromRegistryError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var errcd errcode.ErrorCoder
	if errors.As(err, &errcd) {
		return errcd.ErrorCode().Descriptor().HTTPStatusCode
	}
	return http.StatusInternalServerError
}
