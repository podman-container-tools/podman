//go:build !remote && (linux || freebsd)

package compat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"go.podman.io/buildah"
	buildahDefine "go.podman.io/buildah/define"
	"go.podman.io/buildah/pkg/parse"
	"go.podman.io/common/pkg/config"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/pkg/compression"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/internal/localapi"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/auth"
	"go.podman.io/podman/v6/pkg/channel"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/util"
	"go.podman.io/storage/pkg/archive"
	"go.podman.io/storage/pkg/chrootarchive"
	"go.podman.io/storage/pkg/fileutils"
)

type cleanUpFunc func()

// BuildQuery represents query parameters for the container image build API endpoint.
// Uses struct tags to map HTTP query parameters to Go fields for automatic parsing.
type BuildQuery struct {
	AddHosts                string             `schema:"extrahosts"`
	AdditionalCapabilities  string             `schema:"addcaps"`
	AdditionalBuildContexts string             `schema:"additionalbuildcontexts"`
	AllPlatforms            bool               `schema:"allplatforms"`
	Annotations             string             `schema:"annotations"`
	AppArmor                string             `schema:"apparmor"`
	BuildArgs               string             `schema:"buildargs"`
	CacheFrom               string             `schema:"cachefrom"`
	CacheTo                 string             `schema:"cacheto"`
	CacheTTL                string             `schema:"cachettl"`
	CgroupParent            string             `schema:"cgroupparent"`
	CompatVolumes           bool               `schema:"compatvolumes"`
	Compression             uint64             `schema:"compression"`
	CompressionFormat       string             `schema:"compressionFormat"`
	CompressionLevel        *int               `schema:"compressionLevel"`
	ConfigureNetwork        string             `schema:"networkmode"`
	CPPFlags                string             `schema:"cppflags"`
	CpuPeriod               uint64             `schema:"cpuperiod"`
	CpuQuota                int64              `schema:"cpuquota"`
	CpuSetCpus              string             `schema:"cpusetcpus"`
	CpuSetMems              string             `schema:"cpusetmems"`
	CpuShares               uint64             `schema:"cpushares"`
	CreatedAnnotation       types.OptionalBool `schema:"createdannotation"`
	DNSOptions              string             `schema:"dnsoptions"`
	DNSSearch               string             `schema:"dnssearch"`
	DNSServers              string             `schema:"dnsservers"`
	Devices                 string             `schema:"devices"`
	Dockerfile              string             `schema:"dockerfile"`
	DropCapabilities        string             `schema:"dropcaps"`
	Envs                    []string           `schema:"setenv"`
	Excludes                string             `schema:"excludes"`
	ForceRm                 bool               `schema:"forcerm"`
	ForceCompressionFormat  bool               `schema:"forceCompressionFormat"`
	From                    string             `schema:"from"`
	GroupAdd                []string           `schema:"groupadd"`
	HTTPProxy               bool               `schema:"httpproxy"`
	IDMappingOptions        string             `schema:"idmappingoptions"`
	IdentityLabel           bool               `schema:"identitylabel"`
	Ignore                  bool               `schema:"ignore"`
	IgnoreFile              string             `schema:"ignorefile"`
	InheritLabels           types.OptionalBool `schema:"inheritlabels"`
	InheritAnnotations      types.OptionalBool `schema:"inheritannotations"`
	Isolation               string             `schema:"isolation"`
	Jobs                    int                `schema:"jobs"`
	LabelOpts               string             `schema:"labelopts"`
	Labels                  string             `schema:"labels"`
	LayerLabels             []string           `schema:"layerLabel"`
	Layers                  bool               `schema:"layers"`
	LogRusage               bool               `schema:"rusage"`
	Manifest                string             `schema:"manifest"`
	MemSwap                 int64              `schema:"memswap"`
	Memory                  int64              `schema:"memory"`
	NamespaceOptions        string             `schema:"nsoptions"`
	NoCache                 bool               `schema:"nocache"`
	NoHosts                 bool               `schema:"nohosts"`
	OmitHistory             bool               `schema:"omithistory"`
	OSFeatures              []string           `schema:"osfeature"`
	OSVersion               string             `schema:"osversion"`
	OutputFormat            string             `schema:"outputformat"`
	Platform                []string           `schema:"platform"`
	Pull                    bool               `schema:"pull"`
	PullPolicy              string             `schema:"pullpolicy"`
	Quiet                   bool               `schema:"q"`
	Registry                string             `schema:"registry"`
	Rm                      bool               `schema:"rm"`
	RusageLogFile           string             `schema:"rusagelogfile"`
	Remote                  string             `schema:"remote"`
	RewriteTimestamp        bool               `schema:"rewritetimestamp"`
	Retry                   int                `schema:"retry"`
	RetryDelay              string             `schema:"retry-delay"`
	SaveStages              bool               `schema:"save-stages"`
	Seccomp                 string             `schema:"seccomp"`
	Secrets                 string             `schema:"secrets"`
	SecurityOpt             string             `schema:"securityopt"`
	ShmSize                 int                `schema:"shmsize"`
	SkipUnusedStages        bool               `schema:"skipunusedstages"`
	SourceDateEpoch         int64              `schema:"sourcedateepoch"`
	SourcePolicy            string             `schema:"sourcePolicy"`
	Squash                  bool               `schema:"squash"`
	StageLabels             bool               `schema:"stage-labels"`
	TLSVerify               bool               `schema:"tlsVerify"`
	Tags                    []string           `schema:"t"`
	Target                  string             `schema:"target"`
	Timestamp               int64              `schema:"timestamp"`
	TransientRunMounts      []string           `schema:"transientRunMounts"`
	Ulimits                 string             `schema:"ulimits"`
	UnsetEnvs               []string           `schema:"unsetenv"`
	UnsetLabels             []string           `schema:"unsetlabel"`
	UnsetAnnotations        []string           `schema:"unsetannotation"`
	Volumes                 []string           `schema:"volume"`
	SBOMOutput              string             `schema:"sbom-output"`
	SBOMPURLOutput          string             `schema:"sbom-purl-output"`
	ImageSBOMOutput         string             `schema:"sbom-image-output"`
	ImageSBOMPURLOutput     string             `schema:"sbom-image-purl-output"`
	ImageSBOM               string             `schema:"sbom-scanner-image"`
	SBOMCommands            string             `schema:"sbom-scanner-command"`
	SBOMMergeStrategy       string             `schema:"sbom-merge-strategy"`
}

