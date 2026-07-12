//go:build !remote && (linux || freebsd)

package libpod

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/schema"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/compat"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/specgenutil"
	"go.podman.io/podman/v6/pkg/util"
)

func ContainerExists(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	name := utils.GetName(r)
	query := struct {
		External bool `schema:"external"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	options := entities.ContainerExistsOptions{
		External: query.External,
	}

	report, err := containerEngine.ContainerExists(r.Context(), name, options)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) {
			utils.ContainerNotFound(w, name, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	if report.Value {
		utils.WriteResponse(w, http.StatusNoContent, "")
	} else {
		utils.ContainerNotFound(w, name, define.ErrNoSuchCtr)
	}
}

func ListContainers(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		All       bool `schema:"all"`
		External  bool `schema:"external"`
		Last      int  `schema:"last"` // alias for limit
		Limit     int  `schema:"limit"`
		Namespace bool `schema:"namespace"`
		Size      bool `schema:"size"`
		Sync      bool `schema:"sync"`
	}{
		// override any golang type defaults
	}

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to decode filter parameters for %s: %w", r.URL.String(), err))
		return
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	limit := query.Limit
	// Support `last` as an alias for `limit`.  While Podman uses --last in
	// the CLI, the API is using `limit`.  As we first used `last` in the
	// API as well, we decided to go with aliasing to prevent any
	// regression. See github.com/containers/podman/issues/6413.
	if _, found := r.URL.Query()["last"]; found {
		logrus.Info("List containers: received `last` parameter - overwriting `limit`")
		limit = query.Last
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	containerEngine := abi.ContainerEngine{Libpod: runtime}
	opts := entities.ContainerListOptions{
		All:       query.All,
		External:  query.External,
		Filters:   *filterMap,
		Last:      limit,
		Namespace: query.Namespace,
		// Always return Pod, should not be part of the API.
		// https://github.com/containers/podman/pull/7223
		Pod:  true,
		Size: query.Size,
		Sync: query.Sync,
	}
	pss, err := containerEngine.ContainerList(r.Context(), opts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, pss)
}

func GetContainer(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Size bool `schema:"size"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	container, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	data, err := container.Inspect(query.Size)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	// if client request old v4 payload we should return v4 compatible json
	if _, err := utils.SupportedVersion(r, ">=5.0.0"); err != nil {
		data.Config.V4PodmanCompatMarshal = true
	}

	utils.WriteResponse(w, http.StatusOK, data)
}

func WaitContainer(w http.ResponseWriter, r *http.Request) {
	utils.WaitContainerLibpod(w, r)
}

func UnmountContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	conn, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	// TODO In future it might be an improvement that libpod unmount return a
	// "container not mounted" error so we can surface that to the endpoint user
	if err := conn.Unmount(false); err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

func MountContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	conn, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	m, err := conn.Mount()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, m)
}

func ShowMountedContainers(w http.ResponseWriter, r *http.Request) {
	response := make(map[string]string)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	conns, err := runtime.GetAllContainers()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	for _, conn := range conns {
		mounted, mountPoint, err := conn.Mounted()
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		if !mounted {
			continue
		}
		response[conn.ID()] = mountPoint
	}
	utils.WriteResponse(w, http.StatusOK, response)
}

