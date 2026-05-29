package imagebuildah

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	encconfig "github.com/containers/ocicrypt/config"
	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerfile/parser"
	"github.com/sirupsen/logrus"
	"go.podman.io/buildah"
	"go.podman.io/buildah/define"
	"go.podman.io/buildah/internal"
	"go.podman.io/buildah/internal/metadata"
	internalUtil "go.podman.io/buildah/internal/util"
	"go.podman.io/buildah/pkg/parse"
	"go.podman.io/buildah/pkg/sourcepolicy"
	"go.podman.io/buildah/pkg/sshagent"
	"go.podman.io/buildah/util"
	"go.podman.io/common/libimage"
	nettypes "go.podman.io/common/libnetwork/types"
	"go.podman.io/common/pkg/config"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/pkg/compression"
	storageTransport "go.podman.io/image/v5/storage"
	"go.podman.io/image/v5/transports"
	"go.podman.io/image/v5/transports/alltransports"
	"go.podman.io/image/v5/types"
	"go.podman.io/storage"
	"go.podman.io/storage/pkg/archive"
	"golang.org/x/sync/semaphore"
)

// builtinAllowedBuildArgs is list of built-in allowed build args.  Normally we
// complain if we're given values for arguments which have no corresponding ARG
// instruction in the Dockerfile, since that's usually an indication of a user
// error, but for these values we make exceptions and ignore them.
var builtinAllowedBuildArgs = map[string]struct{}{
	"HTTP_PROXY":                 {},
	"http_proxy":                 {},
	"HTTPS_PROXY":                {},
	"https_proxy":                {},
	"FTP_PROXY":                  {},
	"ftp_proxy":                  {},
	"NO_PROXY":                   {},
	"no_proxy":                   {},
	"TARGETARCH":                 {},
	"TARGETOS":                   {},
	"TARGETPLATFORM":             {},
	"TARGETVARIANT":              {},
	internal.SourceDateEpochName: {},
}

// executor is a buildah-based implementation of the imagebuilder.Executor
// interface.  It coordinates the entire build by using one or more
// stageExecutors to handle each stage of the build.
type executor struct {
	cacheFrom                      []reference.Named
	cacheTo                        []reference.Named
	cacheTTL                       time.Duration
	containerSuffix                string
	logger                         *logrus.Logger
	stages                         map[string]*stageExecutor
	store                          storage.Store
	contextDir                     string
	contextDirWritesAreDiscarded   bool
	pullPolicy                     define.PullPolicy
	registry                       string
	ignoreUnrecognizedInstructions bool
	quiet                          bool
	runtime                        string
	runtimeArgs                    []string
	transientMounts                []Mount
	transientRunMounts             []string
	compression                    archive.Compression
	compressionFormat              *compression.Algorithm
	compressionLevel               *int
	forceCompressionFormat         bool
	output                         string
	outputFormat                   string
	additionalTags                 []string
	log                            func(format string, args ...any) // can be nil
	in                             io.Reader
	inheritLabels                  types.OptionalBool
	inheritAnnotations             types.OptionalBool
	out                            io.Writer
	err                            io.Writer
	signaturePolicyPath            string
	sourcePolicy                   *sourcepolicy.Policy
	skipUnusedStages               types.OptionalBool
	systemContext                  *types.SystemContext
	reportWriter                   io.Writer
	isolation                      define.Isolation
	namespaceOptions               []define.NamespaceOption
	configureNetwork               define.NetworkConfigurationPolicy
	// networkInterface is the libnetwork network interface used to setup netavark networks.
	networkInterface                        nettypes.ContainerNetwork
	idmappingOptions                        *define.IDMappingOptions
	commonBuildOptions                      *define.CommonBuildOptions
	defaultMountsFilePath                   string
	iidfile                                 string
	iidfileRaw                              string
	squash                                  bool
	labels                                  []string
	layerLabels                             []string
	annotations                             []string
	layers                                  bool
	saveStages                              bool
	stageLabels                             bool
	stageImageIDs                           map[string]string // Tracks image IDs for each stage (by name and position) for label references
	noHostname                              bool
	noHosts                                 bool
	useCache                                bool
	removeIntermediateCtrs                  bool
	forceRmIntermediateCtrs                 bool
	imageMap                                map[string]string           // Used to map images that we create to handle the AS construct (stage name to image ID).
	imageDigestMap                          map[string]string           // Used to map images that we create to handle the AS construct (stage name to image (manifest) digest).
	containerMap                            map[string]*buildah.Builder // Used to map from image names to only-created-for-the-rootfs containers.
	baseMap                                 map[string]struct{}         // Holds the names of every base image, as given.
	rootfsMap                               map[string]struct{}         // Holds the names of every stage whose rootfs is referenced in a COPY or ADD instruction.
	afterDependency                         map[string]string           // Maps stage names to their --after dependency.
	blobDirectory                           string
	excludes                                []string
	groupAdd                                []string
	ignoreFile                              string
	args                                    map[string]string
	globalArgs                              map[string]string
	unusedArgs                              map[string]struct{}
	capabilities                            []string
	devices                                 define.ContainerDevices
	deviceSpecs                             []string
	signBy                                  string
	architecture                            string
	timestamp                               *time.Time
	os                                      string
	maxPullPushRetries                      int
	retryPullPushDelay                      time.Duration
	cachePullSourceLookupReferenceFunc      libimage.LookupReferenceFunc
	cachePullDestinationLookupReferenceFunc func(srcRef types.ImageReference) libimage.LookupReferenceFunc
	cachePushSourceLookupReferenceFunc      func(dest types.ImageReference) libimage.LookupReferenceFunc
	cachePushDestinationLookupReferenceFunc libimage.LookupReferenceFunc
	ociDecryptConfig                        *encconfig.DecryptConfig
	lastError                               error
	terminatedStage                         map[string]error
	stagesLock                              sync.Mutex
	stagesSemaphore                         *semaphore.Weighted
	logRusage                               bool
	rusageLogFile                           io.Writer
	imageInfoLock                           sync.Mutex
	imageInfoCache                          map[string]imageTypeAndHistoryAndDiffIDs
	fromOverride                            string
	additionalBuildContexts                 map[string]*define.AdditionalBuildContext
	manifest                                string
	secrets                                 map[string]define.Secret
	sshsources                              map[string]*sshagent.Source
	logPrefix                               string
	unsetEnvs                               []string
	unsetLabels                             []string
	unsetAnnotations                        []string
	processLabel                            string   // Shares processLabel of first stage container with containers of other stages in same build
	mountLabel                              string   // Shares mountLabel of first stage container with containers of other stages in same build
	buildOutputs                            []string // Specifies instructions for any custom build output
	osVersion                               string
	osFeatures                              []string
	envs                                    []string
	confidentialWorkload                    define.ConfidentialWorkloadOptions
	sbomScanOptions                         []define.SBOMScanOptions
	cdiConfigDir                            string
	compatSetParent                         types.OptionalBool
	compatVolumes                           types.OptionalBool
	compatScratchConfig                     types.OptionalBool
	compatLayerOmissions                    types.OptionalBool
	noPivotRoot                             bool
	sourceDateEpoch                         *time.Time
	rewriteTimestamp                        bool
	createdAnnotation                       types.OptionalBool
	metadataFile                            string
}

