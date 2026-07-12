//go:build !remote

package utils

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"unsafe"

	"github.com/blang/semver/v4"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	jsoniter "github.com/json-iterator/go"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/types"

	"go.podman.io/podman/v6/pkg/api/handlers/utils/apiutil"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/bindings/images"
)

// IsLibpodRequest returns true if the request related to a libpod endpoint
// (e.g., /v2/libpod/...).
func IsLibpodRequest(r *http.Request) bool {
	return apiutil.IsLibpodRequest(r)
}

// IsLibpodLocalRequest returns true if the request related to a libpod local endpoint
// (e.g., /v2/libpod/local...).
func IsLibpodLocalRequest(r *http.Request) bool {
	return apiutil.IsLibpodLocalRequest(r)
}

// ValidateContentType validates the Content-Type header and determines if multipart processing is needed.
func ValidateContentType(r *http.Request) (bool, error) {
	multipart := false
	if hdr, found := r.Header["Content-Type"]; found && len(hdr) > 0 {
		contentType, _, err := mime.ParseMediaType(hdr[0])
		if err != nil {
			return false, GetBadRequestError("Content-Type", hdr[0], err)
		}

		switch contentType {
		case "application/tar":
			logrus.Infof("tar file content type is  %s, should use \"application/x-tar\" content type", contentType)
		case "application/x-tar":
			break
		case "multipart/form-data":
			logrus.Infof("Received %s", hdr[0])
			multipart = true
		default:
			if IsLibpodRequest(r) && !IsLibpodLocalRequest(r) {
				return false, GetBadRequestError("Content-Type", hdr[0],
					fmt.Errorf("Content-Type: %s is not supported. Should be \"application/x-tar\"", hdr[0]))
			}
			logrus.Infof("tar file content type is  %s, should use \"application/x-tar\" content type", contentType)
		}
	}
	return multipart, nil
}

// SupportedVersion validates that the version provided by client is included in the given condition
// https://github.com/blang/semver#ranges provides the details for writing conditions
// If a version is not given in URL path, ErrVersionNotGiven is returned
func SupportedVersion(r *http.Request, condition string) (semver.Version, error) {
	return apiutil.SupportedVersion(r, condition)
}

// WriteResponse encodes the given value as JSON or string and renders it for http client
func WriteResponse(w http.ResponseWriter, code int, value any) {
	// RFC2616 explicitly states that the following status codes "MUST NOT
	// include a message-body":
	switch code {
	case http.StatusNoContent, http.StatusNotModified: // 204, 304
		w.WriteHeader(code)
		return
	}

	switch v := value.(type) {
	case string:
		w.Header().Set("Content-Type", "text/plain; charset=us-ascii")
		w.WriteHeader(code)

		if _, err := fmt.Fprintln(w, v); err != nil {
			logrus.Errorf("Unable to send string response: %q", err)
		}
	case *os.File:
		w.Header().Set("Content-Type", "application/octet; charset=us-ascii")
		w.WriteHeader(code)

		if _, err := io.Copy(w, v); err != nil {
			logrus.Errorf("Unable to copy to response: %q", err)
		}
	case io.Reader:
		w.Header().Set("Content-Type", "application/x-tar")
		w.WriteHeader(code)

		if _, err := io.Copy(w, v); err != nil {
			logrus.Errorf("Unable to copy to response: %q", err)
		}
	default:
		WriteJSON(w, code, value)
	}
}