func Checkpoint(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Keep           bool   `schema:"keep"`
		LeaveRunning   bool   `schema:"leaveRunning"`
		TCPEstablished bool   `schema:"tcpEstablished"`
		Export         bool   `schema:"export"`
		IgnoreRootFS   bool   `schema:"ignoreRootFS"`
		PrintStats     bool   `schema:"printStats"`
		PreCheckpoint  bool   `schema:"preCheckpoint"`
		WithPrevious   bool   `schema:"withPrevious"`
		FileLocks      bool   `schema:"fileLocks"`
		CreateImage    string `schema:"createImage"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	name := utils.GetName(r)
	if _, err := runtime.LookupContainer(name); err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	names := []string{name}

	options := entities.CheckpointOptions{
		Keep:           query.Keep,
		LeaveRunning:   query.LeaveRunning,
		TCPEstablished: query.TCPEstablished,
		IgnoreRootFS:   query.IgnoreRootFS,
		PrintStats:     query.PrintStats,
		PreCheckPoint:  query.PreCheckpoint,
		WithPrevious:   query.WithPrevious,
		FileLocks:      query.FileLocks,
		CreateImage:    query.CreateImage,
	}

	if query.Export {
		f, err := os.CreateTemp("", "checkpoint")
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		defer os.Remove(f.Name())
		if err := f.Close(); err != nil {
			utils.InternalServerError(w, err)
			return
		}
		options.Export = f.Name()
	}

	reports, err := containerEngine.ContainerCheckpoint(r.Context(), names, options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if len(reports) != 1 {
		utils.InternalServerError(w, fmt.Errorf("expected 1 restore report but got %d", len(reports)))
		return
	}
	if reports[0].Err != nil {
		utils.InternalServerError(w, reports[0].Err)
		return
	}

	if !query.Export {
		utils.WriteResponse(w, http.StatusOK, reports[0])
		return
	}

	f, err := os.Open(options.Export)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	defer f.Close()
	utils.WriteResponse(w, http.StatusOK, f)
}

func Restore(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Keep            bool   `schema:"keep"`
		TCPEstablished  bool   `schema:"tcpEstablished"`
		TCPClose        bool   `schema:"tcpClose"`
		Import          bool   `schema:"import"`
		Name            string `schema:"name"`
		IgnoreRootFS    bool   `schema:"ignoreRootFS"`
		IgnoreVolumes   bool   `schema:"ignoreVolumes"`
		IgnoreStaticIP  bool   `schema:"ignoreStaticIP"`
		IgnoreStaticMAC bool   `schema:"ignoreStaticMAC"`
		PrintStats      bool   `schema:"printStats"`
		FileLocks       bool   `schema:"fileLocks"`
		PublishPorts    string `schema:"publishPorts"`
		Pod             string `schema:"pod"`
	}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	options := entities.RestoreOptions{
		Name:            query.Name,
		Keep:            query.Keep,
		TCPEstablished:  query.TCPEstablished,
		TCPClose:        query.TCPClose,
		IgnoreRootFS:    query.IgnoreRootFS,
		IgnoreVolumes:   query.IgnoreVolumes,
		IgnoreStaticIP:  query.IgnoreStaticIP,
		IgnoreStaticMAC: query.IgnoreStaticMAC,
		PrintStats:      query.PrintStats,
		FileLocks:       query.FileLocks,
		PublishPorts:    strings.Fields(query.PublishPorts),
		Pod:             query.Pod,
	}

	var names []string
	if query.Import {
		t, err := os.CreateTemp("", "restore")
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		defer os.Remove(t.Name())
		if err := compat.SaveFromBody(t, r); err != nil {
			utils.InternalServerError(w, err)
			return
		}
		options.Import = t.Name()
	} else {
		name := utils.GetName(r)
		if _, err := runtime.LookupContainer(name); err != nil {
			// If container was not found, check if this is a checkpoint image
			ir := abi.ImageEngine{Libpod: runtime}
			report, err := ir.Exists(r.Context(), name)
			if err != nil {
				utils.Error(w, http.StatusNotFound, fmt.Errorf("failed to find container or checkpoint image %s: %w", name, err))
				return
			}
			if !report.Value {
				utils.Error(w, http.StatusNotFound, fmt.Errorf("failed to find container or checkpoint image %s", name))
				return
			}
		}
		names = []string{name}
	}

	reports, err := containerEngine.ContainerRestore(r.Context(), names, options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if len(reports) != 1 {
		utils.InternalServerError(w, fmt.Errorf("expected 1 restore report but got %d", len(reports)))
		return
	}
	if reports[0].Err != nil {
		utils.InternalServerError(w, reports[0].Err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, reports[0])
}

func InitContainer(w http.ResponseWriter, r *http.Request) {
	name := utils.GetName(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}
	err = ctr.Init(r.Context(), ctr.PodID() != "")
	if errors.Is(err, define.ErrCtrStateInvalid) {
		utils.Error(w, http.StatusNotModified, err)
		return
	}
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

// UInt64OrMinusOne is a small helper that unmarshals JSON numbers and allows negative numbers,
// mapping them to uint64 max to represent "unlimited" for rlimits.
type UInt64OrMinusOne uint64

func (x *UInt64OrMinusOne) UnmarshalJSON(b []byte) error {
	var n json.Number

	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err := dec.Decode(&n); err != nil {
		return err
	}

	// Handle signed numbers for the unlimited/maximum case
	if i, err := n.Int64(); err == nil {
		if i < 0 {
			*x = UInt64OrMinusOne(^uint64(0))
			return nil
		}
		*x = UInt64OrMinusOne(uint64(i))
		return nil
	}

	// Handle large unsigned values
	u, err := strconv.ParseUint(n.String(), 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse UInt64OrMinusOne: %w", err)
	}
	*x = UInt64OrMinusOne(u)
	return nil
}

type WirePOSIXRlimit struct {
	// Type of the rlimit to set
	Type string `json:"type"`
	// Hard is the hard limit for the specified type
	Hard UInt64OrMinusOne `json:"hard"`
	// Soft is the soft limit for the specified type
	Soft UInt64OrMinusOne `json:"soft"`
}

// updateEntitiesWire mirrors handlers.UpdateEntities but allows -1 for rlimits
// via the WirePOSIXRlimit numbers.
type updateEntitiesWire struct {
	handlers.UpdateEntities
	Rlimits []WirePOSIXRlimit `json:"r_limits,omitempty"`
}

func UpdateContainer(w http.ResponseWriter, r *http.Request) {
	name := utils.GetName(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)
	query := struct {
		RestartPolicy  string `schema:"restartPolicy"`
		RestartRetries uint   `schema:"restartRetries"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	ctr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	var restartPolicy *string
	var restartRetries *uint
	if query.RestartPolicy != "" {
		restartPolicy = &query.RestartPolicy
		if query.RestartPolicy == define.RestartPolicyOnFailure {
			restartRetries = &query.RestartRetries
		} else if query.RestartRetries != 0 {
			utils.Error(w, http.StatusBadRequest, errors.New("cannot set restart retries unless restart policy is on-failure"))
			return
		}
	} else if query.RestartRetries != 0 {
		utils.Error(w, http.StatusBadRequest, errors.New("cannot set restart retries unless restart policy is set"))
		return
	}

	wire := updateEntitiesWire{}
	if err := utils.ReadJSONFromBody(r, &wire); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	options := wire.UpdateEntities
	resourceLimits, err := specgenutil.UpdateMajorAndMinorNumbers(&options.LinuxResources, &options.UpdateContainerDevicesLimits)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	rlimits, err := parseRLimits(wire.Rlimits)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("invalid rlimit: %w", err))
		return
	}

	updateOptions := &entities.ContainerUpdateOptions{
		Resources:                       resourceLimits,
		ChangedHealthCheckConfiguration: &options.UpdateHealthCheckConfig,
		RestartPolicy:                   restartPolicy,
		RestartRetries:                  restartRetries,
		Env:                             options.Env,
		UnsetEnv:                        options.UnsetEnv,
		Rlimits:                         rlimits,
	}

	err = ctr.Update(updateOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusCreated, ctr.ID())
}

// parseRLimits parses slice of WirePOSIXRlimit to slice of specs.POSIXRlimit.
// Returns nil when rLimits is empty so that the caller can distinguish if
// limits are set. Otherwise rlimits will be wiped on every run.
func parseRLimits(rLimits []WirePOSIXRlimit) ([]specs.POSIXRlimit, error) {
	if len(rLimits) == 0 {
		return nil, nil
	}
	rl := make([]specs.POSIXRlimit, 0, len(rLimits))
	for _, rLimit := range rLimits {
		if rLimit.Type == "" {
			return nil, fmt.Errorf("invalid value for POSIXRlimit.type: empty")
		}
		rl = append(rl, specs.POSIXRlimit{
			Type: rLimit.Type,
			Soft: uint64(rLimit.Soft),
			Hard: uint64(rLimit.Hard),
		})
	}
	return rl, nil
}