// BuildContext represents processed build context and metadata for container image builds.
type BuildContext struct {
	ContextDirectory        string
	AdditionalBuildContexts map[string]*buildahDefine.AdditionalBuildContext
	ContainerFiles          []string
	IgnoreFile              string
}

func (b *BuildContext) validateLocalAPIPaths() error {
	if err := localapi.ValidatePathForLocalAPI(b.ContextDirectory); err != nil {
		return err
	}

	for _, containerfile := range b.ContainerFiles {
		if err := localapi.ValidatePathForLocalAPI(containerfile); err != nil {
			return err
		}
	}

	for _, ctx := range b.AdditionalBuildContexts {
		if ctx.IsURL || ctx.IsImage {
			continue
		}
		if err := localapi.ValidatePathForLocalAPI(ctx.Value); err != nil {
			return err
		}
	}

	if b.IgnoreFile != "" {
		if err := localapi.ValidatePathForLocalAPI(b.IgnoreFile); err != nil {
			return err
		}
	}

	return nil
}

// genSpaceErr wraps filesystem errors to provide more context for disk space issues.
func genSpaceErr(err error) error {
	if errors.Is(err, syscall.ENOSPC) {
		return fmt.Errorf("context directory may be too large: %w", err)
	}
	return err
}

// processCacheReferences processes JSON-encoded lists of repository references for cache operations.
func processCacheReferences(jsonValue, fieldName string, queryValues url.Values) ([]reference.Named, error) {
	var result []reference.Named
	if _, found := queryValues[fieldName]; found {
		var stringList []string
		if err := json.Unmarshal([]byte(jsonValue), &stringList); err != nil {
			return nil, err
		}
		var err error
		result, err = parse.RepoNamesToNamedReferences(stringList)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// processCacheFrom processes the cachefrom query parameter for build cache lookup.
func processCacheFrom(query *BuildQuery, queryValues url.Values) ([]reference.Named, error) {
	return processCacheReferences(query.CacheFrom, "cachefrom", queryValues)
}

// processCacheTo processes the cacheto query parameter for build cache export.
func processCacheTo(query *BuildQuery, queryValues url.Values) ([]reference.Named, error) {
	return processCacheReferences(query.CacheTo, "cacheto", queryValues)
}

// parseBuildQuery parses HTTP query parameters into a BuildQuery struct with defaults.
func parseBuildQuery(r *http.Request, conf *config.Config, queryValues url.Values) (*BuildQuery, error) {
	query := &BuildQuery{
		Dockerfile: "Dockerfile",
		Registry:   "docker.io",
		Rm:         true,
		ShmSize:    64 * 1024 * 1024,
		TLSVerify:  true,
		Retry:      int(conf.Engine.Retry),
		RetryDelay: conf.Engine.RetryDelay,
	}

	decoder := utils.GetDecoder(r)
	if err := decoder.Decode(query, queryValues); err != nil {
		return nil, utils.GetGenericBadRequestError(err)
	}

	// if layers field not set assume its not from a valid podman-client
	// could be a docker client, set `layers=true` since that is the default
	// expected behaviour
	if !utils.IsLibpodRequest(r) {
		if _, found := queryValues["layers"]; !found {
			query.Layers = true
		}
	}

	return query, nil
}

// processBuildContext processes build context directory and container files based on request parameters.
func processBuildContext(query url.Values, r *http.Request, buildContext *BuildContext, anchorDir string) (*BuildContext, error) {
	dockerFileSet := false
	remote := query.Get("remote")

	if utils.IsLibpodRequest(r) && remote != "" {
		tempDir, subDir, err := buildahDefine.TempDirForURL(anchorDir, "buildah", remote)
		if err != nil {
			return nil, utils.GetInternalServerError(genSpaceErr(err))
		}
		if tempDir != "" {
			buildContext.ContextDirectory = filepath.Join(tempDir, subDir)
		} else {
			// Nope, it was local.  Use it as is.
			absDir, err := filepath.Abs(remote)
			if err != nil {
				return nil, utils.GetBadRequestError("remote", remote, err)
			}
			buildContext.ContextDirectory = absDir
		}
	} else {
		if dockerFile := query.Get("dockerfile"); dockerFile != "" {
			m := []string{}
			if err := json.Unmarshal([]byte(dockerFile), &m); err != nil {
				// it's not json, assume just a string
				m = []string{dockerFile}
			}

			for _, containerfile := range m {
				// Add path to containerfile if it is not URL
				if !strings.HasPrefix(containerfile, "http://") && !strings.HasPrefix(containerfile, "https://") {
					if filepath.IsAbs(containerfile) {
						containerfile = filepath.Clean(filepath.FromSlash(containerfile))
					} else {
						containerfile = filepath.Join(buildContext.ContextDirectory,
							filepath.Clean(filepath.FromSlash(containerfile)))
					}
				}
				buildContext.ContainerFiles = append(buildContext.ContainerFiles, containerfile)
			}
			dockerFileSet = true
		}
	}

	if !dockerFileSet {
		buildContext.ContainerFiles = []string{filepath.Join(buildContext.ContextDirectory, "Dockerfile")}
		if utils.IsLibpodRequest(r) {
			buildContext.ContainerFiles = []string{filepath.Join(buildContext.ContextDirectory, "Containerfile")}
			if err := fileutils.Exists(buildContext.ContainerFiles[0]); err != nil {
				buildContext.ContainerFiles = []string{filepath.Join(buildContext.ContextDirectory, "Dockerfile")}
				if err1 := fileutils.Exists(buildContext.ContainerFiles[0]); err1 != nil {
					return nil, utils.GetBadRequestError("dockerfile", query.Get("dockerfile"), err1)
				}
			}
		}
	}

	// Process dockerignore only if no explicit ignorefile was specified
	if ignorefile := query.Get("ignorefile"); ignorefile != "" {
		if filepath.IsAbs(ignorefile) {
			ignoreFilePath := filepath.Clean(ignorefile)
			// In remote builds, the client may send an absolute path that exists
			// only on the client filesystem. The file is bundled in the tarball
			// and unpacked under the context directory at the same absolute path,
			// so fall back to looking it up there if the absolute path is missing.
			if err := fileutils.Exists(ignoreFilePath); err != nil {
				ignoreFilePath = filepath.Join(buildContext.ContextDirectory, ignoreFilePath)
			}
			buildContext.IgnoreFile = ignoreFilePath
		} else {
			ignoreFilePath, err := securejoin.SecureJoin(buildContext.ContextDirectory, ignorefile)
			if err != nil {
				return nil, utils.GetInternalServerError(fmt.Errorf("processing ignore file path: %w", err))
			}
			buildContext.IgnoreFile = ignoreFilePath
		}
	} else {
		_, ignoreFile, err := util.ParseDockerignore(buildContext.ContainerFiles, buildContext.ContextDirectory)
		if err != nil {
			return nil, utils.GetInternalServerError(fmt.Errorf("processing ignore file: %w", err))
		}
		buildContext.IgnoreFile = ignoreFile
	}

	return buildContext, nil
}

// processSecrets processes build secrets for podman-remote operations.
// Moves secrets outside build context to prevent accidental inclusion in images.
func processSecrets(query *BuildQuery, contextDirectory string, queryValues url.Values) ([]string, error) {
	secrets := []string{}
	m := []string{}
	if err := utils.ParseOptionalJSONField(query.Secrets, "secrets", queryValues, &m); err != nil {
		return nil, err
	}

	// for podman-remote all secrets must be picked from context director
	// hence modify src so contextdir is added as prefix
	for _, secret := range m {
		secretOpt := strings.Split(secret, ",")
		if len(secretOpt) > 0 {
			modifiedOpt := []string{}
			for _, token := range secretOpt {
				key, val, hasVal := strings.Cut(token, "=")
				if hasVal {
					if key == "src" {
						/* move secret away from contextDir */
						/* to make sure we dont accidentally commit temporary secrets to image*/
						builderDirectory, _ := filepath.Split(contextDirectory)
						// following path is outside build context
						newSecretPath := filepath.Join(builderDirectory, val)
						oldSecretPath := filepath.Join(contextDirectory, val)
						err := os.Rename(oldSecretPath, newSecretPath)
						if err != nil {
							return nil, err
						}

						modifiedSrc := fmt.Sprintf("src=%s", newSecretPath)
						modifiedOpt = append(modifiedOpt, modifiedSrc)
					} else {
						modifiedOpt = append(modifiedOpt, token)
					}
				}
			}
			secrets = append(secrets, strings.Join(modifiedOpt, ","))
		}
	}
	return secrets, nil
}

// createBuildOptions creates a buildah BuildOptions struct from query parameters and build context.
// WARNING: caller must call the cleanup function if not nil.
func createBuildOptions(query *BuildQuery, buildCtx *BuildContext, queryValues url.Values, r *http.Request) (*buildahDefine.BuildOptions, cleanUpFunc, error) {
	identityLabel, _ := utils.ParseOptionalBool(query.IdentityLabel, "identitylabel", queryValues)

	// Process various query parameters
	addCaps, err := utils.ParseJSONOptionalSlice(query.AdditionalCapabilities, queryValues, "addcaps")
	if err != nil {
		return nil, nil, utils.GetBadRequestError("addcaps", query.AdditionalCapabilities, err)
	}

	dropCaps, err := utils.ParseJSONOptionalSlice(query.DropCapabilities, queryValues, "dropcaps")
	if err != nil {
		return nil, nil, utils.GetBadRequestError("dropcaps", query.DropCapabilities, err)
	}

	devices, err := utils.ParseJSONOptionalSlice(query.Devices, queryValues, "devices")
	if err != nil {
		return nil, nil, utils.GetBadRequestError("devices", query.Devices, err)
	}

	dnsservers, err := utils.ParseJSONOptionalSlice(query.DNSServers, queryValues, "dnsservers")
	if err != nil {
		return nil, nil, utils.GetBadRequestError("dnsservers", query.DNSServers, err)
	}

	dnsoptions, err := utils.ParseJSONOptionalSlice(query.DNSOptions, queryValues, "dnsoptions")
	if err != nil {
		return nil, nil, utils.GetBadRequestError("dnsoptions", query.DNSOptions, err)
	}

	dnssearch, err := utils.ParseJSONOptionalSlice(query.DNSSearch, queryValues, "dnssearch")
	if err != nil {
		return nil, nil, utils.GetBadRequestError("dnssearch", query.DNSSearch, err)
	}

	secrets, err := processSecrets(query, buildCtx.ContextDirectory, queryValues)
	if err != nil {
		return nil, nil, utils.GetBadRequestError("secrets", query.Secrets, err)
	}

	addhosts, err := utils.ParseJSONOptionalSlice(query.AddHosts, queryValues, "extrahosts")
	if err != nil {
		return nil, nil, utils.GetBadRequestError("extrahosts", query.AddHosts, err)
	}

	compatVolumes, _ := utils.ParseOptionalBool(query.CompatVolumes, "compatvolumes", queryValues)

	compressionIntent := archive.Compression(query.Compression)

	if query.StageLabels && !query.SaveStages {
		return nil, nil, utils.GetGenericBadRequestError(errors.New("stage-labels requires save-stages be set as well"))
	}

	// Process tags
	tags := query.Tags
	var output string
	var additionalTags []string
	if len(tags) > 0 {
		possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, tags[0])
		if err != nil {
			return nil, nil, utils.GetInternalServerError(fmt.Errorf("normalizing image: %w", err))
		}
		output = possiblyNormalizedName

		for i := 1; i < len(tags); i++ {
			possiblyNormalizedTag, err := utils.NormalizeToDockerHub(r, tags[i])
			if err != nil {
				return nil, nil, utils.GetInternalServerError(fmt.Errorf("normalizing image: %w", err))
			}
			additionalTags = append(additionalTags, possiblyNormalizedTag)
		}
	}

	// Process build format and isolation
	format := buildah.Dockerv2ImageManifest
	registry := query.Registry
	isolation := buildah.IsolationDefault
	if utils.IsLibpodRequest(r) {
		var err error
		isolation, err = parseLibPodIsolation(query.Isolation)
		if err != nil {
			return nil, nil, utils.GetInternalServerError(fmt.Errorf("failed to parse isolation: %w", err))
		}

		// Make sure to force rootless as rootless otherwise buildah runs code which is intended to be run only as root.
		// Same the other way around: https://github.com/containers/podman/issues/22109
		switch isolation {
		case buildah.IsolationOCI:
			if rootless.IsRootless() {
				isolation = buildah.IsolationOCIRootless
			}
		case buildah.IsolationOCIRootless:
			if !rootless.IsRootless() {
				isolation = buildah.IsolationOCI
			}
		}

		registry = ""
		format = query.OutputFormat
	} else {
		if _, found := queryValues["isolation"]; found {
			if query.Isolation != "" && query.Isolation != "default" {
				logrus.Debugf("invalid `isolation` parameter: %q", query.Isolation)
			}
		}
	}

	// Process IDMapping
	var idMappingOptions buildahDefine.IDMappingOptions
	if err := utils.ParseOptionalJSONField(query.IDMappingOptions, "idmappingoptions", queryValues, &idMappingOptions); err != nil {
		return nil, nil, utils.GetBadRequestError("idmappingoptions", query.IDMappingOptions, err)
	}

	// Process cache options
	cacheFrom, err := processCacheFrom(query, queryValues)
	if err != nil {
		return nil, nil, utils.GetBadRequestError("cachefrom", query.CacheFrom, err)
	}

	cacheTo, err := processCacheTo(query, queryValues)
	if err != nil {
		return nil, nil, utils.GetBadRequestError("cacheTo", query.CacheTo, err)
	}

	var cacheTTL time.Duration
	if _, found := queryValues["cachettl"]; found {
		cacheTTL, err = time.ParseDuration(query.CacheTTL)
		if err != nil {
			return nil, nil, utils.GetBadRequestError("cachettl", query.CacheTTL, err)
		}
	}

	// Process build args
	buildArgs := map[string]string{}
	if err := utils.ParseOptionalJSONField(query.BuildArgs, "buildargs", queryValues, &buildArgs); err != nil {
		return nil, nil, utils.GetBadRequestError("buildargs", query.BuildArgs, err)
	}

	// Process excludes
	excludes := []string{}
	if err := utils.ParseOptionalJSONField(query.Excludes, "excludes", queryValues, &excludes); err != nil {
		return nil, nil, utils.GetBadRequestError("excludes", query.Excludes, err)
	}

	// Process annotations
	annotations := []string{}
	if err := utils.ParseOptionalJSONField(query.Annotations, "annotations", queryValues, &annotations); err != nil {
		return nil, nil, utils.GetBadRequestError("annotations", query.Annotations, err)
	}

	// Process CPP flags
	cppflags := []string{}
	if err := utils.ParseOptionalJSONField(query.CPPFlags, "cppflags", queryValues, &cppflags); err != nil {
		return nil, nil, utils.GetBadRequestError("cppflags", query.CPPFlags, err)
	}

	// Process namespace options
	nsoptions := buildah.NamespaceOptions{}
	if _, found := queryValues["nsoptions"]; found {
		if err := utils.ParseOptionalJSONField(query.NamespaceOptions, "nsoptions", queryValues, &nsoptions); err != nil {
			return nil, nil, utils.GetBadRequestError("nsoptions", query.NamespaceOptions, err)
		}
	} else {
		nsoptions = append(nsoptions, buildah.NamespaceOption{
			Name: string(specs.NetworkNamespace),
			Host: true,
		})
	}

	// Process labels
	labels := []string{}
	if _, found := queryValues["labels"]; found {
		makeLabels := make(map[string]string)
		err := json.Unmarshal([]byte(query.Labels), &makeLabels)
		if err == nil {
			for k, v := range makeLabels {
				labels = append(labels, k+"="+v)
			}
		} else {
			if err := json.Unmarshal([]byte(query.Labels), &labels); err != nil {
				return nil, nil, utils.GetBadRequestError("labels", query.Labels, err)
			}
		}
	}

	jobs := 1
	if _, found := queryValues["jobs"]; found {
		jobs = query.Jobs
	}

	// Process security options
	var (
		labelOpts = []string{}
		seccomp   string
		apparmor  string
	)

	if utils.IsLibpodRequest(r) {
		seccomp = query.Seccomp
		apparmor = query.AppArmor
		// convert labelopts formats
		if err := utils.ParseOptionalJSONField(query.LabelOpts, "labelopts", queryValues, &labelOpts); err != nil {
			return nil, nil, utils.GetBadRequestError("labelopts", query.LabelOpts, err)
		}
	} else {
		// handle security-opt
		securityOpts := []string{}
		if err := utils.ParseOptionalJSONField(query.SecurityOpt, "securityopt", queryValues, &securityOpts); err != nil {
			return nil, nil, utils.GetBadRequestError("securityopt", query.SecurityOpt, err)
		}
		for _, opt := range securityOpts {
			if opt == "no-new-privileges" {
				return nil, nil, utils.GetBadRequestError("securityopt", opt, fmt.Errorf("no-new-privileges is not supported"))
			}
			name, value, hasValue := strings.Cut(opt, "=")
			if !hasValue {
				return nil, nil, utils.GetBadRequestError("securityopt", opt, fmt.Errorf("invalid --security-opt name=value pair: %q", opt))
			}

			switch name {
			case "label":
				labelOpts = append(labelOpts, value)
			case "apparmor":
				apparmor = value
			case "seccomp":
				seccomp = value
			default:
				return nil, nil, utils.GetBadRequestError("securityopt", opt, fmt.Errorf("invalid --security-opt 2: %q", opt))
			}
		}
	}

	// Process ulimits
	ulimits := []string{}
	if err := utils.ParseOptionalJSONField(query.Ulimits, "ulimits", queryValues, &ulimits); err != nil {
		return nil, nil, utils.GetBadRequestError("ulimits", query.Ulimits, err)
	}

	// Process pull policy
	pullPolicy := buildahDefine.PullIfMissing
	if utils.IsLibpodRequest(r) {
		pullPolicy = buildahDefine.PolicyMap[query.PullPolicy]
	} else {
		if _, found := queryValues["pull"]; found {
			if query.Pull {
				pullPolicy = buildahDefine.PullAlways
			}
		}
	}

	// Get authentication
	creds, authfile, err := auth.GetCredentials(r)
	if err != nil {
		// Credential value(s) not returned as their value is not human readable
		return nil, nil, utils.GetGenericBadRequestError(err)
	}

	var temporaryFiles []string
	cleanup := func() {
		auth.RemoveAuthfile(authfile)
		for _, temporaryFile := range temporaryFiles {
			os.Remove(temporaryFile)
		}
	}
	makeTemporaryFileWithContent := func(data []byte, pattern string) (string, error) {
		if pattern == "" {
			pattern = "podman-build-"
		}
		f, err := os.CreateTemp(parse.GetTempDir(), pattern)
		if err != nil {
			return "", err
		}
		filename := f.Name()
		temporaryFiles = append(temporaryFiles, filename)
		_, err = f.Write(data)
		err = errors.Join(err, f.Close())
		if err != nil {
			return "", err
		}
		return filename, nil
	}

	// Process from image
	fromImage := query.From
	if fromImage != "" {
		possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, fromImage)
		if err != nil {
			return nil, cleanup, utils.GetInternalServerError(fmt.Errorf("normalizing image: %w", err))
		}
		fromImage = possiblyNormalizedName
	}

	// Create system context
	systemContext := &types.SystemContext{
		AuthFilePath:     authfile,
		DockerAuthConfig: creds,
	}
	if err := utils.PossiblyEnforceDockerHub(r, systemContext); err != nil {
		return nil, cleanup, utils.GetInternalServerError(fmt.Errorf("checking to enforce DockerHub: %w", err))
	}

	skipUnusedStages, _ := utils.ParseOptionalBool(query.SkipUnusedStages, "skipunusedstages", queryValues)

	if _, found := queryValues["tlsVerify"]; found {
		systemContext.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
		systemContext.OCIInsecureSkipTLSVerify = !query.TLSVerify
		systemContext.DockerDaemonInsecureSkipTLSVerify = !query.TLSVerify
	}

	// Process retry delay
	retryDelay := 2 * time.Second
	if query.RetryDelay != "" {
		retryDelay, err = time.ParseDuration(query.RetryDelay)
		if err != nil {
			return nil, cleanup, utils.GetBadRequestError("retry-delay", query.RetryDelay, err)
		}
	}
	var sbomScanOptions []buildahDefine.SBOMScanOptions
	if query.ImageSBOM != "" ||
		query.SBOMOutput != "" ||
		query.ImageSBOMOutput != "" ||
		query.SBOMPURLOutput != "" ||
		query.ImageSBOMPURLOutput != "" ||
		query.SBOMCommands != "" ||
		query.SBOMMergeStrategy != "" {
		sbomScanOption := &buildahDefine.SBOMScanOptions{
			SBOMOutput:      query.SBOMOutput,
			PURLOutput:      query.SBOMPURLOutput,
			ImageSBOMOutput: query.ImageSBOMOutput,
			ImagePURLOutput: query.ImageSBOMPURLOutput,
			Image:           query.ImageSBOM,
			MergeStrategy:   buildahDefine.SBOMMergeStrategy(query.SBOMMergeStrategy),
			PullPolicy:      pullPolicy,
		}

		if _, found := r.URL.Query()["sbom-scanner-command"]; found {
			m := []string{}
			if err := json.Unmarshal([]byte(query.SBOMCommands), &m); err != nil {
				return nil, cleanup, utils.GetBadRequestError("sbom-scanner-command", query.SBOMCommands, err)
			}
			sbomScanOption.Commands = m
		}

		if !slices.Contains(sbomScanOption.ContextDir, buildCtx.ContextDirectory) {
			sbomScanOption.ContextDir = append(sbomScanOption.ContextDir, buildCtx.ContextDirectory)
		}

		for _, abc := range buildCtx.AdditionalBuildContexts {
			if !abc.IsURL && !abc.IsImage {
				sbomScanOption.ContextDir = append(sbomScanOption.ContextDir, abc.Value)
			}
		}

		sbomScanOptions = append(sbomScanOptions, *sbomScanOption)
	}

	var compressionFormat *compression.Algorithm
	if query.CompressionFormat != "" {
		algo, err := compression.AlgorithmByName(query.CompressionFormat)
		if err != nil {
			return nil, cleanup, utils.GetBadRequestError("compressionFormat", query.CompressionFormat, err)
		}
		compressionFormat = &algo
	}

	// Create build options
	buildOptions := &buildahDefine.BuildOptions{
		AddCapabilities:         addCaps,
		AdditionalBuildContexts: buildCtx.AdditionalBuildContexts,
		AdditionalTags:          additionalTags,
		Annotations:             annotations,
		CPPFlags:                cppflags,
		CacheFrom:               cacheFrom,
		CacheTo:                 cacheTo,
		CacheTTL:                cacheTTL,
		Args:                    buildArgs,
		AllPlatforms:            query.AllPlatforms,
		CommonBuildOpts: &buildah.CommonBuildOptions{
			AddHost:            addhosts,
			ApparmorProfile:    apparmor,
			CPUPeriod:          query.CpuPeriod,
			CPUQuota:           query.CpuQuota,
			CPUSetCPUs:         query.CpuSetCpus,
			CPUSetMems:         query.CpuSetMems,
			CPUShares:          query.CpuShares,
			CgroupParent:       query.CgroupParent,
			DNSOptions:         dnsoptions,
			DNSSearch:          dnssearch,
			DNSServers:         dnsservers,
			HTTPProxy:          query.HTTPProxy,
			IdentityLabel:      identityLabel,
			LabelOpts:          labelOpts,
			Memory:             query.Memory,
			MemorySwap:         query.MemSwap,
			NoHosts:            query.NoHosts,
			OmitHistory:        query.OmitHistory,
			SeccompProfilePath: seccomp,
			ShmSize:            strconv.Itoa(query.ShmSize),
			Ulimit:             ulimits,
			Secrets:            secrets,
			Volumes:            query.Volumes,
		},
		CompatVolumes:                  compatVolumes,
		CompressionFormat:              compressionFormat,
		CompressionLevel:               query.CompressionLevel,
		CreatedAnnotation:              query.CreatedAnnotation,
		Compression:                    compressionIntent,
		ConfigureNetwork:               parseNetworkConfigurationPolicy(query.ConfigureNetwork),
		ContextDirectory:               buildCtx.ContextDirectory,
		Devices:                        devices,
		DropCapabilities:               dropCaps,
		Envs:                           query.Envs,
		Excludes:                       excludes,
		ForceRmIntermediateCtrs:        query.ForceRm,
		GroupAdd:                       query.GroupAdd,
		From:                           fromImage,
		IDMappingOptions:               &idMappingOptions,
		IgnoreUnrecognizedInstructions: query.Ignore,
		IgnoreFile:                     buildCtx.IgnoreFile,
		InheritLabels:                  query.InheritLabels,
		InheritAnnotations:             query.InheritAnnotations,
		Isolation:                      isolation,
		Jobs:                           &jobs,
		Labels:                         labels,
		LayerLabels:                    query.LayerLabels,
		Layers:                         query.Layers,
		LogRusage:                      query.LogRusage,
		Manifest:                       query.Manifest,
		MaxPullPushRetries:             query.Retry,
		NamespaceOptions:               nsoptions,
		NoCache:                        query.NoCache,
		OSFeatures:                     query.OSFeatures,
		OSVersion:                      query.OSVersion,
		Output:                         output,
		OutputFormat:                   format,
		PullPolicy:                     pullPolicy,
		PullPushRetryDelay:             retryDelay,
		Quiet:                          query.Quiet,
		Registry:                       registry,
		RemoveIntermediateCtrs:         query.Rm,
		RewriteTimestamp:               query.RewriteTimestamp,
		RusageLogFile:                  query.RusageLogFile,
		SkipUnusedStages:               skipUnusedStages,
		SaveStages:                     query.SaveStages,
		Squash:                         query.Squash,
		StageLabels:                    query.StageLabels,
		SystemContext:                  systemContext,
		Target:                         query.Target,
		TransientRunMounts:             query.TransientRunMounts,
		UnsetEnvs:                      query.UnsetEnvs,
		UnsetLabels:                    query.UnsetLabels,
		UnsetAnnotations:               query.UnsetAnnotations,
		SBOMScanOptions:                sbomScanOptions,
	}

	// Process platforms
	platforms := query.Platform
	if len(platforms) == 1 {
		// Docker API uses comma separated platform arg so match this here
		platforms = strings.Split(query.Platform[0], ",")
	}
	for _, platformSpec := range platforms {
		os, arch, variant, err := parse.Platform(platformSpec)
		if err != nil {
			return nil, cleanup, utils.GetBadRequestError("platform", platformSpec, err)
		}
		buildOptions.Platforms = append(buildOptions.Platforms, struct{ OS, Arch, Variant string }{
			OS:      os,
			Arch:    arch,
			Variant: variant,
		})
	}

	// Process source policy
	if _, found := queryValues["sourcePolicy"]; found {
		filename, err := makeTemporaryFileWithContent([]byte(query.SourcePolicy), "podman-source-policy-")
		if err != nil {
			return nil, cleanup, utils.GetBadRequestError("sourcePolicy", query.SourcePolicy, err)
		}
		buildOptions.SourcePolicyFile = filename
	}

	// Process timestamps
	if _, found := queryValues["sourcedateepoch"]; found {
		ts := time.Unix(query.SourceDateEpoch, 0)
		buildOptions.SourceDateEpoch = &ts
	}
	if _, found := queryValues["timestamp"]; found {
		ts := time.Unix(query.Timestamp, 0)
		buildOptions.Timestamp = &ts
	}

	return buildOptions, cleanup, nil
}

