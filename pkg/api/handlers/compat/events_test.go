//go:build !remote && (linux || freebsd)

package compat

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestEventsContentType(t *testing.T) {
	testCases := []struct {
		name    string
		path    string
		version string
		accept  string
		want    string
	}{
		{name: "no accept on 1.51", path: "/v1.51/events", version: "1.51", want: "application/json"},
		{name: "no accept on 1.52", path: "/v1.52/events", version: "1.52", want: "application/jsonl"},
		{name: "wildcard on 1.51", path: "/v1.51/events", version: "1.51", accept: "*/*", want: "application/json"},
		{name: "wildcard on 1.52", path: "/v1.52/events", version: "1.52", accept: "*/*", want: "application/jsonl"},
		{name: "json is not offered", path: "/v1.53/events", version: "1.53", accept: "application/json", want: "application/jsonl"},
		{name: "ndjson on 1.52", path: "/v1.52/events", version: "1.52", accept: "application/x-ndjson", want: "application/x-ndjson"},
		{name: "json-seq on 1.52", path: "/v1.52/events", version: "1.52", accept: "application/json-seq", want: "application/json-seq"},
		{name: "jsonl on 1.52", path: "/v1.52/events", version: "1.52", accept: "application/jsonl", want: "application/jsonl"},
		{name: "accept on old api", path: "/v1.44/events", version: "1.44", accept: "application/x-ndjson", want: "application/x-ndjson"},
		{name: "unversioned", path: "/events", want: "application/jsonl"},
		{
			name:    "docker client",
			path:    "/v1.53/events",
			version: "1.53",
			accept:  "application/jsonl, application/x-ndjson;q=0.9, application/json-seq;q=0.5",
			want:    "application/jsonl",
		},
		{name: "libpod", path: "/v6.0.0/libpod/events", version: "6.0.0", accept: "application/x-ndjson", want: "application/json"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, testCase.path, nil)
			if testCase.version != "" {
				req = mux.SetURLVars(req, map[string]string{"version": testCase.version})
			}
			if testCase.accept != "" {
				req.Header.Set("Accept", testCase.accept)
			}
			assert.Equal(t, testCase.want, eventsContentType(req))
		})
	}
}