type imageTypeAndHistoryAndDiffIDs struct {
	manifestType string
	history      []v1.History
	diffIDs      []digest.Digest
	err          error
	architecture string
	os           string
}

// newExecutor creates a new instance of the imagebuilder.Executor interface.
func newExecutor(logger *logrus.Logger, logPrefix string, store storage.Store, options define.BuildOptions, mainNode *parser.Node, containerFiles []string, processLabel, mountLabel string, contextWritesDiscarded bool) (*executor, error) {
	defaultContainerConfig, err := config.Default()
	if err != nil {
		return nil, fmt.Errorf("failed to get container config: %w", err)
	}

	excludes := options.Excludes
	if len(excludes) == 0 {
		excludes, options.IgnoreFile, err = parse.ContainerIgnoreFile(options.ContextDirectory, options.IgnoreFile, containerFiles)
		if err != nil {
			return nil, err
		}
	}
	capabilities, err := defaultContainerConfig.Capabilities("", options.AddCapabilities, options.DropCapabilities)
	if err != nil {
		return nil, err
	}

	var transientMounts []Mount

	for _, volume := range append(defaultContainerConfig.Volumes(), options.TransientMounts...) {
		mount, err := parse.Volume(volume)
		if err != nil {
			return nil, err
		}
		transientMounts = append([]Mount{mount}, transientMounts...)
	}

	secrets, err := parse.Secrets(options.CommonBuildOpts.Secrets)
	if err != nil {
		return nil, err
	}
	sshsources, err := parse.SSH(options.CommonBuildOpts.SSHSources)
	if err != nil {
		return nil, err
	}

	// Load source policy if specified
	var srcPolicy *sourcepolicy.Policy
	if options.SourcePolicyFile != "" {
		srcPolicy, err = sourcepolicy.LoadFromFile(options.SourcePolicyFile)
		if err != nil {
			return nil, fmt.Errorf("loading source policy: %w", err)
		}
	}

	writer := options.ReportWriter
	if options.Quiet {
		writer = io.Discard
	}

	var rusageLogFile io.Writer

	if options.LogRusage && !options.Quiet {
		if options.RusageLogFile == "" {
			rusageLogFile = options.Out
		} else {
			rusageLogFile, err = os.OpenFile(options.RusageLogFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return nil, fmt.Errorf("creating file to store rusage logs: %w", err)
			}
		}
	}

	buildOutputs := slices.Clone(options.BuildOutputs)
	if options.BuildOutput != "" { //nolint:staticcheck
		buildOutputs = append(buildOutputs, options.BuildOutput) //nolint:staticcheck
	}

	exec := executor{
		args:                                    options.Args,
		cacheFrom:                               options.CacheFrom,
		cacheTo:                                 options.CacheTo,
		cacheTTL:                                options.CacheTTL,
		containerSuffix:                         options.ContainerSuffix,
		logger:                                  logger,
		stages:                                  make(map[string]*stageExecutor),
		store:                                   store,
		contextDir:                              options.ContextDirectory,
		contextDirWritesAreDiscarded:            contextWritesDiscarded,
		excludes:                                excludes,
		groupAdd:                                options.GroupAdd,
		ignoreFile:                              options.IgnoreFile,
		pullPolicy:                              options.PullPolicy,
		registry:                                options.Registry,
		ignoreUnrecognizedInstructions:          options.IgnoreUnrecognizedInstructions,
		quiet:                                   options.Quiet,
		runtime:                                 options.Runtime,
		runtimeArgs:                             options.RuntimeArgs,
		transientMounts:                         transientMounts,
		transientRunMounts:                      options.TransientRunMounts,
		compression:                             options.Compression,
		compressionFormat:                       options.CompressionFormat,
		compressionLevel:                        options.CompressionLevel,
		forceCompressionFormat:                  options.ForceCompressionFormat,
		output:                                  options.Output,
		outputFormat:                            options.OutputFormat,
		additionalTags:                          options.AdditionalTags,
		signaturePolicyPath:                     options.SignaturePolicyPath,
		sourcePolicy:                            srcPolicy,
		skipUnusedStages:                        options.SkipUnusedStages,
		systemContext:                           options.SystemContext,
		log:                                     options.Log,
		in:                                      options.In,
		out:                                     options.Out,
		err:                                     options.Err,
		reportWriter:                            writer,
		isolation:                               options.Isolation,
		inheritLabels:                           options.InheritLabels,
		inheritAnnotations:                      options.InheritAnnotations,
		namespaceOptions:                        options.NamespaceOptions,
		configureNetwork:                        options.ConfigureNetwork,
		networkInterface:                        options.NetworkInterface,
		idmappingOptions:                        options.IDMappingOptions,
		commonBuildOptions:                      options.CommonBuildOpts,
		defaultMountsFilePath:                   options.DefaultMountsFilePath,
		iidfile:                                 options.IIDFile,
		iidfileRaw:                              options.IIDFileRaw,
		squash:                                  options.Squash,
		labels:                                  slices.Clone(options.Labels),
		layerLabels:                             slices.Clone(options.LayerLabels),
		processLabel:                            processLabel,
		mountLabel:                              mountLabel,
		annotations:                             slices.Clone(options.Annotations),
		layers:                                  options.Layers,
		saveStages:                              options.SaveStages,
		stageLabels:                             options.StageLabels,
		stageImageIDs:                           make(map[string]string),
		noHostname:                              options.CommonBuildOpts.NoHostname,
		noHosts:                                 options.CommonBuildOpts.NoHosts,
		useCache:                                !options.NoCache,
		removeIntermediateCtrs:                  options.RemoveIntermediateCtrs,
		forceRmIntermediateCtrs:                 options.ForceRmIntermediateCtrs,
		imageMap:                                make(map[string]string),
		imageDigestMap:                          make(map[string]string),
		containerMap:                            make(map[string]*buildah.Builder),
		baseMap:                                 make(map[string]struct{}),
		rootfsMap:                               make(map[string]struct{}),
		blobDirectory:                           options.BlobDirectory,
		unusedArgs:                              make(map[string]struct{}),
		capabilities:                            capabilities,
		deviceSpecs:                             options.Devices,
		signBy:                                  options.SignBy,
		architecture:                            options.Architecture,
		timestamp:                               options.Timestamp,
		os:                                      options.OS,
		maxPullPushRetries:                      options.MaxPullPushRetries,
		retryPullPushDelay:                      options.PullPushRetryDelay,
		cachePullSourceLookupReferenceFunc:      options.CachePullSourceLookupReferenceFunc,
		cachePullDestinationLookupReferenceFunc: options.CachePullDestinationLookupReferenceFunc,
		cachePushSourceLookupReferenceFunc:      options.CachePushSourceLookupReferenceFunc,
		cachePushDestinationLookupReferenceFunc: options.CachePushDestinationLookupReferenceFunc,
		ociDecryptConfig:                        options.OciDecryptConfig,
		terminatedStage:                         make(map[string]error),
		stagesSemaphore:                         options.JobSemaphore,
		logRusage:                               options.LogRusage,
		rusageLogFile:                           rusageLogFile,
		imageInfoCache:                          make(map[string]imageTypeAndHistoryAndDiffIDs),
		fromOverride:                            options.From,
		additionalBuildContexts:                 options.AdditionalBuildContexts,
		manifest:                                options.Manifest,
		secrets:                                 secrets,
		sshsources:                              sshsources,
		logPrefix:                               logPrefix,
		unsetEnvs:                               slices.Clone(options.UnsetEnvs),
		unsetLabels:                             slices.Clone(options.UnsetLabels),
		unsetAnnotations:                        slices.Clone(options.UnsetAnnotations),
		buildOutputs:                            buildOutputs,
		osVersion:                               options.OSVersion,
		osFeatures:                              slices.Clone(options.OSFeatures),
		envs:                                    slices.Clone(options.Envs),
		confidentialWorkload:                    options.ConfidentialWorkload,
		sbomScanOptions:                         options.SBOMScanOptions,
		cdiConfigDir:                            options.CDIConfigDir,
		compatSetParent:                         options.CompatSetParent,
		compatVolumes:                           options.CompatVolumes,
		compatScratchConfig:                     options.CompatScratchConfig,
		compatLayerOmissions:                    options.CompatLayerOmissions,
		noPivotRoot:                             options.NoPivotRoot,
		sourceDateEpoch:                         options.SourceDateEpoch,
		rewriteTimestamp:                        options.RewriteTimestamp,
		createdAnnotation:                       options.CreatedAnnotation,
		metadataFile:                            options.MetadataFile,
	}
	// sort unsetAnnotations because we will later write these
	// values to the history of the image therefore we want to
	// make sure that order is always consistent.
	slices.Sort(exec.unsetAnnotations)

	if exec.err == nil {
		exec.err = os.Stderr
	}
	if exec.out == nil {
		exec.out = os.Stdout
	}

	for arg := range options.Args {
		if _, isBuiltIn := builtinAllowedBuildArgs[arg]; !isBuiltIn {
			exec.unusedArgs[arg] = struct{}{}
		}
	}
	// Use this flag to collect all args declared before
	// first stage and treat them as global args which is
	// accessible to all stages.
	foundFirstStage := false
	globalArgs := make(map[string]string)
	for _, line := range mainNode.Children {
		node := line
		for node != nil { // tokens on this line, though we only care about the first
			switch strings.ToUpper(node.Value) { // first token - instruction
			case "ARG":
				for arg := node.Next; arg != nil; arg = arg.Next {
					// Each token is name=value or name (multiple ARGs in one instruction, see imagebuilder PR #210)
					argName, argValue, hasValue := strings.Cut(arg.Value, "=")
					if argName == "" {
						continue
					}
					if !foundFirstStage {
						if hasValue {
							globalArgs[argName] = argValue
						}
					}
					delete(exec.unusedArgs, argName)
				}
			case "FROM":
				foundFirstStage = true
			}
			break
		}
	}
	exec.globalArgs = globalArgs
	return &exec, nil
}