// executeBuild performs the container build operation and streams results to the client.
func executeBuild(runtime *libpod.Runtime, w http.ResponseWriter, r *http.Request, buildOptions *buildahDefine.BuildOptions, containerFiles []string, query *BuildQuery) {
	// Channels all mux'ed in select{} below to follow API build protocol
	stdout := channel.NewWriter(make(chan []byte))
	defer stdout.Close()

	auxout := channel.NewWriter(make(chan []byte))
	defer auxout.Close()

	stderr := channel.NewWriter(make(chan []byte))
	defer stderr.Close()

	reporter := channel.NewWriter(make(chan []byte))
	defer reporter.Close()

	// Set output channels
	buildOptions.Err = auxout
	buildOptions.Out = stdout
	buildOptions.ReportWriter = reporter

	var (
		imageID string
		success bool
	)

	runCtx, cancel := context.WithCancel(r.Context())
	go func() {
		defer cancel()
		var err error
		imageID, _, err = runtime.Build(r.Context(), *buildOptions, containerFiles...)
		if err == nil {
			success = true
		} else {
			stderr.Write([]byte(err.Error() + "\n"))
		}
	}()

	// Send headers and prime client for stream to come
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	sender := utils.NewBuildResponseSender(w)
	var stepErrors []string

	for {
		select {
		case e := <-stdout.Chan():
			sender.SendBuildStream(string(e))
		case e := <-reporter.Chan():
			sender.SendBuildStream(string(e))
		case e := <-auxout.Chan():
			if !query.Quiet {
				sender.SendBuildStream(string(e))
			} else {
				stepErrors = append(stepErrors, string(e))
			}
		case e := <-stderr.Chan():
			// Docker-API Compat parity : Build failed so
			// output all step errors irrespective of quiet
			// flag.
			for _, stepError := range stepErrors {
				sender.SendBuildStream(stepError)
			}
			sender.SendBuildError(string(e))
			return
		case <-runCtx.Done():
			if success {
				if !utils.IsLibpodRequest(r) && !query.Quiet {
					sender.SendBuildAux(fmt.Appendf(nil, `{"ID":"sha256:%s"}`, imageID))
					sender.SendBuildStream(fmt.Sprintf("Successfully built %12.12s\n", imageID))
					for _, tag := range query.Tags {
						sender.SendBuildStream(fmt.Sprintf("Successfully tagged %s\n", tag))
					}
				}
			}
			return
		case <-r.Context().Done():
			cancel()
			logrus.Infof("Client disconnect reported for build %q / %q.", buildOptions.Registry, query.Dockerfile)
			return
		}
	}
}

