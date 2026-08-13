//go:build !remote && (linux || freebsd)

package compat

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/moby/moby/api/types/registry"
	"go.podman.io/common/pkg/auth"
	DockerClient "go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// isLocalhostServerAddress reports whether serverAddress refers to the
// "localhost" host, either as a bare "host[:port]" address (e.g.
// "localhost:5000") or as an "https://" URL (e.g. "https://localhost:5000").
//
// This intentionally parses the address as a URL instead of doing a plain
// string-prefix check: a naive check like
// strings.HasPrefix(serverAddress, "https://localhost:") can be bypassed
// with userinfo syntax such as "https://localhost:password@evil.example",
// which has that exact prefix but actually refers to "evil.example".
// Parsing the address and inspecting only the Hostname() avoids that
// confusion between userinfo and host.
func isLocalhostServerAddress(serverAddress string) bool {
	addr := serverAddress
	hasScheme := strings.Contains(addr, "://")
	if !hasScheme {
		// No scheme was given, so this is a bare "host[:port]" address
		// (e.g. "localhost:5000"). Give it a scheme so url.Parse()
		// splits host/port (and any userinfo) the same way it would
		// for a full URL.
		addr = "https://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil {
		return false
	}
	if hasScheme && u.Scheme != "https" {
		return false
	}
	return strings.EqualFold(u.Hostname(), "localhost")
}

func Auth(w http.ResponseWriter, r *http.Request) {
	var authConfig registry.AuthConfig
	if err := utils.ReadJSONFromBody(r, &authConfig); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	skipTLS := types.NewOptionalBool(false)
	if isLocalhostServerAddress(authConfig.ServerAddress) {
		// support for local testing
		skipTLS = types.NewOptionalBool(true)
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	sysCtx := runtime.SystemContext()
	sysCtx.DockerInsecureSkipTLSVerify = skipTLS

	loginOpts := &auth.LoginOptions{
		Username:    authConfig.Username,
		Password:    authConfig.Password,
		Stdout:      io.Discard,
		NoWriteBack: true, // to prevent credentials to be written on disk
	}
	if err := auth.Login(r.Context(), sysCtx, loginOpts, []string{authConfig.ServerAddress}); err == nil {
		utils.WriteResponse(w, http.StatusOK, entities.AuthReport{
			IdentityToken: "",
			Status:        "Login Succeeded",
		})
	} else {
		var msg string

		var unauthErr DockerClient.ErrUnauthorizedForCredentials
		if errors.As(err, &unauthErr) {
			msg = "401 Unauthorized"
		} else {
			msg = err.Error()
		}

		utils.WriteResponse(w, http.StatusInternalServerError, struct {
			Message string `json:"message"`
		}{
			Message: "login attempt to " + authConfig.ServerAddress + " failed with status: " + msg,
		})
	}
}
