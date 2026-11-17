//go:build !remote && (linux || freebsd)

package utils

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/api/handlers"

	"go.podman.io/podman/v6/libpod/define"

	"github.com/gorilla/schema"
	"go.podman.io/podman/v6/libpod"
)

type waitQueryDocker struct {
	Condition string `schema:"condition"`
}

type waitQueryLibpod struct {
	Interval   string   `schema:"interval"`
	Conditions []string `schema:"condition"`
}

func WaitContainerDocker(w http.ResponseWriter, r *http.Request) {
	var err error
	ctx := r.Context()

	query := waitQueryDocker{}

	decoder := ctx.Value(api.DecoderKey).(*schema.Decoder)
	if err = decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	interval := time.Millisecond * 250

	condition := query.Condition
	switch condition {
	case "":
		condition = define.WaitConditionNotRunning
	case define.WaitConditionNextExit, define.WaitConditionNotRunning, define.WaitConditionRemoved:
	default:
		BadRequest(w, "condition", condition, errors.New("not a valid docker condition"))
		return
	}

	runtime := ctx.Value(api.RuntimeKey).(*libpod.Runtime)
	name := GetName(r)

	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, define.ErrCtrRemoved) {
			ContainerNotFound(w, name, define.ErrNoSuchCtr)
			return
		}
		InternalServerError(w, err)
		return
	}

	// In docker compatibility mode we have to send headers in advance,
	// otherwise Docker client would freeze. They must only be sent once the
	// wait is armed: docker run sends /start after receiving them, so the
	// exit could be missed otherwise (#28514).
	headersSent := false
	ready := func() {
		headersSent = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	exitCode, err := ctr.WaitForConditionWithInterval(ctx, ready, interval, condition)
	if err != nil && !headersSent {
		InternalServerError(w, err)
		return
	}
	var errStruct *struct{ Message string }
	if err != nil {
		logrus.Errorf("While waiting on condition: %q", err)
		errStruct = &struct {
			Message string
		}{
			Message: err.Error(),
		}
	}

	responseData := handlers.ContainerWaitOKBody{
		StatusCode: int(exitCode),
		Error:      errStruct,
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	err = enc.Encode(&responseData)
	if err != nil {
		logrus.Errorf("Unable to write json: %q", err)
	}
}

func WaitContainerLibpod(w http.ResponseWriter, r *http.Request) {
	var (
		err      error
		interval = time.Millisecond * 250
	)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := waitQueryLibpod{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if _, found := r.URL.Query()["interval"]; found {
		interval, err = time.ParseDuration(query.Interval)
		if err != nil {
			InternalServerError(w, err)
			return
		}
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	containerEngine := &abi.ContainerEngine{Libpod: runtime}
	opts := entities.WaitOptions{
		Conditions: query.Conditions,
		Interval:   interval,
	}
	name := GetName(r)
	reports, err := containerEngine.ContainerWait(r.Context(), []string{name}, opts)
	if err == nil && len(reports) == 1 {
		err = reports[0].Error
	}
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			// Special case: In the common scenario of podman-remote run --rm
			// the API is required to attach + start + wait to get exit code.
			// This has the problem that the wait call races against the container
			// removal from the cleanup process so it may not get the exit code back.
			// However we keep the exit code around for longer than the container so
			// we can just look it up here. Of course this only works when we get a
			// full id as param but podman-remote will do that
			if len(opts.Conditions) == 0 {
				if code, err := runtime.GetContainerExitCode(name); err == nil {
					WriteResponse(w, http.StatusOK, strconv.Itoa(int(code)))
					return
				}
			}
			ContainerNotFound(w, name, err)
			return
		}
		InternalServerError(w, err)
		return
	}
	if len(reports) != 1 {
		Error(w, http.StatusInternalServerError, fmt.Errorf("the ContainerWait() function returned unexpected count of reports: %d", len(reports)))
		return
	}

	WriteResponse(w, http.StatusOK, strconv.Itoa(int(reports[0].ExitCode)))
}

// PSTitles merges CAPS headers from ps output. All PS headers are single words, except for
// CAPS. Function compines CAP Headers into single field separated by a space.
func PSTitles(output string) []string {
	var titles []string

	for title := range strings.FieldsSeq(output) {
		switch title {
		case "AMBIENT", "INHERITED", "PERMITTED", "EFFECTIVE", "BOUNDING":
			{
				titles = append(titles, title+" CAPS")
			}
		case "CAPS":
			continue
		default:
			titles = append(titles, title)
		}
	}
	return titles
}