// handleLocalBuildContexts processes build contexts for local API builds and validates local paths.
//
// This function handles the main build context and any additional build contexts specified in the request:
// - Validates that the main context directory (localcontextdir) exists and is accessible for local API usage
// - Processes additional build contexts which can be:
//   - URLs (url:) - downloads content to temporary directories under anchorDir
//   - Container images (image:) - records image references for later resolution during build
//   - Local paths (localpath:) - validates and cleans local filesystem paths
//
// Returns a BuildContext struct with the main context directory and a map of additional build contexts,
// or an error if validation fails or required parameters are missing.
func handleLocalBuildContexts(query url.Values, anchorDir string) (*BuildContext, error) {
	localContextDir := query.Get("localcontextdir")
	if localContextDir == "" {
		return nil, utils.GetBadRequestError("localcontextdir", localContextDir, fmt.Errorf("localcontextdir cannot be empty"))
	}
	localContextDir = filepath.Clean(localContextDir)
	if err := localapi.ValidatePathForLocalAPI(localContextDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, utils.GetFileNotFoundError(err)
		}
		return nil, utils.GetGenericBadRequestError(err)
	}

	out := &BuildContext{
		ContextDirectory:        localContextDir,
		AdditionalBuildContexts: make(map[string]*buildahDefine.AdditionalBuildContext),
	}

	for _, url := range query["additionalbuildcontexts"] {
		name, value, found := strings.Cut(url, "=")
		if !found {
			return nil, utils.GetInternalServerError(fmt.Errorf("additionalbuildcontexts must be in name=value format: %q", url))
		}

		logrus.Debugf("Processing additional build context: name=%q, value=%q", name, value)

		switch {
		case strings.HasPrefix(value, "url:"):
			value = strings.TrimPrefix(value, "url:")
			tempDir, subdir, err := buildahDefine.TempDirForURL(anchorDir, "buildah", value)
			if err != nil {
				return nil, utils.GetInternalServerError(genSpaceErr(err))
			}

			contextPath := filepath.Join(tempDir, subdir)
			out.AdditionalBuildContexts[name] = &buildahDefine.AdditionalBuildContext{
				IsURL:           true,
				IsImage:         false,
				Value:           contextPath,
				DownloadedCache: contextPath,
			}
		case strings.HasPrefix(value, "image:"):
			value = strings.TrimPrefix(value, "image:")
			out.AdditionalBuildContexts[name] = &buildahDefine.AdditionalBuildContext{
				IsURL:   false,
				IsImage: true,
				Value:   value,
			}
		case strings.HasPrefix(value, "localpath:"):
			value = strings.TrimPrefix(value, "localpath:")
			out.AdditionalBuildContexts[name] = &buildahDefine.AdditionalBuildContext{
				IsURL:   false,
				IsImage: false,
				Value:   filepath.Clean(value),
			}
		}
	}
	return out, nil
}

