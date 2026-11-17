//go:build !remote && (linux || freebsd)

package libpod

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/schema"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/specgen/generate"
	"go.podman.io/podman/v6/pkg/specgenutil"
	"go.podman.io/podman/v6/pkg/util"
)

func PodCreate(w http.ResponseWriter, r *http.Request) {
	const (
		failedToDecodeSpecgen = "failed to decode specgen"
	)
	var (
		runtime = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		err     error
	)
	psg := specgen.PodSpecGenerator{InfraContainerSpec: &specgen.SpecGenerator{}}
	if err := utils.ReadJSONFromBody(r, &psg); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	if !psg.NoInfra {
		infraOptions := entities.NewInfraContainerCreateOptions() // options for pulling the image and FillOutSpec
		infraOptions.Net = &entities.NetOptions{}
		infraOptions.Devices = psg.Devices
		infraOptions.SecurityOpt = psg.SecurityOpt
		if !psg.Userns.IsDefault() {
			infraOptions.UserNS = psg.Userns.String()
		}
		if psg.ShareParent == nil {
			t := true
			psg.ShareParent = &t
		}
		err = specgenutil.FillOutSpecGen(psg.InfraContainerSpec, &infraOptions, []string{}) // necessary for default values in many cases (userns, idmappings)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("filling out specgen: %w", err))
			return
		}
		out, err := json.Marshal(psg) // marshal our spec so the matching options can be unmarshaled into infra
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("%v: %w", failedToDecodeSpecgen, err))
			return
		}
		err = json.Unmarshal(out, psg.InfraContainerSpec) // unmarhal matching options
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("%v: %w", failedToDecodeSpecgen, err))
			return
		}
		// a few extra that do not have the same json tags
		psg.InfraContainerSpec.Name = psg.InfraName
		psg.InfraContainerSpec.ConmonPidFile = psg.InfraConmonPidFile
		psg.InfraContainerSpec.Image = psg.InfraImage
		psg.InfraContainerSpec.RawImageName = psg.InfraImage
	}
	podSpecComplete := entities.PodSpec{PodSpecGen: psg}
	pod, err := generate.MakePod(&podSpecComplete, runtime)
	if err != nil {
		httpCode := http.StatusInternalServerError
		if errors.Is(err, define.ErrPodExists) {
			httpCode = http.StatusConflict
		}
		utils.Error(w, httpCode, fmt.Errorf("failed to make pod: %w", err))
		return
	}
	utils.WriteResponse(w, http.StatusCreated, entities.IDResponse{ID: pod.ID()})
}

func Pods(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	podPSOptions := entities.PodPSOptions{
		Filters: *filterMap,
	}
	pods, err := containerEngine.PodPs(r.Context(), podPSOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, pods)
}

