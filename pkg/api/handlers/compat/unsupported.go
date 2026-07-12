//go:build !remote

package compat

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

func UnsupportedHandler(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf("Path %s is not supported", r.URL.Path)
	log.Infof("Request Failed: %s", msg)

	utils.WriteJSON(w, http.StatusNotFound, errorhandling.ErrorModel{Message: msg})
}
