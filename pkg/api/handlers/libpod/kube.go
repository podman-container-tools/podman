//go:build !remote && (linux || freebsd)

package libpod

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/auth"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/storage/pkg/chrootarchive"
)

// ExtractPlayReader provide an io.Reader given a http.Request object
// the function will extract the Content-Type header, if not provided, the body will be returned
// of the header define a text format (json, yaml or text) it will also return the body
// if the Content-Type is tar, we extract the content to the anchorDir and try to read the `play.yaml` file
func extractPlayReader(anchorDir string, r *http.Request) (io.Reader, error) {
	hdr, found := r.Header["Content-Type"]

	// If Content-Type is not specific we use the body
	if !found || len(hdr) == 0 {
		return r.Body, nil
	}

	var reader io.Reader
	switch hdr[0] {
	// backward compatibility
	case "text/plain":
		fallthrough
	case "application/json":
		fallthrough
	case "application/yaml":
		fallthrough
	case "application/text":
		fallthrough
	case "application/x-yaml":
		reader = r.Body
	case "application/x-tar":
		// un-tar the content
		err := chrootarchive.Untar(r.Body, anchorDir, nil)
		if err != nil {
			return nil, err
		}

		// check for play.yaml
		path := filepath.Join(anchorDir, "play.yaml")
		// open the play.yaml file
		f, err := os.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("file not found: tar missing play.yaml file at root")
		} else if err != nil {
			return nil, err
		}
		defer f.Close()
		reader = f
	default:
		return nil, fmt.Errorf("Content-Type: %s is not supported. Should be \"application/x-tar\"", hdr[0])
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func KubePlay(w http.ResponseWriter, r *http.Request) {
	// create a tmp directory
	contextDirectory, err := os.MkdirTemp("", "libpod_kube")
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// cleanup the tmp directory
	defer func() {
		err := os.RemoveAll(contextDirectory)
		if err != nil {
			logrus.Warn(fmt.Errorf("failed to remove libpod_kube tmp directory %q: %w", contextDirectory, err))
		}
	}()

	// extract the reader
	reader, err := extractPlayReader(contextDirectory, r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Annotations      map[string]string `schema:"annotations"`
		LogDriver        string            `schema:"logDriver"`
		LogOptions       []string          `schema:"logOptions"`
		Network          []string          `schema:"network"`
		NoHostname       bool              `schema:"noHostname"`
		NoHosts          bool              `schema:"noHosts"`
		NoTrunc          bool              `schema:"noTrunc"`
		Replace          bool              `schema:"replace"`
		PublishPorts     []string          `schema:"publishPorts"`
		PublishAllPorts  bool              `schema:"publishAllPorts"`
		ServiceContainer bool              `schema:"serviceContainer"`
		Start            bool              `schema:"start"`
		StaticIPs        []string          `schema:"staticIPs"`
		StaticMACs       []string          `schema:"staticMACs"`
		TLSVerify        bool              `schema:"tlsVerify"`
		Userns           string            `schema:"userns"`
		Wait             bool              `schema:"wait"`
		Build            bool              `schema:"build"`
		NoPodPrefix      bool              `schema:"noPodPrefix"`
	}{
		TLSVerify: true,
		Start:     true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	staticIPs := make([]net.IP, 0, len(query.StaticIPs))
	for _, ipString := range query.StaticIPs {
		ip := net.ParseIP(ipString)
		if ip == nil {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("invalid IP address %s", ipString))
			return
		}
		staticIPs = append(staticIPs, ip)
	}

	staticMACs := make([]net.HardwareAddr, 0, len(query.StaticMACs))
	for _, macString := range query.StaticMACs {
		mac, err := net.ParseMAC(macString)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		staticMACs = append(staticMACs, mac)
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)
	var username, password string
	if authConf != nil {
		username = authConf.Username
		password = authConf.Password
	}

	logDriver := query.LogDriver
	if logDriver == "" {
		config, err := runtime.GetConfig()
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, err)
			return
		}
		logDriver = config.Containers.LogDriver
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := entities.PlayKubeOptions{
		Annotations:        query.Annotations,
		Authfile:           authfile,
		IsRemote:           true,
		LogDriver:          logDriver,
		LogOptions:         query.LogOptions,
		Networks:           query.Network,
		NoHostname:         query.NoHostname,
		NoHosts:            query.NoHosts,
		Password:           password,
		PublishPorts:       query.PublishPorts,
		PublishAllPorts:    query.PublishAllPorts,
		Quiet:              true,
		Replace:            query.Replace,
		ServiceContainer:   query.ServiceContainer,
		StaticIPs:          staticIPs,
		StaticMACs:         staticMACs,
		UseLongAnnotations: query.NoTrunc,
		Username:           username,
		Userns:             query.Userns,
		Wait:               query.Wait,
		ContextDir:         contextDirectory,
		NoPodPrefix:        query.NoPodPrefix,
	}
	if _, found := r.URL.Query()["build"]; found {
		options.Build = types.NewOptionalBool(query.Build)
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}
	if _, found := r.URL.Query()["start"]; found {
		options.Start = types.NewOptionalBool(query.Start)
	}
	report, err := containerEngine.PlayKube(r.Context(), reader, options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("playing YAML file: %w", err))
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func KubePlayDown(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		Force bool `schema:"force"`
	}{
		Force: false,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	report, err := containerEngine.PlayKubeDown(r.Context(), r.Body, entities.PlayKubeDownOptions{Force: query.Force})
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("tearing down YAML file: %w", err))
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func KubeGenerate(w http.ResponseWriter, r *http.Request) {
	GenerateKube(w, r)
}

func KubeApply(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		CACertFile string `schema:"caCertFile"`
		Kubeconfig string `schema:"kubeconfig"`
		Namespace  string `schema:"namespace"`
	}{
		// Defaults would go here.
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	containerEngine := abi.ContainerEngine{Libpod: runtime}
	options := entities.ApplyOptions{CACertFile: query.CACertFile, Kubeconfig: query.Kubeconfig, Namespace: query.Namespace}
	if err := containerEngine.KubeApply(r.Context(), r.Body, options); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("error applying YAML to k8s cluster: %w", err))
		return
	}

	utils.WriteResponse(w, http.StatusOK, "Deployed!")
}
