//go:build !remote && (linux || freebsd)

package compat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// newSecretCreateRequest marshals body to JSON and returns a secret-create request carrying it.
func newSecretCreateRequest(t *testing.T, body entities.SecretCreateRequest) *http.Request {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	return httptest.NewRequest(http.MethodPost, "/secrets/create", bytes.NewReader(data))
}

// TestParseSecretCreateRequestInvalidBase64Data checks that Data must be standard, padded base64.
func TestParseSecretCreateRequestInvalidBase64Data(t *testing.T) {
	tests := []struct {
		name    string
		body    entities.SecretCreateRequest
		wantErr string
	}{
		{
			name:    "data not base64 encoded",
			body:    entities.SecretCreateRequest{Name: "mysecret", Data: "hunter2"},
			wantErr: "secret data must be base64 encoded: illegal base64 data at input byte 4",
		},
		{
			name:    "space in data",
			body:    entities.SecretCreateRequest{Name: "mysecret", Data: "c2V jcmV0"},
			wantErr: "secret data must be base64 encoded: illegal base64 data at input byte 3",
		},
		{
			name:    "valid prefix then garbage",
			body:    entities.SecretCreateRequest{Name: "mysecret", Data: "c2VjcmV0!!!"},
			wantErr: "secret data must be base64 encoded: illegal base64 data at input byte 8",
		},
		{
			name:    "url-safe alphabet",
			body:    entities.SecretCreateRequest{Name: "mysecret", Data: "_-8="},
			wantErr: "secret data must be base64 encoded: illegal base64 data at input byte 0",
		},
		{
			name:    "unneeded padding",
			body:    entities.SecretCreateRequest{Name: "mysecret", Data: "c2VjcmV0="},
			wantErr: "secret data must be base64 encoded: illegal base64 data at input byte 8",
		},
		{
			name:    "missing padding",
			body:    entities.SecretCreateRequest{Name: "mysecret", Data: "c2VjcmV0MQ"},
			wantErr: "secret data must be base64 encoded: illegal base64 data at input byte 8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSecretCreateRequest(newSecretCreateRequest(t, tt.body))
			require.EqualError(t, err, tt.wantErr)
			assert.Nil(t, got)
		})
	}
}

// TestParseSecretCreateRequestInvalidBody covers bodies that cannot be produced by
// marshaling an entities.SecretCreateRequest, so they are written out as raw JSON.
func TestParseSecretCreateRequestInvalidBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		// ReadJSONFromBody skips decoding when there is no body at all.
		{name: "no body", body: "", wantErr: "missing secret name and data"},
		// These decode fine but never allocate the embedded *SecretCreateRequest.
		{name: "empty object", body: `{}`, wantErr: "missing secret name and data"},
		{name: "json null", body: `null`, wantErr: "missing secret name and data"},
		{name: "only labels", body: `{"Labels":{"a":"b"}}`, wantErr: "missing secret name and data"},
		{name: "only unknown fields", body: `{"Foo":"bar"}`, wantErr: "missing secret name and data"},
		// Decoder errors are rejected before the nil check is reached.
		{name: "malformed json", body: `{`, wantErr: "decoding request body as JSON"},
		{name: "array instead of object", body: `[]`, wantErr: "decoding request body as JSON"},
		{name: "wrong field type", body: `{"Name":123}`, wantErr: "decoding request body as JSON"},
		{name: "plain text", body: `not json`, wantErr: "decoding request body as JSON"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/secrets/create", strings.NewReader(tt.body))

			got, err := parseSecretCreateRequest(req)
			require.Error(t, err)
			assert.Nil(t, got)
			assert.True(t, strings.HasPrefix(err.Error(), tt.wantErr), "error %q should start with %q", err.Error(), tt.wantErr)
		})
	}
}

// secretCreateBody returns the JSON of a secret-create request that is exactly size bytes long.
func secretCreateBody(t *testing.T, size int) []byte {
	t.Helper()
	secret := entities.SecretCreateRequest{Name: "mysecret", Labels: map[string]string{"pad": ""}}
	empty, err := json.Marshal(secret)
	require.NoError(t, err)
	// Base64 data comes in blocks of 4 characters. The label will take the remainder
	free := size - len(empty)
	secret.Data = strings.Repeat("A", free/4*4)
	secret.Labels["pad"] = strings.Repeat("x", free%4)
	data, err := json.Marshal(secret)
	require.NoError(t, err)
	require.Len(t, data, size)
	return data
}

// TestParseSecretCreateRequestLargeBody checks that a body over the limit is
// rejected without being read past the limit, and that a body at the limit is accepted.
func TestParseSecretCreateRequestLargeBody(t *testing.T) {
	tests := []struct {
		name          string
		size          int
		contentLength bool
		maxRead       int64
		wantErr       string
	}{
		{name: "over the limit, unknown length", size: 2 * maxSecretCreateBody, maxRead: maxSecretCreateBody + 1, wantErr: "request body is larger than 4194304 bytes"},
		{name: "over the limit, Content-Length set", size: 2 * maxSecretCreateBody, contentLength: true, maxRead: 0, wantErr: "request body is larger than 4194304 bytes"},
		{name: "one byte over the limit", size: maxSecretCreateBody + 1, maxRead: maxSecretCreateBody + 1, wantErr: "request body is larger than 4194304 bytes"},
		{name: "at the limit, unknown length", size: maxSecretCreateBody, maxRead: maxSecretCreateBody},
		{name: "at the limit, Content-Length set", size: maxSecretCreateBody, contentLength: true, maxRead: maxSecretCreateBody},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.NewReader(secretCreateBody(t, tt.size))
			req := httptest.NewRequest(http.MethodPost, "/secrets/create", body)
			// httptest.NewRequest sets ContentLength from a *bytes.Reader.
			if !tt.contentLength {
				req.ContentLength = -1
			}

			got, err := parseSecretCreateRequest(req)
			assert.LessOrEqual(t, int64(tt.size-body.Len()), tt.maxRead, "bytes read")
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
		})
	}
}