func PodInspect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	podData, err := pod.Inspect()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	report := entities.PodInspectReport{
		InspectPodData: podData,
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func PodStop(w http.ResponseWriter, r *http.Request) {
	var (
		stopError error
		runtime   = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		decoder   = r.Context().Value(api.DecoderKey).(*schema.Decoder)
		responses map[string]error
	)
	query := struct {
		Timeout int `schema:"t"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}

	status, err := pod.GetPodStatus()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	if status != define.PodStateRunning {
		utils.WriteResponse(w, http.StatusNotModified, "")
		return
	}

	if query.Timeout > 0 {
		responses, stopError = pod.StopWithTimeout(r.Context(), false, query.Timeout)
	} else {
		responses, stopError = pod.Stop(r.Context(), false)
	}
	if stopError != nil && !errors.Is(stopError, define.ErrPodPartialFail) {
		utils.Error(w, http.StatusInternalServerError, stopError)
		return
	}
	// Try to clean up the pod - but only warn on failure, it's nonfatal.
	if cleanupCtrs, cleanupErr := pod.Cleanup(r.Context()); cleanupErr != nil {
		logrus.Errorf("Cleaning up pod %s: %v", pod.ID(), cleanupErr)
		for id, err := range cleanupCtrs {
			logrus.Errorf("Cleaning up pod %s container %s: %v", pod.ID(), id, err)
		}
	}

	report := entities.PodStopReport{
		Id:       pod.ID(),
		RawInput: pod.Name(),
	}
	for id, err := range responses {
		report.Errs = append(report.Errs, fmt.Errorf("stopping container %s: %w", id, err))
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, report)
}

func PodStart(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	status, err := pod.GetPodStatus()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if status == define.PodStateRunning {
		utils.WriteResponse(w, http.StatusNotModified, "")
		return
	}

	responses, err := pod.Start(r.Context())
	if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
		utils.InternalServerError(w, err)
		return
	}

	cfg, err := pod.Config()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	report := entities.PodStartReport{
		Id:       pod.ID(),
		RawInput: cfg.Name,
	}
	for id, err := range responses {
		report.Errs = append(report.Errs, fmt.Errorf("%v: %w", "starting container "+id, err))
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, report)
}

func PodDelete(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		decoder = r.Context().Value(api.DecoderKey).(*schema.Decoder)
	)
	query := struct {
		Force   bool  `schema:"force"`
		Timeout *uint `schema:"timeout"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	ctrs, err := runtime.RemovePod(r.Context(), pod, true, query.Force, query.Timeout)
	if err != nil {
		if len(ctrs) > 0 {
			// We have container errors to send as well.
			// Since we're just writing an error, and we don't want
			// special error-handling for just this endpoint: use a
			// multierror to package up all container errors.
			var allCtrErrors error
			for _, ctrErr := range ctrs {
				allCtrErrors = multierror.Append(allCtrErrors, ctrErr)
			}

			err = fmt.Errorf("%w. %s", err, allCtrErrors.Error())
		}

		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	report := entities.PodRmReport{Id: pod.ID(), RemovedCtrs: ctrs}
	utils.WriteResponse(w, http.StatusOK, report)
}

func PodRestart(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	responses, err := pod.Restart(r.Context())
	if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	report := entities.PodRestartReport{Id: pod.ID()}
	for id, err := range responses {
		report.Errs = append(report.Errs, fmt.Errorf("restarting container %s: %w", id, err))
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, report)
}

func PodPrune(w http.ResponseWriter, r *http.Request) {
	reports, err := PodPruneHelper(r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

func PodPruneHelper(r *http.Request) ([]*entities.PodPruneReport, error) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	responses, err := runtime.PrunePods(r.Context())
	if err != nil {
		return nil, err
	}
	reports := make([]*entities.PodPruneReport, 0, len(responses))
	for k, v := range responses {
		reports = append(reports, &entities.PodPruneReport{
			Err: v,
			Id:  k,
		})
	}
	return reports, nil
}

func PodPause(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	responses, err := pod.Pause(r.Context())
	if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	report := entities.PodPauseReport{Id: pod.ID()}
	for id, v := range responses {
		report.Errs = append(report.Errs, fmt.Errorf("pausing container %s: %w", id, v))
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, report)
}

func PodUnpause(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	responses, err := pod.Unpause(r.Context())
	if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	report := entities.PodUnpauseReport{Id: pod.ID()}
	for id, v := range responses {
		report.Errs = append(report.Errs, fmt.Errorf("unpausing container %s: %w", id, v))
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, &report)
}

func PodTop(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := struct {
		Delay  int    `schema:"delay"`
		PsArgs string `schema:"ps_args"`
		Stream bool   `schema:"stream"`
	}{
		Delay: 5,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if query.Delay < 1 {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("\"delay\" parameter of value %d < 1", query.Delay))
		return
	}

	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}

	wroteContent := false
	w.Header().Set("Content-Type", "application/json")

	encoder := json.NewEncoder(w)

loop: // break out of for/select infinite` loop
	for {
		select {
		case <-r.Context().Done():
			break loop
		default:
			output, err := pod.GetPodPidInformation([]string{query.PsArgs})
			if err != nil {
				if !wroteContent {
					utils.InternalServerError(w, err)
				} else {
					// ship has sailed, client already got a 200 response and expects valid
					// PodTopOKBody json format so we no longer can send the error.
					logrus.Infof("Error from %s %q : %v", r.Method, r.URL, err)
				}
				return
			}

			if len(output) > 0 {
				if !wroteContent {
					// Write header only first time around
					w.WriteHeader(http.StatusOK)
					wroteContent = true
				}
				body := handlers.PodTopOKBody{}
				body.Titles = utils.PSTitles(output[0])
				for i := range body.Titles {
					body.Titles[i] = strings.TrimSpace(body.Titles[i])
				}

				for _, line := range output[1:] {
					process := strings.Split(line, "\t")
					for i := range process {
						process[i] = strings.TrimSpace(process[i])
					}
					body.Processes = append(body.Processes, process)
				}

				if err := encoder.Encode(body); err != nil {
					logrus.Infof("Error from %s %q : %v", r.Method, r.URL, err)
					break loop
				}
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}

			if query.Stream {
				time.Sleep(time.Duration(query.Delay) * time.Second)
			} else {
				break loop
			}
		}
	}
}

func PodKill(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		decoder = r.Context().Value(api.DecoderKey).(*schema.Decoder)
		signal  = "SIGKILL"
	)
	query := struct {
		Signal string `schema:"signal"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	if _, found := r.URL.Query()["signal"]; found {
		signal = query.Signal
	}

	sig, err := util.ParseSignal(signal)
	if err != nil {
		utils.InternalServerError(w, fmt.Errorf("unable to parse signal value: %w", err))
		return
	}
	name := utils.GetName(r)
	pod, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	logrus.Debugf("Killing pod %s with signal %d", pod.ID(), sig)
	podStates, err := pod.Status()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	hasRunning := false
	for _, s := range podStates {
		if s == define.ContainerStateRunning {
			hasRunning = true
			break
		}
	}
	if !hasRunning {
		utils.Error(w, http.StatusConflict, fmt.Errorf("cannot kill a pod with no running containers: %s", pod.ID()))
		return
	}

	responses, err := pod.Kill(r.Context(), uint(sig))
	if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	report := &entities.PodKillReport{Id: pod.ID()}
	for _, v := range responses {
		if v != nil {
			report.Errs = append(report.Errs, v)
		}
	}

	code := http.StatusOK
	if len(report.Errs) > 0 {
		code = http.StatusConflict
	}
	utils.WriteResponse(w, code, report)
}

func PodExists(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	_, err := runtime.LookupPod(name)
	if err != nil {
		utils.PodNotFound(w, name, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func PodStats(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := struct {
		NamesOrIDs []string `schema:"namesOrIDs"`
		All        bool     `schema:"all"`
		Stream     bool     `schema:"stream"`
		Delay      int      `schema:"delay"`
	}{
		// default would go here
		Delay:  5,
		Stream: false,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	// Validate input.
	options := entities.PodStatsOptions{All: query.All}
	if err := entities.ValidatePodStatsOptions(query.NamesOrIDs, &options); err != nil {
		utils.InternalServerError(w, err)
		return
	}

	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}
	// Collect the stats and send them over the wire.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	reports, err := containerEngine.PodStats(r.Context(), query.NamesOrIDs, options)
	// Error checks as documented in swagger.
	if err != nil {
		if errors.Is(err, define.ErrNoSuchPod) {
			utils.Error(w, http.StatusNotFound, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)

	if err := coder.Encode(reports); err != nil {
		logrus.Infof("Error from %s %q : %v", r.Method, r.URL, err)
	}
	flush()
	if query.Stream {
		for {
			select {
			case <-r.Context().Done():
				return
			default:
				time.Sleep(time.Duration(query.Delay) * time.Second)
				reports, err = containerEngine.PodStats(r.Context(), query.NamesOrIDs, options)
				if err != nil {
					return
				}
				if err := coder.Encode(reports); err != nil {
					logrus.Infof("Error from %s %q : %v", r.Method, r.URL, err)
					return
				}
				flush()
			}
		}
	}
}