// getLocalBuildContext processes build contexts from Local API HTTP request to a BuildContext struct.
func getLocalBuildContext(r *http.Request, query url.Values, anchorDir string, _ bool) (*BuildContext, error) {
	// Handle build contexts
	buildContext, err := handleLocalBuildContexts(query, anchorDir)
	if err != nil {
		return nil, err
	}

	// Process build context and container files
	buildContext, err = processBuildContext(query, r, buildContext, anchorDir)
	if err != nil {
		return nil, err
	}

	if err := buildContext.validateLocalAPIPaths(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, utils.GetFileNotFoundError(err)
		}
		return nil, utils.GetGenericBadRequestError(err)
	}

	return buildContext, nil
}

// LocalBuildImage handles HTTP requests for building container images using the Local API.
//
// Uses localcontextdir and additionalbuildcontexts query parameters to specify build contexts
// from the server's local filesystem. All paths must be absolute and exist on the server.
// Processes build parameters, executes the build using buildah, and streams output to the client.
func LocalBuildImage(w http.ResponseWriter, r *http.Request) {
	buildImage(w, r, getLocalBuildContext)
}

// BuildImage handles HTTP requests for building container images using the Docker-compatible API.
//
// Extracts build contexts from the request body (tar/multipart), processes build parameters,
// executes the build using buildah, and streams output back to the client.
func BuildImage(w http.ResponseWriter, r *http.Request) {
	buildImage(w, r, getBuildContext)
}

