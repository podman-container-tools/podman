//go:build !remote && (linux || freebsd)

package compat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	dockerImage "github.com/moby/moby/api/types/image"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
)

func TestFormatImageInspectResponse(t *testing.T) {
	tests := []struct {
		name       string
		version    string // empty means no version in the URL
		inspect    handlers.ImageInspect
		wantFields map[string]string
	}{
		{
			name:    "API 1.51 keeps empty Parent, DockerVersion, and Author",
			version: "1.51",
			inspect: handlers.ImageInspect{
				InspectResponse: dockerImage.InspectResponse{ID: "sha256:abc"},
			},
			wantFields: map[string]string{
				"Parent":        "",
				"DockerVersion": "",
				"Author":        "",
			},
		},
		{
			name:    "API 1.51 preserves non-empty Parent, DockerVersion, and Author",
			version: "1.51",
			inspect: handlers.ImageInspect{
				InspectResponse: dockerImage.InspectResponse{
					ID:     "sha256:abc",
					Author: "bob",
				},
				Parent:        "sha256:parent",
				DockerVersion: "28.0.0",
			},
			wantFields: map[string]string{
				"Parent":        "sha256:parent",
				"DockerVersion": "28.0.0",
				"Author":        "bob",
			},
		},
		{
			name:    "API 1.52 omits empty Parent, DockerVersion, and Author",
			version: "1.52",
			inspect: handlers.ImageInspect{
				InspectResponse: dockerImage.InspectResponse{ID: "sha256:abc"},
			},
			wantFields: map[string]string{},
		},
		{
			name:    "API 1.52 preserves non-empty Parent, DockerVersion, and Author",
			version: "1.52",
			inspect: handlers.ImageInspect{
				InspectResponse: dockerImage.InspectResponse{
					ID:     "sha256:abc",
					Author: "bob",
				},
				Parent:        "sha256:parent",
				DockerVersion: "28.0.0",
			},
			wantFields: map[string]string{
				"Parent":        "sha256:parent",
				"DockerVersion": "28.0.0",
				"Author":        "bob",
			},
		},
		{
			name:    "API 1.52 omits only the empty fields",
			version: "1.52",
			inspect: handlers.ImageInspect{
				InspectResponse: dockerImage.InspectResponse{
					ID:     "sha256:abc",
					Author: "alice",
				},
			},
			wantFields: map[string]string{
				"Author": "alice",
			},
		},
		{
			name:    "unversioned request omits empty Parent, DockerVersion, and Author",
			version: "",
			inspect: handlers.ImageInspect{
				InspectResponse: dockerImage.InspectResponse{ID: "sha256:abc"},
			},
			wantFields: map[string]string{},
		},
		{
			name:    "unversioned request preserves non-empty Parent, DockerVersion, and Author",
			version: "",
			inspect: handlers.ImageInspect{
				InspectResponse: dockerImage.InspectResponse{
					ID:     "sha256:abc",
					Author: "eric",
				},
				Parent:        "sha256:parent",
				DockerVersion: "28.0.0",
			},
			wantFields: map[string]string{
				"Parent":        "sha256:parent",
				"DockerVersion": "28.0.0",
				"Author":        "eric",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/images/test/json", nil)
			if test.version != "" {
				r = mux.SetURLVars(r, map[string]string{"version": test.version})
			}

			rec := httptest.NewRecorder()
			inspect := test.inspect
			utils.WriteResponse(rec, http.StatusOK, formatImageInspectResponse(r, &inspect))
			require.Equal(t, http.StatusOK, rec.Code)

			raw := rec.Body.String()
			var got map[string]any
			require.NoError(t, json.Unmarshal([]byte(raw), &got))

			for _, key := range []string{"Parent", "DockerVersion", "Author"} {
				want, wantPresent := test.wantFields[key]
				val, ok := got[key]
				if !wantPresent {
					assert.False(t, ok, "expected %q to be omitted, got %v", key, val)
					assert.Equal(t, 0, strings.Count(raw, `"`+key+`":`), "raw JSON should not contain %q", key)
					continue
				}
				assert.True(t, ok, "expected %q to be present", key)
				assert.Equal(t, want, val)
				assert.Equal(t, 1, strings.Count(raw, `"`+key+`":`), "raw JSON should contain %q exactly once", key)
			}
		})
	}
}
