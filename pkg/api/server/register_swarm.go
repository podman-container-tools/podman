//go:build !remote && (linux || freebsd)

package server

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
)

func (s *APIServer) registerSwarmHandlers(r *mux.Router) error {
	r.PathPrefix("/v{version:[0-9.]+}/configs/").HandlerFunc(noSwarm)
	r.PathPrefix("/v{version:[0-9.]+}/nodes/").HandlerFunc(noSwarm)
	r.PathPrefix("/v{version:[0-9.]+}/secrets/").HandlerFunc(noSwarm)
	r.PathPrefix("/v{version:[0-9.]+}/services/").HandlerFunc(noSwarm)
	r.PathPrefix("/v{version:[0-9.]+}/swarm/").HandlerFunc(noSwarm)
	r.PathPrefix("/v{version:[0-9.]+}/tasks/").HandlerFunc(noSwarm)

	// Added non version path to URI to support docker non versioned paths
	r.PathPrefix("/configs/").HandlerFunc(noSwarm)
	r.PathPrefix("/nodes/").HandlerFunc(noSwarm)
	r.PathPrefix("/secrets/").HandlerFunc(noSwarm)
	r.PathPrefix("/services/").HandlerFunc(noSwarm)
	r.PathPrefix("/swarm/").HandlerFunc(noSwarm)
	r.PathPrefix("/tasks/").HandlerFunc(noSwarm)
	return nil
}

// noSwarm returns http.StatusServiceUnavailable rather than something like http.StatusInternalServerError,
// this allows the client to decide if they still can talk to us
func noSwarm(w http.ResponseWriter, r *http.Request) {
	logrus.Errorf("%s is not a podman supported service", r.URL.String())
	utils.Error(w, http.StatusServiceUnavailable, errors.New("Podman does not support service: "+r.URL.String()))
}