// startStage creates a new stage executor that will be referenced whenever a
// COPY or ADD statement uses a --from=NAME flag.
func (b *executor) startStage(ctx context.Context, stage *imagebuilder.Stage, stages imagebuilder.Stages, output string) *stageExecutor {
	// create a copy of systemContext for each stage executor.
	systemContext := *b.systemContext
	stageExec := &stageExecutor{
		ctx:             ctx,
		executor:        b,
		systemContext:   &systemContext,
		log:             b.log,
		index:           stage.Position,
		stages:          stages,
		name:            stage.Name,
		volumeCache:     make(map[string]string),
		volumeCacheInfo: make(map[string]os.FileInfo),
		output:          output,
		stage:           stage,
	}
	b.stages[stage.Name] = stageExec
	if idx := strconv.Itoa(stage.Position); idx != stage.Name {
		b.stages[idx] = stageExec
	}
	return stageExec
}

// resolveNameToImageRef creates a types.ImageReference for the output name in local storage
func (b *executor) resolveNameToImageRef(output string) (types.ImageReference, error) {
	if imageRef, err := alltransports.ParseImageName(output); err == nil {
		return imageRef, nil
	}
	resolved, err := libimage.NormalizeName(output)
	if err != nil {
		return nil, err
	}
	imageRef, err := storageTransport.Transport.ParseStoreReference(b.store, resolved.String())
	if err == nil {
		return imageRef, nil
	}

	return imageRef, err
}