type getBuildContextFunc func(r *http.Request, query url.Values, anchorDir string, multipart bool) (*BuildContext, error)

func buildImage(w http.ResponseWriter, r *http.Request, getBuildContextFunc getBuildContextFunc) {
	// Create temporary directory for build context
	anchorDir, err := os.MkdirTemp(parse.GetTempDir(), "libpod_builder")
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	defer func() {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			if v, found := os.LookupEnv("PODMAN_RETAIN_BUILD_ARTIFACT"); found {
				if keep, _ := strconv.ParseBool(v); keep {
					return
				}
			}
		}
		err := os.RemoveAll(anchorDir)
		if err != nil {
			logrus.Warn(fmt.Errorf("failed to remove build scratch directory %q: %w", anchorDir, err))
		}
	}()

	// If we have a multipart we use the operations, if not default extraction for main context
	// Validate content type
	multipart, err := utils.ValidateContentType(r)
	if err != nil {
		utils.ProcessBuildError(w, err)
		return
	}
	queryValues := r.URL.Query()

	buildContext, err := getBuildContextFunc(r, queryValues, anchorDir, multipart)
	if err != nil {
		utils.ProcessBuildError(w, err)
		return
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	conf, err := runtime.GetConfigNoCopy()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	query, err := parseBuildQuery(r, conf, queryValues)
	if err != nil {
		utils.ProcessBuildError(w, err)
		return
	}

	// Create build options
	buildOptions, cleanup, err := createBuildOptions(query, buildContext, queryValues, r)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		utils.ProcessBuildError(w, err)
		return
	}

	// Execute build
	executeBuild(runtime, w, r, buildOptions, buildContext.ContainerFiles, query)
}