func init() {
	jsoniter.RegisterTypeEncoderFunc("error", MarshalErrorJSON, MarshalErrorJSONIsEmpty)
	jsoniter.RegisterTypeEncoderFunc("[]error", MarshalErrorSliceJSON, MarshalErrorSliceJSONIsEmpty)
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// MarshalErrorJSON writes error to stream as string
func MarshalErrorJSON(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	p := *((*error)(ptr))
	if p == nil {
		stream.WriteNil()
	} else {
		stream.WriteString(p.Error())
	}
}

// MarshalErrorSliceJSON writes []error to stream as []string JSON blob
func MarshalErrorSliceJSON(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	a := *((*[]error)(ptr))
	switch {
	case len(a) == 0:
		stream.WriteNil()
	default:
		stream.WriteArrayStart()
		for i, e := range a {
			if i > 0 {
				stream.WriteMore()
			}
			stream.WriteString(e.Error())
		}
		stream.WriteArrayEnd()
	}
}

func MarshalErrorJSONIsEmpty(ptr unsafe.Pointer) bool {
	return *((*error)(ptr)) == nil
}

func MarshalErrorSliceJSONIsEmpty(ptr unsafe.Pointer) bool {
	return len(*((*[]error)(ptr))) == 0
}

// ReadJSONFromBody reads JSON from a request body into the given struct.
// If the body is not provided/is empty, this returns immediately with no modification to the given struct.
// This means that defaults will NOT be overwritten in this case.
func ReadJSONFromBody(r *http.Request, unmarshalTo any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(unmarshalTo); err != nil {
		return fmt.Errorf("decoding request body as JSON: %w", err)
	}
	return nil
}

// WriteJSON writes an interface value encoded as JSON to w
func WriteJSON(w http.ResponseWriter, code int, value any) {
	// FIXME: we don't need to write the header in all/some circumstances.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(false)
	if err := coder.Encode(value); err != nil {
		logrus.Errorf("Unable to write json: %q", err)
	}
}

func GetVar(r *http.Request, k string) string {
	val := mux.Vars(r)[k]
	safeVal, err := url.PathUnescape(val)
	if err != nil {
		logrus.Error(fmt.Errorf("failed to unescape mux key %s, value %s: %w", k, val, err))
		return val
	}
	return safeVal
}

// GetName extracts the name from the mux
func GetName(r *http.Request) string {
	return GetVar(r, "name")
}

func GetDecoder(r *http.Request) *schema.Decoder {
	if IsLibpodRequest(r) {
		return r.Context().Value(api.DecoderKey).(*schema.Decoder)
	}
	return r.Context().Value(api.CompatDecoderKey).(*schema.Decoder)
}

// ParseOptionalJSONField unmarshals a JSON string only if the field exists in query values.
func ParseOptionalJSONField[T any](jsonStr, fieldName string, queryValues url.Values, target *T) error {
	if _, found := queryValues[fieldName]; found {
		return json.Unmarshal([]byte(jsonStr), target)
	}
	return nil
}

// ParseOptionalBool creates a types.OptionalBool if the field exists in query values.
// Returns the OptionalBool and whether the field was found.
func ParseOptionalBool(value bool, fieldName string, queryValues url.Values) (types.OptionalBool, bool) {
	if _, found := queryValues[fieldName]; found {
		return types.NewOptionalBool(value), true
	}
	return types.OptionalBoolUndefined, false
}

// ParseJSONOptionalSlice parses a JSON array string into a slice if the parameter exists.
// Returns nil if the parameter is not found.
func ParseJSONOptionalSlice(value string, query url.Values, paramName string) ([]string, error) {
	if _, found := query[paramName]; found {
		var result []string
		if err := json.Unmarshal([]byte(value), &result); err != nil {
			return nil, err
		}
		return result, nil
	}
	return nil, nil
}

// ResponseSender provides streaming JSON responses with automatic flushing.
type ResponseSender struct {
	encoder *jsoniter.Encoder
	flusher func()
}

// NewBuildResponseSender creates a ResponseSender for streaming build responses.
// Optionally writes to a debug file if PODMAN_RETAIN_BUILD_ARTIFACT is set.
func NewBuildResponseSender(w http.ResponseWriter) *ResponseSender {
	body := w.(io.Writer)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		if v, found := os.LookupEnv("PODMAN_RETAIN_BUILD_ARTIFACT"); found {
			if keep, _ := strconv.ParseBool(v); keep {
				if t, err := os.CreateTemp("", "build_*_server"); err != nil {
					logrus.Warnf("Failed to create temp file: %v", err)
				} else {
					defer t.Close()
					body = io.MultiWriter(t, w)
				}
			}
		}
	}

	enc := jsoniter.NewEncoder(body)
	enc.SetEscapeHTML(true)

	flusher := func() {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	return &ResponseSender{encoder: enc, flusher: flusher}
}

// Send encodes and sends a response object as JSON with automatic flushing.
func (b *ResponseSender) Send(response any) {
	if err := b.encoder.Encode(response); err != nil {
		logrus.Warnf("Failed to json encode build response: %v", err)
	}
	b.flusher()
}

// SendBuildStream sends a build stream message to the client.
func (b *ResponseSender) SendBuildStream(message string) {
	b.Send(images.BuildResponse{Stream: message})
}

// SendBuildError sends an error message as a build response.
func (b *ResponseSender) SendBuildError(message string) {
	response := images.BuildResponse{
		ErrorMessage: message,
		Error: &jsonstream.Error{
			Message: message,
		},
	}
	b.Send(response)
}

// SendBuildAux sends auxiliary data as part of a build response.
func (b *ResponseSender) SendBuildAux(aux []byte) {
	b.Send(images.BuildResponse{Aux: aux})
}
