//go:build !remote

package utils

import (
	"errors"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/errorhandling"
	"go.podman.io/storage"
)

var ErrLinkNotSupport = errors.New("link is not supported")

// TODO: document the exported functions in this file and make them more
// generic (e.g., not tied to one ctr/pod).

// Error formats an API response to an error
//
// apiMessage and code must match the container API, and are sent to client
// err is logged on the system running the podman service
func Error(w http.ResponseWriter, code int, err error) {
	// Log detailed message of what happened to machine running podman service
	log.Infof("Request Failed(%s): %s", http.StatusText(code), err.Error())
	em := errorhandling.ErrorModel{
		Because:      errorhandling.Cause(err).Error(),
		Message:      err.Error(),
		ResponseCode: code,
	}
	WriteJSON(w, code, em)
}

func VolumeNotFound(w http.ResponseWriter, _ string, err error) {
	if errors.Is(err, define.ErrNoSuchVolume) || errors.Is(err, define.ErrVolumeExists) {
		Error(w, http.StatusNotFound, err)
		return
	}
	InternalServerError(w, err)
}

func ContainerNotFound(w http.ResponseWriter, _ string, err error) {
	if errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, define.ErrCtrExists) {
		Error(w, http.StatusNotFound, err)
		return
	}
	InternalServerError(w, err)
}

func ImageNotFound(w http.ResponseWriter, _ string, err error) {
	if !errors.Is(err, storage.ErrImageUnknown) {
		InternalServerError(w, err)
		return
	}
	Error(w, http.StatusNotFound, err)
}

func ArtifactNotFound(w http.ResponseWriter, _ string, err error) {
	Error(w, http.StatusNotFound, err)
}

func NetworkNotFound(w http.ResponseWriter, _ string, err error) {
	if !errors.Is(err, define.ErrNoSuchNetwork) {
		InternalServerError(w, err)
		return
	}
	Error(w, http.StatusNotFound, err)
}

func PodNotFound(w http.ResponseWriter, _ string, err error) {
	if !errors.Is(err, define.ErrNoSuchPod) {
		InternalServerError(w, err)
		return
	}
	Error(w, http.StatusNotFound, err)
}

func SessionNotFound(w http.ResponseWriter, _ string, err error) {
	if !errors.Is(err, define.ErrNoSuchExecSession) {
		InternalServerError(w, err)
		return
	}
	Error(w, http.StatusNotFound, err)
}

func SecretNotFound(w http.ResponseWriter, _ string, err error) {
	if errorhandling.Cause(err).Error() != "no such secret" {
		InternalServerError(w, err)
		return
	}
	Error(w, http.StatusNotFound, err)
}

func InternalServerError(w http.ResponseWriter, err error) {
	Error(w, http.StatusInternalServerError, err)
}

func BadRequest(w http.ResponseWriter, key string, value string, err error) {
	e := fmt.Errorf("failed to parse query parameter '%s': %q: %w", key, value, err)
	Error(w, http.StatusBadRequest, e)
}

// UnsupportedParameter logs a given param by its string name as not supported.
func UnSupportedParameter(param string) {
	log.Infof("API parameter %q: not supported", param)
}

type BuildError struct {
	err  error
	code int
}

func (e *BuildError) Error() string {
	return e.err.Error()
}

func GetFileNotFoundError(err error) *BuildError {
	return &BuildError{code: http.StatusNotFound, err: err}
}

func GetBadRequestError(key, value string, err error) *BuildError {
	return &BuildError{code: http.StatusBadRequest, err: fmt.Errorf("failed to parse query parameter '%s': %q: %w", key, value, err)}
}

func GetGenericBadRequestError(err error) *BuildError {
	return &BuildError{code: http.StatusBadRequest, err: err}
}

func GetInternalServerError(err error) *BuildError {
	return &BuildError{code: http.StatusInternalServerError, err: err}
}

func ProcessBuildError(w http.ResponseWriter, err error) {
	if buildErr, ok := err.(*BuildError); ok {
		Error(w, buildErr.code, buildErr.err)
		return
	}
	InternalServerError(w, err)
}
