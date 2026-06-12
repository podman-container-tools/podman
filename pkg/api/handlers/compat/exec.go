//go:build !remote && (linux || freebsd)

package compat

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/resize"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	"go.podman.io/podman/v6/pkg/api/server/idle"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgenutil"
	"go.podman.io/podman/v6/pkg/util"
)

// ExecCreateHandler creates an exec session for a given container.
func ExecCreateHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	input := new(handlers.ExecCreateConfig)
	if err := utils.ReadJSONFromBody(r, &input); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	ctrName := utils.GetName(r)
	ctr, err := runtime.LookupContainer(ctrName)
	if err != nil {
		utils.ContainerNotFound(w, ctrName, err)
		return
	}

	libpodConfig := new(libpod.ExecConfig)
	libpodConfig.Command = input.Cmd
	libpodConfig.Terminal = input.Tty
	libpodConfig.AttachStdin = input.AttachStdin
	libpodConfig.AttachStderr = input.AttachStderr
	libpodConfig.AttachStdout = input.AttachStdout
	if input.DetachKeys != "" {
		libpodConfig.DetachKeys = &input.DetachKeys
	}
	libpodConfig.Environment = make(map[string]string)
	for _, envStr := range input.Env {
		key, val, hasVal := strings.Cut(envStr, "=")
		if !hasVal {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("environment variable %q badly formed, must be key=value", envStr))
			return
		}
		libpodConfig.Environment[key] = val
	}
	libpodConfig.WorkDir = input.WorkingDir
	libpodConfig.Privileged = input.Privileged
	libpodConfig.User = input.User

	if input.Tty {
		util.ExecAddTERM(ctr.Env(), libpodConfig.Environment)
	}

	// Make our exit command
	storageConfig := runtime.StorageConfig()
	runtimeConfig, err := runtime.GetConfig()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	// Automatically log to syslog if the server has log-level=debug set
	exitCommandArgs, err := specgenutil.CreateExitCommandArgs(storageConfig, runtimeConfig, logrus.IsLevelEnabled(logrus.DebugLevel), true, false, true)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	libpodConfig.ExitCommand = exitCommandArgs

	// Run the exit command after 5 minutes, to mimic Docker's exec cleanup
	// behavior.
	libpodConfig.ExitCommandDelay = runtimeConfig.Engine.ExitCommandDelay

	sessID, err := ctr.ExecCreate(libpodConfig)
	if err != nil {
		if errors.Is(err, define.ErrCtrStateInvalid) {
			// Check if the container is paused. If so, return a 409
			state, err := ctr.State()
			if err == nil {
				// Ignore the error != nil case. We're already
				// throwing an InternalServerError below.
				if state == define.ContainerStatePaused {
					utils.Error(w, http.StatusConflict, fmt.Errorf("cannot create exec session as container %s is paused", ctr.ID()))
					return
				}
			}
		}
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusCreated, entities.IDResponse{ID: sessID})
}

// ExecInspectHandler inspects a given exec session.
func ExecInspectHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	sessionID := mux.Vars(r)["id"]
	sessionCtr, err := runtime.GetExecSessionContainer(sessionID)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	logrus.Debugf("Inspecting exec session %s of container %s", sessionID, sessionCtr.ID())

	session, err := sessionCtr.ExecSession(sessionID)
	if err != nil {
		utils.InternalServerError(w, fmt.Errorf("retrieving exec session %s from container %s: %w", sessionID, sessionCtr.ID(), err))
		return
	}

	inspectOut, err := session.Inspect()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, inspectOut)
}

// ExecStartHandler runs a given exec session.
func ExecStartHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	sessionID := mux.Vars(r)["id"]

	// TODO: We should read/support Tty from here.
	bodyParams := new(handlers.ExecStartConfig)
	if err := utils.ReadJSONFromBody(r, &bodyParams); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	// TODO: Verify TTY setting against what inspect session was made with

	sessionCtr, err := runtime.GetExecSessionContainer(sessionID)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	logrus.Debugf("Starting exec session %s of container %s", sessionID, sessionCtr.ID())

	state, err := sessionCtr.State()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if state != define.ContainerStateRunning {
		utils.Error(w, http.StatusConflict, fmt.Errorf("cannot exec in a container that is not running; container %s is %s", sessionCtr.ID(), state.String()))
		return
	}

	// If no streams were requested for attach, detach is implied
	if !bodyParams.Detach {
		execSession, err := sessionCtr.ExecSession(sessionID)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}

		if !execSession.Config.AttachStderr && !execSession.Config.AttachStdout && !execSession.Config.AttachStdin {
			bodyParams.Detach = true
		}
	}

	if bodyParams.Detach {
		// If we are detaching, we do NOT want to hijack.
		// Instead, we perform a detached start, and return 200 if
		// successful.
		if err := sessionCtr.ExecStart(sessionID); err != nil {
			utils.InternalServerError(w, err)
			return
		}
		// This is a 200 despite having no content
		utils.WriteResponse(w, http.StatusOK, "")
		return
	}

	logErr := func(e error) {
		logrus.Error(fmt.Errorf("attaching to container %s exec session %s: %w", sessionCtr.ID(), sessionID, e))
	}

	var size *resize.TerminalSize
	if bodyParams.Tty && (bodyParams.Height > 0 || bodyParams.Width > 0) {
		size = &resize.TerminalSize{
			Height: bodyParams.Height,
			Width:  bodyParams.Width,
		}
	}

	hijackChan := make(chan bool, 1)
	err = sessionCtr.ExecHTTPStartAndAttach(sessionID, r, w, nil, nil, nil, hijackChan, size)

	if <-hijackChan {
		// If connection was Hijacked, we have to signal it's being closed
		t := r.Context().Value(api.IdleTrackerKey).(*idle.Tracker)
		defer t.Close()

		if err != nil && !errors.Is(err, define.ErrDetach) {
			// Cannot report error to client as a 500 as the Upgrade set status to 101
			logErr(err)
		}
	} else {
		// If the Hijack failed we are going to assume we can still inform client of failure
		utils.InternalServerError(w, err)
		logErr(err)
	}
	logrus.Debugf("Attach for container %s exec session %s completed successfully", sessionCtr.ID(), sessionID)
}

// ExecRemoveHandler removes a exec session.
func ExecRemoveHandler(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	sessionID := mux.Vars(r)["id"]

	bodyParams := new(handlers.ExecRemoveConfig)
	if err := utils.ReadJSONFromBody(r, &bodyParams); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	sessionCtr, err := runtime.GetExecSessionContainer(sessionID)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	logrus.Debugf("Removing exec session %s of container %s", sessionID, sessionCtr.ID())
	if err := sessionCtr.ExecRemove(sessionID, bodyParams.Force); err != nil {
		utils.InternalServerError(w, err)
		return
	}
	logrus.Debugf("Removing exec session %s for container %s completed successfully", sessionID, sessionCtr.ID())
}
