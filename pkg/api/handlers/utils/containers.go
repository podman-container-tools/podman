//go:build !remote && (linux || freebsd)

package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.podman.io/podman/v6/libpod/events"
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

	condition := "not-running"
	if _, found := r.URL.Query()["condition"]; found {
		condition = query.Condition
		if !isValidDockerCondition(query.Condition) {
			BadRequest(w, "condition", condition, errors.New("not a valid docker condition"))
			return
		}
	}

	name := GetName(r)

	exists, err := containerExists(ctx, name)
	if err != nil {
		InternalServerError(w, err)
		return
	}
	if !exists {
		ContainerNotFound(w, name, define.ErrNoSuchCtr)
		return
	}

	// For next-exit, subscribe to "died" events BEFORE sending the 200
	// status.  The Docker client (docker run) sends /wait, then
	// /start, but it only sends /start after receiving the 200 from
	// /wait.  By subscribing before the flush, the event listener is
	// guaranteed to be ready before the container can start and exit.
	// https://github.com/containers/podman/issues/28514
	var eventChannel chan events.ReadResult
	var cancelEvents context.CancelFunc
	if condition == "next-exit" {
		eventChannel, cancelEvents, err = subscribeContainerDied(ctx, name)
		if err != nil {
			InternalServerError(w, err)
			return
		}
		defer cancelEvents()
	}

	// In docker compatibility mode we have to send headers in advance,
	// otherwise docker client would freeze.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	exitCode, err := waitDockerCondition(ctx, name, eventChannel, interval, condition)
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
	}
	if len(reports) != 1 {
		Error(w, http.StatusInternalServerError, fmt.Errorf("the ContainerWait() function returned unexpected count of reports: %d", len(reports)))
		return
	}

	WriteResponse(w, http.StatusOK, strconv.Itoa(int(reports[0].ExitCode)))
}

type containerWaitFn func(conditions ...define.ContainerStatus) (int32, error)

func createContainerWaitFn(ctx context.Context, containerName string, interval time.Duration) containerWaitFn {
	runtime := ctx.Value(api.RuntimeKey).(*libpod.Runtime)
	var containerEngine entities.ContainerEngine = &abi.ContainerEngine{Libpod: runtime}

	return func(conditions ...define.ContainerStatus) (int32, error) {
		rawConditions := make([]string, 0, len(conditions))
		for _, con := range conditions {
			rawConditions = append(rawConditions, con.String())
		}
		opts := entities.WaitOptions{
			Conditions: rawConditions,
			Interval:   interval,
		}
		ctrWaitReport, err := containerEngine.ContainerWait(ctx, []string{containerName}, opts)
		if err != nil {
			return -1, err
		}
		if len(ctrWaitReport) != 1 {
			return -1, fmt.Errorf("the ContainerWait() function returned unexpected count of reports: %d", len(ctrWaitReport))
		}
		return ctrWaitReport[0].ExitCode, ctrWaitReport[0].Error
	}
}

func containerExists(ctx context.Context, name string) (bool, error) {
	runtime := ctx.Value(api.RuntimeKey).(*libpod.Runtime)
	var containerEngine entities.ContainerEngine = &abi.ContainerEngine{Libpod: runtime}

	var ctrExistsOpts entities.ContainerExistsOptions
	ctrExistRep, err := containerEngine.ContainerExists(ctx, name, ctrExistsOpts)
	if err != nil {
		return false, err
	}
	return ctrExistRep.Value, nil
}

func isValidDockerCondition(cond string) bool {
	switch cond {
	case "next-exit", "removed", "not-running", "":
		return true
	}
	return false
}

func waitDockerCondition(ctx context.Context, containerName string, eventChannel chan events.ReadResult, interval time.Duration, dockerCondition string) (int32, error) {
	containerWait := createContainerWaitFn(ctx, containerName, interval)

	var err error
	var code int32
	switch dockerCondition {
	case "next-exit":
		code, err = waitNextExit(eventChannel)
	case "removed":
		code, err = waitRemoved(containerWait)
	case "not-running", "":
		code, err = waitNotRunning(containerWait)
	default:
		panic("not a valid docker condition")
	}
	return code, err
}

var notRunningStates = []define.ContainerStatus{
	define.ContainerStateConfigured,
	define.ContainerStateCreated,
	define.ContainerStateStopped,
	define.ContainerStateExited,
	define.ContainerStateRemoving,
}

func waitRemoved(ctrWait containerWaitFn) (int32, error) {
	var code int32
	for {
		c, err := ctrWait(define.ContainerStateExited)
		if errors.Is(err, define.ErrNoSuchCtr) {
			// Make sure to wait until the container has been removed.
			break
		}
		if err != nil {
			return code, err
		}
		// If the container doesn't exist, the return code is -1, so
		// only set it in case of success.
		code = c
	}
	return code, nil
}

// subscribeContainerDied starts listening for "died" events for the given
// container and returns the event channel and a cancel function.
//
// The caller must call cancel when done to stop the background goroutine.
func subscribeContainerDied(ctx context.Context, containerName string) (chan events.ReadResult, context.CancelFunc, error) {
	runtime := ctx.Value(api.RuntimeKey).(*libpod.Runtime)
	containerEngine := &abi.ContainerEngine{Libpod: runtime}
	eventChannel := make(chan events.ReadResult)
	opts := entities.EventsOptions{
		EventChan: eventChannel,
		Filter:    []string{"event=died", fmt.Sprintf("container=%s", containerName)},
		Stream:    true,
	}
	ctx, cancel := context.WithCancel(ctx)
	if err := containerEngine.Events(ctx, opts); err != nil {
		cancel()
		return nil, nil, err
	}
	return eventChannel, cancel, nil
}

func waitNextExit(eventChannel chan events.ReadResult) (int32, error) {
	for evt := range eventChannel {
		if evt.Error != nil {
			return -1, evt.Error
		}
		if evt.Event.ContainerExitCode != nil {
			return int32(*evt.Event.ContainerExitCode), nil
		}
	}
	return -1, nil
}

func waitNotRunning(ctrWait containerWaitFn) (int32, error) {
	return ctrWait(notRunningStates...)
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
