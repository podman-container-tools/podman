//go:build !remote

package compat

import (
	"fmt"
	"net/http"
	"runtime"

	"go.podman.io/buildah"
)

// Ping returns headers to client about the service
//
// This handler must always be the same for the compatibility and libpod URL trees!
// Clients will use the Header availability to test which backend engine is in use.
// Note: Additionally handler supports GET and HEAD methods
func Ping(w http.ResponseWriter, r *http.Request) {
	// Note: API-Version and Libpod-API-Version are set in handler_api.go
	w.Header().Set("BuildKit-Version", "")
	// Docker uses Builder-Version 1 for classic builder and 2 for BuildKit
	w.Header().Set("Builder-Version", "1")
	w.Header().Set("Docker-Experimental", "true")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("OSType", runtime.GOOS)
	w.Header().Set("Pragma", "no-cache")

	w.Header().Set("Libpod-Buildah-Version", buildah.Version)
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodGet {
		fmt.Fprint(w, "OK")
	}
}
