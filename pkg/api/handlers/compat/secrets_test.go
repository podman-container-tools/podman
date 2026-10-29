//go:build !remote && (linux || freebsd)

package compat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

func TestCreateSecretInvalidBody(t *testing.T) {
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
			ctx := context.WithValue(context.Background(), types.RuntimeKey, (*libpod.Runtime)(nil))
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/secrets/create", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			require.NotPanics(t, func() { CreateSecret(rec, req) })
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var resp errorhandling.ErrorModel
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp), "body: %s", rec.Body.String())
			assert.True(t, strings.HasPrefix(resp.Message, tt.wantErr), "message %q should start with %q", resp.Message, tt.wantErr)
		})
	}
}