// waitForStage waits for an entry to be added to terminatedStage indicating
// that the specified stage has finished.  If there is no stage defined by that
// name, then it will return (false, nil).  If there is a stage defined by that
// name, it will return true along with any error it encounters.
func (b *executor) waitForStage(ctx context.Context, name string, stages imagebuilder.Stages) (bool, error) {
	found := false
	for _, otherStage := range stages {
		if otherStage.Name == name || strconv.Itoa(otherStage.Position) == name {
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	for {
		if b.lastError != nil {
			return true, b.lastError
		}

		b.stagesLock.Lock()
		terminationError, terminated := b.terminatedStage[name]
		b.stagesLock.Unlock()

		if terminationError != nil {
			return false, terminationError
		}
		if terminated {
			return true, nil
		}

		b.stagesSemaphore.Release(1)
		time.Sleep(time.Millisecond * 10)
		if err := b.stagesSemaphore.Acquire(ctx, 1); err != nil {
			return true, fmt.Errorf("reacquiring job semaphore: %w", err)
		}
	}
}

// getImageTypeAndHistoryAndDiffIDs returns the os, architecture, manifest type, history, and diff IDs list of imageID.
func (b *executor) getImageTypeAndHistoryAndDiffIDs(ctx context.Context, imageID string) (string, string, string, []v1.History, []digest.Digest, error) {
	b.imageInfoLock.Lock()
	imageInfo, ok := b.imageInfoCache[imageID]
	b.imageInfoLock.Unlock()
	if ok {
		return imageInfo.os, imageInfo.architecture, imageInfo.manifestType, imageInfo.history, imageInfo.diffIDs, imageInfo.err
	}
	imageRef, err := storageTransport.Transport.ParseStoreReference(b.store, "@"+imageID)
	if err != nil {
		return "", "", "", nil, nil, fmt.Errorf("getting image reference %q: %w", imageID, err)
	}
	ref, err := imageRef.NewImage(ctx, nil)
	if err != nil {
		return "", "", "", nil, nil, fmt.Errorf("creating new image from reference to image %q: %w", imageID, err)
	}
	defer ref.Close()
	oci, err := ref.OCIConfig(ctx)
	if err != nil {
		return "", "", "", nil, nil, fmt.Errorf("getting possibly-converted OCI config of image %q: %w", imageID, err)
	}
	manifestBytes, manifestFormat, err := ref.Manifest(ctx)
	if err != nil {
		return "", "", "", nil, nil, fmt.Errorf("getting manifest of image %q: %w", imageID, err)
	}
	if manifestFormat == "" && len(manifestBytes) > 0 {
		manifestFormat = manifest.GuessMIMEType(manifestBytes)
	}
	b.imageInfoLock.Lock()
	b.imageInfoCache[imageID] = imageTypeAndHistoryAndDiffIDs{
		manifestType: manifestFormat,
		history:      oci.History,
		diffIDs:      oci.RootFS.DiffIDs,
		err:          nil,
		architecture: oci.Architecture,
		os:           oci.OS,
	}
	b.imageInfoLock.Unlock()
	return oci.OS, oci.Architecture, manifestFormat, oci.History, oci.RootFS.DiffIDs, nil
}

func (b *executor) buildStage(ctx context.Context, cleanupStages map[int]*stageExecutor, stages imagebuilder.Stages, stageIndex int) (imageID string, commitResults *buildah.CommitResults, onlyBaseImage bool, err error) {
	var prependInstructions, appendInstructions []string
	stage := stages[stageIndex]
	ib := stage.Builder
	node := stage.Node

	// Wait for any --after deps before ib.From(node) which may try to access images
	// via local transports (like oci-archive:) populated by those deps.
	if afterDep, ok := b.afterDependency[stage.Name]; ok {
		logrus.Debugf("stage %d (%s): waiting for --after dependency %q", stageIndex, stage.Name, afterDep)
		if isStage, err := b.waitForStage(ctx, afterDep, stages[:stageIndex]); isStage && err != nil {
			return "", nil, false, fmt.Errorf("waiting for --after=%s: %w", afterDep, err)
		}
	}

	base, err := ib.From(node)
	if err != nil {
		logrus.Debugf("buildStage(node.Children=%#v)", node.Children)
		return "", nil, false, err
	}

	// If this is the last stage, then the image that we produce at
	// its end should be given the desired output name.
	output := ""
	if stageIndex == len(stages)-1 {
		output = b.output
		// Check if any labels were passed in via the API, and add an instruction at
		// the end of the Dockerfile stage that would provide the intended result.
		// Reason: Docker adds label modification as a last step which can be
		// processed like regular steps, and if no modification is done to
		// layers, its easier to reuse cached layers.
		if len(b.labels) > 0 {
			labelLine := "LABEL"
			for _, labelSpec := range b.labels {
				key, value, _ := strings.Cut(labelSpec, "=")
				// check only for an empty key since docker allows empty values
				if key != "" {
					labelLine += fmt.Sprintf(" %q=%q", key, value)
				}
			}
			appendInstructions = slices.Concat(appendInstructions, []string{labelLine})
		}
	}

	// If we were given environment variables to set via the API, add them as instructions
	// at the beginning of the stage so that they affect subsequent RUN instructions and
	// factor into the image history.
	if len(b.envs) > 0 {
		envLine := "ENV"
		for _, envSpec := range b.envs {
			key, value, hasValue := strings.Cut(envSpec, "=")
			if hasValue {
				envLine += fmt.Sprintf(" %q=%q", key, value)
			} else {
				return "", nil, false, fmt.Errorf("BUG: unresolved environment variable: %q", key)
			}
		}
		prependInstructions = slices.Concat([]string{envLine}, prependInstructions)
	}

	// Create stage labels for all stage images including final stage
	// Skip if stage has no instructions
	if b.saveStages && b.stageLabels && len(stage.Node.Children) > 0 {
		// Wait for base stage if it references a previous stage
		// This ensures stageImageIDs map is populated before buildStageLabelLine reads it
		if isStage, err := b.waitForStage(ctx, base, stages[:stageIndex]); isStage && err != nil {
			return "", nil, false, fmt.Errorf("waiting for base stage %s: %w", base, err)
		}

		labelLine := b.buildStageLabelLine(&stage, base, stages[:stageIndex])
		prependInstructions = slices.Concat([]string{labelLine}, prependInstructions)
	}

	// If we're supposed to be appending or prepending instructions to this stage, add them now.
	if len(prependInstructions) > 0 {
		addLines := strings.Join(prependInstructions, "\n")
		additionalNodes, err := imagebuilder.ParseDockerfile(strings.NewReader(addLines))
		if err != nil {
			return "", nil, false, fmt.Errorf("while adding additional steps %q: %w", prependInstructions, err)
		}
		stage.Node.Children = append(additionalNodes.Children, stage.Node.Children...)
	}
	if len(appendInstructions) > 0 {
		addLines := strings.Join(appendInstructions, "\n")
		additionalNodes, err := imagebuilder.ParseDockerfile(strings.NewReader(addLines))
		if err != nil {
			return "", nil, false, fmt.Errorf("while adding additional steps %q: %w", appendInstructions, err)
		}
		stage.Node.Children = append(stage.Node.Children, additionalNodes.Children...)
	}

	b.stagesLock.Lock()
	stageExecutor := b.startStage(ctx, &stage, stages, output)
	if stageExecutor.log == nil {
		stepCounter := 0
		stageExecutor.log = func(format string, args ...any) {
			prefix := b.logPrefix
			if len(stages) > 1 {
				prefix += fmt.Sprintf("[%d/%d] ", stageIndex+1, len(stages))
			}
			if !strings.HasPrefix(format, "COMMIT") {
				stepCounter++
				prefix += fmt.Sprintf("STEP %d", stepCounter)
				if stepCounter <= len(stage.Node.Children)+1 {
					prefix += fmt.Sprintf("/%d", len(stage.Node.Children)+1)
				}
				prefix += ": "
			}
			suffix := "\n"
			fmt.Fprintf(stageExecutor.executor.out, prefix+format+suffix, args...)
		}
	}
	b.stagesLock.Unlock()

	// If this a single-layer build, or if it's a multi-layered
	// build and b.forceRmIntermediateCtrs is set, make sure we
	// remove the intermediate/build containers, regardless of
	// whether or not the stage's build fails.
	if b.forceRmIntermediateCtrs || !b.layers {
		b.stagesLock.Lock()
		cleanupStages[stage.Position] = stageExecutor
		b.stagesLock.Unlock()
	}

	// Build this stage.
	if imageID, commitResults, onlyBaseImage, err = stageExecutor.execute(ctx, base); err != nil {
		return "", nil, onlyBaseImage, err
	}

	// Store image ID for this stage so subsequent stages can reference it in labels
	if imageID != "" {
		b.stagesLock.Lock()
		if stage.Name != "" {
			b.stageImageIDs[stage.Name] = imageID
		}
		b.stageImageIDs[strconv.Itoa(stageIndex)] = imageID
		b.stagesLock.Unlock()
	}

	// The stage succeeded, so remove its build container if we're
	// told to delete successful intermediate/build containers for
	// multi-layered builds.
	// Skip cleanup if the stage has no instructions.
	if b.removeIntermediateCtrs && len(stage.Node.Children) > 0 {
		b.stagesLock.Lock()
		cleanupStages[stage.Position] = stageExecutor
		b.stagesLock.Unlock()
	}

	return imageID, commitResults, onlyBaseImage, nil
}

type stageDependencyInfo struct {
	Name           string
	Position       int
	Needs          []string
	NeededByTarget bool
}

// Marks `NeededByTarget` as true for the given stage and all its dependency stages as true recursively.
func markDependencyStagesForTarget(dependencyMap map[string]*stageDependencyInfo, stage string) {
	if stageDependencyInfo, ok := dependencyMap[stage]; ok {
		if !stageDependencyInfo.NeededByTarget {
			stageDependencyInfo.NeededByTarget = true
			for _, need := range stageDependencyInfo.Needs {
				markDependencyStagesForTarget(dependencyMap, need)
			}
		}
	}
}

// buildStageLabelLine creates a LABEL instruction line for stage metadata
func (b *executor) buildStageLabelLine(stage *imagebuilder.Stage, base string, stages imagebuilder.Stages) string {
	labelLine := "LABEL"
	labelLine += fmt.Sprintf(" %q=%q", "io.buildah.stage.name", stage.Name)
	// Check if base of the stage is another (previous) stage.
	// If yes, base is set as image ID of this stage.
	// If not original base name is set (pullspec).
	for i, st := range stages {
		if st.Name == base || strconv.Itoa(st.Position) == base {
			b.stagesLock.Lock()
			if imgID, ok := b.stageImageIDs[base]; ok {
				base = imgID
			} else if imgID, ok := b.stageImageIDs[strconv.Itoa(i)]; ok {
				base = imgID
			}
			b.stagesLock.Unlock()
			break
		}
	}
	labelLine += fmt.Sprintf(" %q=%q", "io.buildah.stage.base", base)
	return labelLine
}

func (b *executor) warnOnUnsetBuildArgs(stages imagebuilder.Stages, dependencyMap map[string]*stageDependencyInfo, args map[string]string) {
	argFound := make(map[string]struct{})
	for _, stage := range stages {
		node := stage.Node // first line
		for node != nil {  // each line
			for _, child := range node.Children {
				switch strings.ToUpper(child.Value) {
				case "ARG":
					for arg := child.Next; arg != nil; arg = arg.Next {
						argToken := arg.Value
						argName, argValue, hasEqual := strings.Cut(argToken, "=")
						if argName == "" {
							continue
						}
						if hasEqual && argValue != "" {
							argFound[argName] = struct{}{}
						}
						argHasValue := internalUtil.SetHas(argFound, argName)
						if _, ok := args[argName]; !argHasValue && !ok {
							shouldWarn := true
							if stageDependencyInfo, ok := dependencyMap[stage.Name]; ok {
								if !stageDependencyInfo.NeededByTarget && b.skipUnusedStages != types.OptionalBoolFalse {
									shouldWarn = false
								}
							}
							if _, isBuiltIn := builtinAllowedBuildArgs[argName]; isBuiltIn {
								shouldWarn = false
							}
							if _, isGlobalArg := b.globalArgs[argName]; isGlobalArg {
								shouldWarn = false
							}
							if shouldWarn {
								b.logger.Warnf("missing %q build argument. Try adding %q to the command line", argName, fmt.Sprintf("--build-arg %s=<VALUE>", argName))
							}
						}
					}
				default:
					continue
				}
			}
			node = node.Next
		}
	}
}

// Build takes care of the details of running Prepare/Execute/Commit/Delete
// over each of the one or more parsed Dockerfiles and stages.
func (b *executor) Build(ctx context.Context, stages imagebuilder.Stages) (imageID string, ref reference.Canonical, err error) {
	if len(stages) == 0 {
		return "", nil, errors.New("building: no stages to build")
	}
	var cleanupImages []string
	cleanupStages := make(map[int]*stageExecutor)

	stdout := b.out
	if b.quiet {
		b.out = io.Discard
	}

	cleanup := func() error {
		var lastErr error
		// Clean up any containers associated with the final container
		// built by a stage, for stages that succeeded, since we no
		// longer need their filesystem contents.

		b.stagesLock.Lock()
		for _, stage := range cleanupStages {
			if err := stage.Delete(); err != nil {
				logrus.Debugf("Failed to cleanup stage containers: %v", err)
				lastErr = err
			}
		}
		cleanupStages = nil
		b.stagesLock.Unlock()

		// Clean up any builders that we used to get data from images.
		for _, builder := range b.containerMap {
			if err := builder.Delete(); err != nil {
				logrus.Debugf("Failed to cleanup image containers: %v", err)
				lastErr = err
			}
		}
		b.containerMap = nil
		// Clean up any intermediate containers associated with stages,
		// since we're not keeping them for debugging.
		if b.removeIntermediateCtrs {
			if err := b.deleteSuccessfulIntermediateCtrs(); err != nil {
				logrus.Debugf("Failed to cleanup intermediate containers: %v", err)
				lastErr = err
			}
		}
		// Remove images from stages except the last one, since we're
		// not going to use them as a starting point for any new
		// stages.
		for i := range cleanupImages {
			removeID := cleanupImages[len(cleanupImages)-i-1]
			if removeID == imageID {
				continue
			}
			if _, err := b.store.DeleteImage(removeID, true); err != nil {
				logrus.Debugf("failed to remove intermediate image %q: %v", removeID, err)
				if b.forceRmIntermediateCtrs || !errors.Is(err, storage.ErrImageUsedByContainer) {
					lastErr = err
				}
			}
		}
		cleanupImages = nil

		if b.rusageLogFile != nil && b.rusageLogFile != b.out {
			// we deliberately ignore the error here, as this
			// function can be called multiple times
			if closer, ok := b.rusageLogFile.(interface{ Close() error }); ok {
				closer.Close()
			}
		}
		return lastErr
	}

	defer func() {
		if cleanupErr := cleanup(); cleanupErr != nil {
			if err == nil {
				err = cleanupErr
			} else {
				err = fmt.Errorf("%v: %w", cleanupErr.Error(), err)
			}
		}
	}()

	// dependencyMap contains dependencyInfo for each stage,
	// dependencyInfo is used later to mark if a particular
	// stage is needed by target or not.
	dependencyMap := make(map[string]*stageDependencyInfo)
	// Initialize afterDependency map to track --after= dependency per stage
	b.afterDependency = make(map[string]string)
	// Build maps of every named base image and every referenced stage root
	// filesystem.  Individual stages can use them to determine whether or
	// not they can skip certain steps near the end of their stages.
	for stageIndex, stage := range stages {
		dependencyMap[stage.Name] = &stageDependencyInfo{Name: stage.Name, Position: stage.Position}
		stageLocalScopeArgs := make(map[string]string)
		node := stage.Node // first line
		for node != nil {  // each line
			for _, child := range node.Children { // tokens on this line, though we only care about the first
				switch strings.ToUpper(child.Value) { // first token - instruction
				case "FROM":
					if child.Next != nil { // second token on this line
						// If we have a fromOverride, replace the value of
						// image name for the first FROM in the Containerfile.
						if b.fromOverride != "" {
							child.Next.Value = b.fromOverride
							b.fromOverride = ""
						}
						base := child.Next.Value
						if base != "" && base != buildah.BaseImageFakeName {
							if replaceBuildContext, ok := b.additionalBuildContexts[child.Next.Value]; ok {
								if replaceBuildContext.IsImage {
									child.Next.Value = replaceBuildContext.Value
									base = child.Next.Value
								}
							}
							builtinArgs := argsMapToSlice(stage.Builder.BuiltinArgDefaults)
							headingArgs := argsMapToSlice(stage.Builder.HeadingArgs)
							userArgs := argsMapToSlice(stage.Builder.Args)
							localScopeArgs := argsMapToSlice(stageLocalScopeArgs)
							// ProcessWord uses first match; put highest priority first so
							// --build-arg overrides stage ARG overrides header ARG overrides builtin.
							userArgs = slices.Concat(userArgs, localScopeArgs, headingArgs, builtinArgs)
							baseWithArg, err := imagebuilder.ProcessWord(base, userArgs)
							if err != nil {
								return "", nil, fmt.Errorf("while replacing arg variables with values for format %q: %w", base, err)
							}
							b.baseMap[baseWithArg] = struct{}{}
							logrus.Debugf("base for stage %d: %q resolves to %q", stageIndex, base, baseWithArg)
							// Check if selected base is not an additional
							// build context and if base is a valid stage
							// add it to current stage's dependency tree.
							if _, ok := b.additionalBuildContexts[baseWithArg]; !ok {
								if _, ok := dependencyMap[baseWithArg]; ok {
									// update current stage's dependency info
									currentStageInfo := dependencyMap[stage.Name]
									currentStageInfo.Needs = append(currentStageInfo.Needs, baseWithArg)
								}
							}
						}
					}
					// Parse any --after= flag for explicit stage dependency
					for _, flag := range child.Flags {
						if after, ok := strings.CutPrefix(flag, "--after="); ok {
							// only allow one --after flag per FROM for now; nothing necessarily
							// semantically wrong with multiple --after, but keeping it conservative until a
							// use case shows up
							if _, exists := b.afterDependency[stage.Name]; exists {
								return "", nil, fmt.Errorf("FROM --after=%s: only one --after flag is allowed per FROM instruction", after)
							}
							builtinArgs := argsMapToSlice(stage.Builder.BuiltinArgDefaults)
							headingArgs := argsMapToSlice(stage.Builder.HeadingArgs)
							userArgs := argsMapToSlice(stage.Builder.Args)
							localScopeArgs := argsMapToSlice(stageLocalScopeArgs)
							// ProcessWord uses first match; put highest priority first so
							// --build-arg overrides stage ARG overrides header ARG overrides builtin.
							userArgs = slices.Concat(userArgs, localScopeArgs, headingArgs, builtinArgs)
							afterResolved, err := imagebuilder.ProcessWord(after, userArgs)
							if err != nil {
								return "", nil, fmt.Errorf("while replacing arg variables with values for --after=%q: %w", after, err)
							}
							// If --after=<index> convert index to name
							if index, err := strconv.Atoi(afterResolved); err == nil && index >= 0 && index < stageIndex {
								afterResolved = stages[index].Name
							}
							if depInfo, ok := dependencyMap[afterResolved]; !ok {
								return "", nil, fmt.Errorf("FROM --after=%s: stage %q not found", after, afterResolved)
							} else if depInfo.Position >= stageIndex {
								return "", nil, fmt.Errorf("FROM --after=%s: cannot depend on later stage %q", after, afterResolved)
							}
							// Mark the stage as a dep so we actually build it
							currentStageInfo := dependencyMap[stage.Name]
							currentStageInfo.Needs = append(currentStageInfo.Needs, afterResolved)
							// And mark it on the stage executor itself so it knows to wait before even pulling
							b.afterDependency[stage.Name] = afterResolved
							logrus.Debugf("stage %d: explicit dependency on %q via --after", stageIndex, afterResolved)
						}
					}
				case "ADD", "COPY":
					for _, flag := range child.Flags { // flags for this instruction
						if after, ok := strings.CutPrefix(flag, "--from="); ok {
							// Populate dependency tree and check
							// if following ADD or COPY needs any other
							// stage.
							stageName := after
							builtinArgs := argsMapToSlice(stage.Builder.BuiltinArgDefaults)
							headingArgs := argsMapToSlice(stage.Builder.HeadingArgs)
							userArgs := argsMapToSlice(stage.Builder.Args)
							localScopeArgs := argsMapToSlice(stageLocalScopeArgs)
							// ProcessWord uses first match; put highest priority first so
							// --build-arg overrides stage ARG overrides header ARG overrides builtin.
							userArgs = slices.Concat(userArgs, localScopeArgs, headingArgs, builtinArgs)
							baseWithArg, err := imagebuilder.ProcessWord(stageName, userArgs)
							if err != nil {
								return "", nil, fmt.Errorf("while replacing arg variables with values for format %q: %w", stageName, err)
							}
							if baseWithArg != "" {
								b.rootfsMap[baseWithArg] = struct{}{}
							}
							logrus.Debugf("stage %d name: %q resolves to %q", stageIndex, stageName, baseWithArg)
							stageName = baseWithArg
							// If --from=<index> convert index to name
							if index, err := strconv.Atoi(stageName); err == nil && index >= 0 && index < stageIndex {
								stageName = stages[index].Name
							}
							// Check if selected base is not an additional
							// build context and if base is a valid stage
							// add it to current stage's dependency tree.
							if _, ok := b.additionalBuildContexts[stageName]; !ok {
								if _, ok := dependencyMap[stageName]; ok {
									// update current stage's dependency info
									currentStageInfo := dependencyMap[stage.Name]
									currentStageInfo.Needs = append(currentStageInfo.Needs, stageName)
								}
							}
						}
					}
				case "ARG":
					for arg := child.Next; arg != nil; arg = arg.Next {
						argName, argValue, hasValue := strings.Cut(arg.Value, "=")
						if hasValue && argName != "" {
							argValue, err := imagebuilder.ProcessWord(argValue, argsMapToSlice(stage.Builder.BuiltinArgDefaults))
							if err != nil {
								return "", nil, fmt.Errorf("while replacing arg variables with values for format %q: %w", arg.Value, err)
							}
							stageLocalScopeArgs[argName] = argValue
						}
					}
				case "RUN":
					for _, flag := range child.Flags { // flags for this instruction
						// We need to populate dependency tree of stages
						// if it is using `--mount` and `from=` field is set
						// and `from=` points to a stage consider it in
						// dependency calculation.
						if strings.HasPrefix(flag, "--mount=") && strings.Contains(flag, "from") {
							mountFlags := strings.TrimPrefix(flag, "--mount=")
							fields := strings.SplitSeq(mountFlags, ",")
							for field := range fields {
								if mountFrom, hasFrom := strings.CutPrefix(field, "from="); hasFrom {
									// Check if this base is a stage if yes
									// add base to current stage's dependency tree
									// but also confirm if this is not in additional context.
									if _, ok := b.additionalBuildContexts[mountFrom]; !ok {
										// Treat from as a rootfs we need to preserve
										b.rootfsMap[mountFrom] = struct{}{}
										if _, ok := dependencyMap[mountFrom]; ok {
											// update current stage's dependency info
											currentStageInfo := dependencyMap[stage.Name]
											currentStageInfo.Needs = append(currentStageInfo.Needs, mountFrom)
										}
									}
								}
							}
						}
					}
				}
			}
			node = node.Next // next line
		}
		// Last stage is always target stage.
		// Since last/target stage is processed
		// let's calculate dependency map of stages
		// so we can mark stages which can be skipped.
		if stage.Position == (len(stages) - 1) {
			markDependencyStagesForTarget(dependencyMap, stage.Name)
		}
	}
	b.warnOnUnsetBuildArgs(stages, dependencyMap, b.args)

	type Result struct {
		Index         int
		ImageID       string
		OnlyBaseImage bool
		CommitResults buildah.CommitResults
		Error         error
	}

	ch := make(chan Result, len(stages))

	if b.stagesSemaphore == nil {
		b.stagesSemaphore = semaphore.NewWeighted(int64(len(stages)))
	}

	var wg sync.WaitGroup
	wg.Add(len(stages))

	var commitResults buildah.CommitResults
	go func() {
		cancel := false
		for stageIndex := range stages {
			index := stageIndex
			// Acquire the semaphore before creating the goroutine so we are sure they
			// run in the specified order.
			if err := b.stagesSemaphore.Acquire(ctx, 1); err != nil {
				cancel = true
				b.lastError = err
				ch <- Result{
					Index: index,
					Error: err,
				}
				wg.Done()
				continue
			}
			b.stagesLock.Lock()
			cleanupStages := cleanupStages
			b.stagesLock.Unlock()
			go func() {
				defer b.stagesSemaphore.Release(1)
				defer wg.Done()
				if cancel || cleanupStages == nil {
					var err error
					if stages[index].Name != strconv.Itoa(index) {
						err = fmt.Errorf("not building stage %d: build canceled", index)
					} else {
						err = fmt.Errorf("not building stage %d (%s): build canceled", index, stages[index].Name)
					}
					ch <- Result{
						Index: index,
						Error: err,
					}
					return
				}
				// Skip stage if it is not needed by TargetStage
				// or any of its dependency stages and `SkipUnusedStages`
				// is not set to `false`.
				if stageDependencyInfo, ok := dependencyMap[stages[index].Name]; ok {
					if !stageDependencyInfo.NeededByTarget && b.skipUnusedStages != types.OptionalBoolFalse {
						logrus.Debugf("Skipping stage with Name %q and index %d since its not needed by the target stage", stages[index].Name, index)
						ch <- Result{
							Index: index,
							Error: nil,
						}
						return
					}
				}
				stageID, stageResults, stageOnlyBaseImage, stageErr := b.buildStage(ctx, cleanupStages, stages, index)
				if stageErr != nil {
					cancel = true
					ch <- Result{
						Index:         index,
						Error:         stageErr,
						OnlyBaseImage: stageOnlyBaseImage,
					}
					return
				}

				ch <- Result{
					Index:         index,
					ImageID:       stageID,
					CommitResults: *stageResults,
					OnlyBaseImage: stageOnlyBaseImage,
					Error:         nil,
				}
			}()
		}
	}()
	go func() {
		wg.Wait()
		close(ch)
	}()

	for r := range ch {
		stage := stages[r.Index]

		b.stagesLock.Lock()
		b.terminatedStage[stage.Name] = r.Error
		b.terminatedStage[strconv.Itoa(stage.Position)] = r.Error

		if r.Error != nil {
			b.stagesLock.Unlock()
			b.lastError = r.Error
			return "", nil, r.Error
		}

		// If this is an intermediate stage, make a note of the ID, so
		// that we can look it up later.
		if r.Index < len(stages)-1 && r.ImageID != "" {
			b.imageMap[stage.Name] = r.ImageID
			b.imageDigestMap[stage.Name] = r.CommitResults.Digest.String()
			// We're not populating the cache with intermediate
			// images, so add this one to the list of images that
			// we'll remove later.
			// Only remove intermediate image if `--layers` is not provided,
			// `--save-stages` is not enabled, or following stage was not
			// only a base image (i.e. a different image).
			if !b.layers && !b.saveStages && !r.OnlyBaseImage {
				cleanupImages = append(cleanupImages, r.ImageID)
			}
		}
		if r.Index == len(stages)-1 {
			imageID = r.ImageID
			commitResults = r.CommitResults
			ref = commitResults.Canonical
		}
		b.stagesLock.Unlock()
	}

	if len(b.unusedArgs) > 0 {
		unusedList := make([]string, 0, len(b.unusedArgs))
		for k := range b.unusedArgs {
			unusedList = append(unusedList, k)
		}
		slices.Sort(unusedList)
		fmt.Fprintf(b.out, "[Warning] one or more build args were not consumed: %v\n", unusedList)
	}

	// Add additional tags and print image names recorded in storage
	if dest, err := b.resolveNameToImageRef(b.output); err == nil {
		switch dest.Transport().Name() {
		case storageTransport.Transport.Name():
			_, img, err := storageTransport.ResolveReference(dest)
			if err != nil {
				return imageID, ref, fmt.Errorf("locating just-written image %q: %w", transports.ImageName(dest), err)
			}
			if len(b.additionalTags) > 0 {
				if err = util.AddImageNames(b.store, "", b.systemContext, img, b.additionalTags); err != nil {
					return imageID, ref, fmt.Errorf("setting image names to %v: %w", append(img.Names, b.additionalTags...), err)
				}
				logrus.Debugf("assigned names %v to image %q", img.Names, img.ID)
			}
			// Report back the caller the tags applied, if any.
			_, img, err = storageTransport.ResolveReference(dest)
			if err != nil {
				return imageID, ref, fmt.Errorf("locating just-written image %q: %w", transports.ImageName(dest), err)
			}
			for _, name := range img.Names {
				fmt.Fprintf(b.out, "Successfully tagged %s\n", name)
			}

		default:
			if len(b.additionalTags) > 0 {
				b.logger.Warnf("don't know how to add tags to images stored in %q transport", dest.Transport().Name())
			}
		}
	}

	if err := cleanup(); err != nil {
		return "", nil, err
	}
	logrus.Debugf("printing final image id %q", imageID)
	if b.iidfile != "" {
		iid := imageID
		if iid != "" {
			cdigest, err := digest.Parse("sha256:" + imageID)
			if err != nil {
				return imageID, ref, fmt.Errorf("coercing image ID into a digest structure: %w", err)
			}
			iid = cdigest.String()
		}
		if err = os.WriteFile(b.iidfile, []byte(iid), 0o644); err != nil {
			return imageID, ref, fmt.Errorf("failed to write image ID to file %q: %w", b.iidfile, err)
		}
	}
	if b.iidfileRaw != "" {
		if err = os.WriteFile(b.iidfileRaw, []byte(imageID), 0o644); err != nil {
			return imageID, ref, fmt.Errorf("failed to write image ID to file %q: %w", b.iidfileRaw, err)
		}
	}
	if b.iidfile == "" && b.iidfileRaw == "" {
		if _, err := stdout.Write([]byte(imageID + "\n")); err != nil {
			return imageID, ref, fmt.Errorf("failed to write image ID to stdout: %w", err)
		}
	}
	if b.metadataFile != "" {
		var cdigest digest.Digest
		if imageID != "" {
			if cdigest, err = digest.Parse("sha256:" + imageID); err != nil {
				return imageID, ref, fmt.Errorf("coercing image ID into a digest structure: %w", err)
			}
		}
		metadata, err := metadata.Build(cdigest, v1.Descriptor{
			MediaType: commitResults.MediaType,
			Digest:    commitResults.Digest,
			Size:      int64(len(commitResults.ImageManifest)),
		})
		if err != nil {
			return imageID, ref, fmt.Errorf("building metadata for metadata file: %w", err)
		}
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			return imageID, ref, fmt.Errorf("encoding metadata for metadata file: %w", err)
		}
		if err = os.WriteFile(b.metadataFile, metadataBytes, 0o644); err != nil {
			return imageID, ref, fmt.Errorf("failed to write image metadata to file %q: %w", b.metadataFile, err)
		}
	}
	return imageID, ref, nil
}

// deleteSuccessfulIntermediateCtrs goes through the container IDs in each
// stage's containerIDs list and deletes the containers associated with those
// IDs.
func (b *executor) deleteSuccessfulIntermediateCtrs() error {
	var lastErr error
	for _, s := range b.stages {
		for _, ctr := range s.containerIDs {
			if err := b.store.DeleteContainer(ctr); err != nil {
				b.logger.Errorf("error deleting build container %q: %v\n", ctr, err)
				lastErr = err
			}
		}
		// The stages map includes some stages under multiple keys, so
		// clearing their lists after we process a given stage is
		// necessary to avoid triggering errors that would occur if we
		// tried to delete a given stage's containers multiple times.
		s.containerIDs = nil
	}
	return lastErr
}