// getBuildContext processes build contexts from HTTP request to a BuildContext struct.
func getBuildContext(r *http.Request, query url.Values, anchorDir string, multipart bool) (*BuildContext, error) {
	// Handle build contexts (extract from tar/multipart)
	buildContext, err := handleBuildContexts(r, query, anchorDir, multipart)
	if err != nil {
		return nil, utils.GetInternalServerError(genSpaceErr(err))
	}

	// Process build context and container files
	buildContext, err = processBuildContext(query, r, buildContext, anchorDir)
	if err != nil {
		return nil, err
	}

	return buildContext, nil
}

// handleBuildContexts extracts and processes build contexts from the HTTP request body.
// Supports both single-context builds and multi-context builds with named references.
func handleBuildContexts(r *http.Request, query url.Values, anchorDir string, multipart bool) (*BuildContext, error) {
	var err error
	out := &BuildContext{
		AdditionalBuildContexts: make(map[string]*buildahDefine.AdditionalBuildContext),
	}

	for _, url := range query["additionalbuildcontexts"] {
		name, value, found := strings.Cut(url, "=")
		if !found {
			return nil, fmt.Errorf("invalid additional build context format: %q", url)
		}

		logrus.Debugf("name: %q, context: %q", name, value)

		if urlValue, ok := strings.CutPrefix(value, "url:"); ok {
			tempDir, subdir, err := buildahDefine.TempDirForURL(anchorDir, "buildah", urlValue)
			if err != nil {
				return nil, fmt.Errorf("downloading URL %q: %w", name, err)
			}

			contextPath := filepath.Join(tempDir, subdir)
			out.AdditionalBuildContexts[name] = &buildahDefine.AdditionalBuildContext{
				IsURL:           true,
				IsImage:         false,
				Value:           contextPath,
				DownloadedCache: contextPath,
			}

			logrus.Debugf("Downloaded URL context %q to %q", name, contextPath)
		} else if imageValue, ok := strings.CutPrefix(value, "image:"); ok {
			out.AdditionalBuildContexts[name] = &buildahDefine.AdditionalBuildContext{
				IsURL:   false,
				IsImage: true,
				Value:   imageValue,
			}

			logrus.Debugf("Using image context %q: %q", name, imageValue)
		}
	}

	if !multipart {
		logrus.Debug("No multipart needed")
		out.ContextDirectory, err = extractTarFile(anchorDir, r.Body)
		if err != nil {
			return nil, err
		}
		return out, nil
	}

	logrus.Debug("Multipart is needed")
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart reader: %w", err)
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read multipart: %w", err)
		}
		defer part.Close()

		fieldName := part.FormName()

		if fieldName == "MainContext" {
			mainDir, err := extractTarFile(anchorDir, part)
			if err != nil {
				return nil, fmt.Errorf("extracting main context in multipart: %w", err)
			}
			if mainDir == "" {
				return nil, fmt.Errorf("main context directory is empty")
			}
			out.ContextDirectory = mainDir
		} else if contextName, ok := strings.CutPrefix(fieldName, "build-context-"); ok {
			// Create temp directory directly under anchorDir
			additionalAnchor, err := os.MkdirTemp(anchorDir, contextName+"-*")
			if err != nil {
				return nil, fmt.Errorf("creating temp directory for additional context %q: %w", contextName, err)
			}

			if err := chrootarchive.Untar(part, additionalAnchor, nil); err != nil {
				return nil, fmt.Errorf("extracting additional context %q: %w", contextName, err)
			}

			var latestModTime time.Time
			fileCount := 0
			walkErr := filepath.Walk(additionalAnchor, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				// Skip the root directory itself since it's always going to have the latest timestamp
				if path == additionalAnchor {
					return nil
				}
				if !info.IsDir() {
					fileCount++
				}
				// Use any extracted content timestamp (files or subdirectories)
				if info.ModTime().After(latestModTime) {
					latestModTime = info.ModTime()
				}
				return nil
			})
			if walkErr != nil {
				return nil, fmt.Errorf("error walking additional context: %w", walkErr)
			}

			// If we found any files, set the timestamp on the additional context directory
			// to the latest modified time found in the files.
			if !latestModTime.IsZero() {
				if err := os.Chtimes(additionalAnchor, latestModTime, latestModTime); err != nil {
					logrus.Warnf("Failed to set timestamp on additional context directory: %v", err)
				}
			}

			out.AdditionalBuildContexts[contextName] = &buildahDefine.AdditionalBuildContext{
				IsURL:   false,
				IsImage: false,
				Value:   additionalAnchor,
			}
		} else {
			logrus.Debugf("Ignoring unknown multipart field: %s", fieldName)
		}
	}

	return out, nil
}

func parseNetworkConfigurationPolicy(network string) buildah.NetworkConfigurationPolicy {
	if val, err := strconv.Atoi(network); err == nil {
		return buildah.NetworkConfigurationPolicy(val)
	}
	switch network {
	case "NetworkDefault":
		return buildah.NetworkDefault
	case "NetworkDisabled":
		return buildah.NetworkDisabled
	case "NetworkEnabled":
		return buildah.NetworkEnabled
	default:
		return buildah.NetworkDefault
	}
}

func parseLibPodIsolation(isolation string) (buildah.Isolation, error) {
	if val, err := strconv.Atoi(isolation); err == nil {
		return buildah.Isolation(val), nil
	}
	return parse.IsolationOption(isolation)
}

func extractTarFile(anchorDir string, r io.ReadCloser) (string, error) {
	buildDir := filepath.Join(anchorDir, "build")
	err := os.Mkdir(buildDir, 0o700)
	if err != nil {
		return "", err
	}

	err = chrootarchive.Untar(r, buildDir, nil)
	return buildDir, err
}
