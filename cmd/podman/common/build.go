package common

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	encconfig "github.com/containers/ocicrypt/config"
	enchelpers "github.com/containers/ocicrypt/helpers"
	"github.com/openshift/imagebuilder"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	buildahDefine "go.podman.io/buildah/define"
	buildahCLI "go.podman.io/buildah/pkg/cli"
	"go.podman.io/buildah/pkg/parse"
	buildahUtil "go.podman.io/buildah/pkg/util"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/config"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/pkg/compression"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/env"
)

// BuildFlagsWrapper are local to cmd/ as the build code is using Buildah-internal
// types.  Hence, after parsing, we are converting buildFlagsWrapper to the entities'
// options which essentially embed the Buildah types.
type BuildFlagsWrapper struct {
	// Buildah stuff first
	buildahCLI.BudResults
	buildahCLI.LayerResults
	buildahCLI.FromAndBudResults
	buildahCLI.NameSpaceResults
	buildahCLI.UserNSResults

	// SquashAll squashes all layers into a single layer.
	SquashAll bool
	// Cleanup removes built images from remote connections on success
	Cleanup bool
}

// FarmBuildHiddenFlags are the flags hidden from the farm build command because they are either not
// supported or don't make sense in the farm build use case
var FarmBuildHiddenFlags = []string{
	"arch", "all-platforms", "compress", "cw", "disable-content-trust",
	"logsplit", "manifest", "metadata-file", "os", "output", "platform", "sign-by", "signature-policy", "stdin",
	"variant",
}

func DefineBuildFlags(cmd *cobra.Command, buildOpts *BuildFlagsWrapper, isFarmBuild bool) {
	flags := cmd.Flags()

	// buildx build --load ignored, but added for compliance
	flags.Bool("load", false, "buildx --load")
	_ = flags.MarkHidden("load")

	// buildx build --progress ignored, but added for compliance
	flags.String("progress", "auto", "buildx --progress")
	_ = flags.MarkHidden("progress")

	// Podman flags
	flags.BoolVarP(&buildOpts.SquashAll, "squash-all", "", false, "Squash all layers into a single layer")

	// Bud flags
	budFlags := buildahCLI.GetBudFlags(&buildOpts.BudResults)

	// --pull flag
	flag := budFlags.Lookup("pull")
	flag.DefValue = "missing"
	if err := flag.Value.Set("missing"); err != nil {
		logrus.Errorf("Unable to set --pull to 'missing': %v", err)
	}
	flag.Usage = `Pull image policy ("always"|"missing"|"never"|"newer")`
	flags.AddFlagSet(&budFlags)

	// Add the completion functions
	budCompletions := buildahCLI.GetBudFlagsCompletions()
	completion.CompleteCommandFlags(cmd, budCompletions)

	// Layer flags
	layerFlags := buildahCLI.GetLayerFlags(&buildOpts.LayerResults)
	// --layers flag
	flag = layerFlags.Lookup("layers")
	useLayersVal := useLayers()
	buildOpts.Layers = useLayersVal == "true"
	if err := flag.Value.Set(useLayersVal); err != nil {
		logrus.Errorf("Unable to set --layers to %v: %v", useLayersVal, err)
	}
	flag.DefValue = useLayersVal
	// --force-rm flag
	flag = layerFlags.Lookup("force-rm")
	if err := flag.Value.Set("true"); err != nil {
		logrus.Errorf("Unable to set --force-rm to true: %v", err)
	}
	flag.DefValue = "true"
	flags.AddFlagSet(&layerFlags)

	// FromAndBud flags
	fromAndBudFlags, err := buildahCLI.GetFromAndBudFlags(&buildOpts.FromAndBudResults, &buildOpts.UserNSResults, &buildOpts.NameSpaceResults)
	if err != nil {
		logrus.Errorf("Setting up build flags: %v", err)
		os.Exit(1)
	}

	flags.AddFlagSet(&fromAndBudFlags)
	// Add the completion functions
	fromAndBudFlagsCompletions := buildahCLI.GetFromAndBudFlagsCompletions()
	completion.CompleteCommandFlags(cmd, fromAndBudFlagsCompletions)
	flags.SetNormalizeFunc(buildahCLI.AliasFlags)
	if registry.IsRemote() {
		// Unset the isolation default as we never want to send this over the API
		// as it can be wrong (root vs rootless).
		_ = flags.Lookup("isolation").Value.Set("")
		_ = flags.MarkHidden("disable-content-trust")
		_ = flags.MarkHidden("sign-by")
		_ = flags.MarkHidden("signature-policy")
		_ = flags.MarkHidden("compress")
		_ = flags.MarkHidden("output")
		_ = flags.MarkHidden("logsplit")
		_ = flags.MarkHidden("cw")
		// Support for farm build in podman-remote
		if !isFarmBuild {
			_ = flags.MarkHidden("tls-verify")
		}
	}
	if isFarmBuild {
		for _, f := range FarmBuildHiddenFlags {
			_ = flags.MarkHidden(f)
		}
	}
}

func ParseBuildOpts(cmd *cobra.Command, args []string, buildOpts *BuildFlagsWrapper) (*entities.BuildOptions, error) {
	if cmd.Flags().Changed("squash-all") && cmd.Flags().Changed("squash") {
		return nil, errors.New("cannot specify --squash-all with --squash")
	}

	if cmd.Flag("output").Changed && registry.IsRemote() {
		return nil, errors.New("'--output' option is not supported in remote mode")
	}
	if cmd.Flag("metadata-file").Changed && registry.IsRemote() {
		return nil, errors.New("'--metadata-file' option is not supported in remote mode")
	}

	if buildOpts.Network == "none" {
		if cmd.Flag("dns").Changed {
			return nil, errors.New("the --dns option cannot be used with --network=none")
		}
		if cmd.Flag("dns-option").Changed {
			return nil, errors.New("the --dns-option option cannot be used with --network=none")
		}
		if cmd.Flag("dns-search").Changed {
			return nil, errors.New("the --dns-search option cannot be used with --network=none")
		}
	}

	if cmd.Flag("network").Changed {
		if buildOpts.Network != "host" && buildOpts.Isolation == buildahDefine.IsolationChroot.String() {
			return nil, fmt.Errorf("cannot set --network other than host with --isolation %s", buildOpts.Isolation)
		}
	}

	if buildOpts.StageLabels && !buildOpts.SaveStages {
		return nil, errors.New(`"--stage-labels" requires "--save-stages"`)
	}

	// Extract container files from the CLI (i.e., --file/-f) first.
	var containerFiles []string
	for _, f := range buildOpts.File {
		if f == "-" {
			if len(args) == 0 {
				args = append(args, "-")
			} else {
				containerFiles = append(containerFiles, "/dev/stdin")
			}
		} else {
			containerFiles = append(containerFiles, f)
		}
	}

	// Determine context directory.
	var (
		contextDir   string
		apiBuildOpts entities.BuildOptions
	)
	if len(args) > 0 {
		// The context directory could be a URL.  Try to handle that.
		tempDir, subDir, err := buildahDefine.TempDirForURL("", "buildah", args[0])
		if err != nil {
			return nil, fmt.Errorf("prepping temporary context directory: %w", err)
		}
		if tempDir != "" {
			apiBuildOpts.TmpDirToClose = tempDir
			contextDir = filepath.Join(tempDir, subDir)
		} else {
			// Nope, it was local.  Use it as is.
			absDir, err := filepath.Abs(args[0])
			if err != nil {
				return nil, fmt.Errorf("determining path to directory %q: %w", args[0], err)
			}
			contextDir = absDir
		}
	} else {
		// No context directory or URL was specified.  Try to use the home of
		// the first locally-available Containerfile.
		for i := range containerFiles {
			if isURL(containerFiles[i]) {
				continue
			}
			absFile, err := filepath.Abs(containerFiles[i])
			if err != nil {
				return nil, fmt.Errorf("determining path to file %q: %w", containerFiles[i], err)
			}
			contextDir = filepath.Dir(absFile)
			// /dev/fd cannot be used as an overlay mount lower layer, use a tmp context dir instead
			if contextDir == "/dev/fd" || contextDir == "/proc/self/fd" {
				tempDir, err := os.MkdirTemp("", "podman-build-context-")
				if err != nil {
					return nil, fmt.Errorf("creating temporary context directory: %w", err)
				}
				apiBuildOpts.TmpDirToClose = tempDir
				contextDir = tempDir
			}
			containerFiles[i] = absFile
			break
		}
	}

	if contextDir == "" {
		return nil, errors.New("no context directory and no Containerfile specified")
	}
	if !utils.IsDir(contextDir) {
		return nil, fmt.Errorf("context must be a directory: %q", contextDir)
	}
	if len(containerFiles) == 0 {
		switch {
		case utils.FileExists(filepath.Join(contextDir, "Containerfile")):
			if utils.IsDir(filepath.Join(contextDir, "Containerfile")) {
				return nil, fmt.Errorf("containerfile:  cannot be path or directory")
			}
			containerFiles = append(containerFiles, filepath.Join(contextDir, "Containerfile"))
		case utils.FileExists(filepath.Join(contextDir, "Dockerfile")):
			if utils.IsDir(filepath.Join(contextDir, "Dockerfile")) {
				return nil, fmt.Errorf("dockerfile:  cannot be path or directory")
			}
			containerFiles = append(containerFiles, filepath.Join(contextDir, "Dockerfile"))
		default:
			return nil, fmt.Errorf("no Containerfile or Dockerfile specified or found in context directory, %s: %w", contextDir, syscall.ENOENT)
		}
	}

	if err := areContainerfilesValid(contextDir, containerFiles); err != nil {
		return nil, err
	}

	var logFile *os.File
	if cmd.Flag("logfile").Changed {
		var err error
		logFile, err = os.OpenFile(buildOpts.Logfile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, err
		}
		apiBuildOpts.LogFileToClose = logFile
	}

	buildahDefineOpts, err := buildFlagsWrapperToOptions(cmd, contextDir, buildOpts, logFile, buildOpts.Layers, buildOpts.Squash)
	if err != nil {
		return nil, err
	}
	apiBuildOpts.BuildOptions = *buildahDefineOpts
	apiBuildOpts.ContainerFiles = containerFiles
	apiBuildOpts.Authfile = buildOpts.Authfile

	return &apiBuildOpts, err
}

// buildFlagsWrapperToOptions converts the local build flags to the build options used
// in the API which embed Buildah types used across the build code.  Doing the
// conversion here prevents the API from doing that (redundantly).
//
// TODO: this code should really be in Buildah.
func buildFlagsWrapperToOptions(c *cobra.Command, contextDir string, flags *BuildFlagsWrapper, logfile *os.File, layers, squash bool) (*buildahDefine.BuildOptions, error) {
	output := ""
	tags := []string{}
	if c.Flag("tag").Changed {
		tags = flags.Tag
		if len(tags) > 0 {
			output = tags[0]
			tags = tags[1:]
		}
	}

	if c.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(flags.Authfile); err != nil {
			return nil, err
		}
	}

	commonOpts, err := parse.CommonBuildOptions(c)
	if err != nil {
		return nil, err
	}

	pullFlagsCount := 0
	if c.Flag("pull").Changed {
		pullFlagsCount++
	}
	if c.Flag("pull-always").Changed {
		pullFlagsCount++
	}
	if c.Flag("pull-never").Changed {
		pullFlagsCount++
	}

	if pullFlagsCount > 1 {
		return nil, errors.New("can only set one of 'pull' or 'pull-always' or 'pull-never'")
	}

	// Allow for --pull, --pull=true, --pull=false, --pull=never, --pull=always
	// --pull-always and --pull-never.  The --pull-never and --pull-always options
	// will not be documented.
	pullPolicy := buildahDefine.PullIfMissing
	if c.Flags().Changed("pull") && strings.EqualFold(strings.TrimSpace(flags.Pull), "true") {
		pullPolicy = buildahDefine.PullAlways
	}
	if flags.PullAlways || strings.EqualFold(strings.TrimSpace(flags.Pull), "always") {
		pullPolicy = buildahDefine.PullAlways
	}

	if flags.PullNever ||
		strings.EqualFold(strings.TrimSpace(flags.Pull), "false") ||
		strings.EqualFold(strings.TrimSpace(flags.Pull), "never") {
		pullPolicy = buildahDefine.PullNever
	}

	if strings.EqualFold(strings.TrimSpace(flags.Pull), "newer") {
		pullPolicy = buildahDefine.PullIfNewer
	}

	var cleanTmpFile bool
	flags.Authfile, cleanTmpFile = buildahUtil.MirrorToTempFileIfPathIsDescriptor(flags.Authfile)
	if cleanTmpFile {
		defer os.Remove(flags.Authfile)
	}

	args := make(map[string]string)
	if c.Flag("build-arg-file").Changed {
		for _, argfile := range flags.BuildArgFile {
			fargs, err := env.ParseFile(argfile)
			if err != nil {
				return nil, err
			}
			maps.Copy(args, fargs)
		}
	}
	if c.Flag("build-arg").Changed {
		for _, arg := range flags.BuildArg {
			key, val, hasVal := strings.Cut(arg, "=")
			if hasVal {
				args[key] = val
			} else {
				// check if the env is set in the local environment and use that value if it is
				if val, present := os.LookupEnv(key); present {
					args[key] = val
				} else {
					delete(args, key)
				}
			}
		}
	}
	flags.Layers = layers

	// `buildah bud --layers=false` acts like `docker build --squash` does.
	// That is all of the new layers created during the build process are
	// condensed into one, any layers present prior to this build are
	// retained without condensing.  `buildah bud --squash` squashes both
	// new and old layers down into one.  Translate Podman commands into
	// Buildah.  Squash invoked, retain old layers, squash new layers into
	// one.
	if c.Flags().Changed("squash") && squash {
		flags.Squash = false
		flags.Layers = false
	}
	// Squash-all invoked, squash both new and old layers into one.
	if c.Flags().Changed("squash-all") {
		flags.Squash = true
		if !c.Flags().Changed("layers") {
			// Buildah  supports using layers and --squash together
			// after https://github.com/containers/buildah/pull/3674
			// so podman must honor if user wants to still use layers
			//  with --squash-all.
			flags.Layers = false
		}
	}

	var stdin io.Reader
	if flags.Stdin {
		stdin = os.Stdin
	}
	var stdout, stderr, reporter *os.File
	stdout = os.Stdout
	stderr = os.Stderr
	reporter = os.Stderr

	if logfile != nil {
		logrus.SetOutput(logfile)
		stdout = logfile
		stderr = logfile
		reporter = logfile
	}

	nsValues, networkPolicy, err := parse.NamespaceOptions(c)
	if err != nil {
		return nil, err
	}

	compressionIntent := buildahDefine.Gzip
	if flags.DisableCompression {
		compressionIntent = buildahDefine.Uncompressed
	}

	isolation := buildahDefine.IsolationDefault
	// Only parse the isolation when it is actually needed as we do not want to send a wrong default
	// to the server in the remote case (root vs rootless).
	if flags.Isolation != "" {
		isolation, err = parse.IsolationOption(flags.Isolation)
		if err != nil {
			return nil, err
		}
	}

	usernsOption, idmappingOptions, err := parse.IDMappingOptions(c, isolation)
	if err != nil {
		return nil, err
	}
	nsValues = append(nsValues, usernsOption...)

	systemContext, err := parse.SystemContextFromOptions(c)
	if err != nil {
		return nil, err
	}

	var format string
	flags.Format = strings.ToLower(flags.Format)
	switch {
	case strings.HasPrefix(flags.Format, buildahDefine.OCI):
		format = buildahDefine.OCIv1ImageManifest
	case strings.HasPrefix(flags.Format, buildahDefine.DOCKER):
		format = buildahDefine.Dockerv2ImageManifest
	default:
		return nil, fmt.Errorf("unrecognized image type %q", flags.Format)
	}

	runtimeFlags := []string{}
	for _, arg := range flags.RuntimeFlags {
		runtimeFlags = append(runtimeFlags, "--"+arg)
	}

	podmanConfig := registry.PodmanConfig()
	for _, arg := range podmanConfig.RuntimeFlags {
		runtimeFlags = append(runtimeFlags, "--"+arg)
	}
	configIndex := filepath.Base(podmanConfig.RuntimePath)
	if len(runtimeFlags) == 0 {
		for _, arg := range podmanConfig.ContainersConfDefaultsRO.Engine.OCIRuntimesFlags[configIndex] {
			runtimeFlags = append(runtimeFlags, "--"+arg)
		}
	}
	if podmanConfig.ContainersConf.Engine.CgroupManager == config.SystemdCgroupsManager {
		runtimeFlags = append(runtimeFlags, "--systemd-cgroup")
	}

	platforms, err := parse.PlatformsFromOptions(c)
	if err != nil {
		return nil, err
	}

	decConfig, err := getDecryptConfig(flags.DecryptionKeys)
	if err != nil {
		return nil, fmt.Errorf("unable to obtain decrypt config: %w", err)
	}

	additionalBuildContext := make(map[string]*buildahDefine.AdditionalBuildContext)
	if c.Flag("build-context").Changed {
		for _, contextString := range flags.BuildContext {
			key, val, hasVal := strings.Cut(contextString, "=")
			if hasVal {
				parseAdditionalBuildContext, err := parse.GetAdditionalBuildContext(val)
				if err != nil {
					return nil, fmt.Errorf("while parsing additional build context: %w", err)
				}
				additionalBuildContext[key] = &parseAdditionalBuildContext
			} else {
				return nil, fmt.Errorf("while parsing additional build context: %s, accepts value in the form of key=value", contextString)
			}
		}
	}
	var cacheTo []reference.Named
	var cacheFrom []reference.Named
	if c.Flag("cache-to").Changed {
		cacheTo, err = parse.RepoNamesToNamedReferences(flags.CacheTo)
		if err != nil {
			return nil, fmt.Errorf("unable to parse value provided `%s` to --cache-to: %w", flags.CacheTo, err)
		}
	}
	if c.Flag("cache-from").Changed {
		cacheFrom, err = parse.RepoNamesToNamedReferences(flags.CacheFrom)
		if err != nil {
			return nil, fmt.Errorf("unable to parse value provided `%s` to --cache-from: %w", flags.CacheTo, err)
		}
	}
	var cacheTTL time.Duration
	if c.Flag("cache-ttl").Changed {
		cacheTTL, err = time.ParseDuration(flags.CacheTTL)
		if err != nil {
			return nil, fmt.Errorf("unable to parse value provided %q as --cache-ttl: %w", flags.CacheTTL, err)
		}
	}

	var confidentialWorkloadOptions buildahDefine.ConfidentialWorkloadOptions
	if c.Flag("cw").Changed {
		confidentialWorkloadOptions, err = parse.GetConfidentialWorkloadOptions(flags.CWOptions)
		if err != nil {
			return nil, err
		}
	}

	retryDelay := 2 * time.Second
	if flags.RetryDelay != "" {
		retryDelay, err = time.ParseDuration(flags.RetryDelay)
		if err != nil {
			return nil, fmt.Errorf("unable to parse value provided %q as --retry-delay: %w", flags.RetryDelay, err)
		}
	}

	var sbomScanOptions []buildahDefine.SBOMScanOptions
	if c.Flag("sbom").Changed || c.Flag("sbom-scanner-command").Changed || c.Flag("sbom-scanner-image").Changed || c.Flag("sbom-image-output").Changed || c.Flag("sbom-merge-strategy").Changed || c.Flag("sbom-output").Changed || c.Flag("sbom-image-output").Changed || c.Flag("sbom-purl-output").Changed || c.Flag("sbom-image-purl-output").Changed {
		sbomScanOption, err := parse.SBOMScanOptions(c)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(sbomScanOption.ContextDir, contextDir) {
			sbomScanOption.ContextDir = append(sbomScanOption.ContextDir, contextDir)
		}
		for _, abc := range additionalBuildContext {
			if !abc.IsURL && !abc.IsImage {
				sbomScanOption.ContextDir = append(sbomScanOption.ContextDir, abc.Value)
			}
		}
		sbomScanOption.PullPolicy = pullPolicy
		sbomScanOptions = append(sbomScanOptions, *sbomScanOption)
	}

	if c.Flag("disable-compression").Changed && flags.DisableCompression {
		if c.Flag("compression-format").Changed {
			return nil, errors.New("--disable-compression and --compression-format cannot be used together")
		}
		if c.Flag("force-compression").Changed {
			return nil, errors.New("--disable-compression and --force-compression cannot be used together")
		}
	}
	var compressionLevel *int
	if c.Flag("compression-level").Changed {
		compressionLevel = &flags.CompressionLevel
	} else {
		compressionLevel = podmanConfig.ContainersConfDefaultsRO.Engine.CompressionLevel
	}
	var compressionFormat *compression.Algorithm
	forceCompressionFormat := flags.ForceCompressionFormat
	if c.Flag("compression-format").Changed {
		algo, err := compression.AlgorithmByName(flags.CompressionFormat)
		if err != nil {
			return nil, fmt.Errorf("unable to parse value provided %q as --compression-format: %w", flags.CompressionFormat, err)
		}
		compressionFormat = &algo
		if !c.Flag("disable-compression").Changed {
			compressionIntent = buildahDefine.Gzip
		}
	} else {
		algo, err := compression.AlgorithmByName(podmanConfig.ContainersConfDefaultsRO.Engine.CompressionFormat)
		if err != nil {
			return nil, fmt.Errorf("parsing compression_format from containers.conf: %w", err)
		}
		compressionFormat = &algo
		if !c.Flag("force-compression").Changed {
			forceCompressionFormat = true
		}
		if !c.Flag("disable-compression").Changed {
			compressionIntent = buildahDefine.Gzip
		}
	}
	opts := buildahDefine.BuildOptions{
		AddCapabilities:         flags.CapAdd,
		AdditionalTags:          tags,
		AdditionalBuildContexts: additionalBuildContext,
		AllPlatforms:            flags.AllPlatforms,
		Annotations:             flags.Annotation,
		Args:                    args,
		BlobDirectory:           flags.BlobCache,
		BuildOutputs:            flags.BuildOutputs,
		CacheFrom:               cacheFrom,
		CacheTo:                 cacheTo,
		CacheTTL:                cacheTTL,
		ConfidentialWorkload:    confidentialWorkloadOptions,
		CommonBuildOpts:         commonOpts,
		Compression:             compressionIntent,
		CompressionFormat:       compressionFormat,
		CompressionLevel:        compressionLevel,
		ConfigureNetwork:        networkPolicy,
		ContextDirectory:        contextDir,
		CPPFlags:                flags.CPPFlags,
		DefaultMountsFilePath:   podmanConfig.ContainersConfDefaultsRO.Containers.DefaultMountsFile,
		Devices:                 flags.Devices,
		DropCapabilities:        flags.CapDrop,
		Envs:                    buildahCLI.LookupEnvVarReferences(flags.Envs, os.Environ()),
		Err:                     stderr,
		ForceCompressionFormat:  forceCompressionFormat,
		ForceRmIntermediateCtrs: flags.ForceRm,
		From:                    flags.From,
		GroupAdd:                flags.GroupAdd,
		IDMappingOptions:        idmappingOptions,
		In:                      stdin,
		Isolation:               isolation,
		Jobs:                    &flags.Jobs,
		Labels:                  flags.Label,
		LayerLabels:             flags.LayerLabel,
		Layers:                  flags.Layers,
		LogRusage:               flags.LogRusage,
		LogFile:                 flags.Logfile,
		LogSplitByPlatform:      flags.LogSplitByPlatform,
		Manifest:                flags.Manifest,
		MaxPullPushRetries:      flags.Retry,
		NamespaceOptions:        nsValues,
		NoCache:                 flags.NoCache,
		OSFeatures:              flags.OSFeatures,
		OSVersion:               flags.OSVersion,
		OciDecryptConfig:        decConfig,
		Out:                     stdout,
		Output:                  output,
		OutputFormat:            format,
		Platforms:               platforms,
		PullPolicy:              pullPolicy,
		PullPushRetryDelay:      retryDelay,
		Quiet:                   flags.Quiet,
		RemoveIntermediateCtrs:  flags.Rm,
		ReportWriter:            reporter,
		RewriteTimestamp:        flags.RewriteTimestamp,
		Runtime:                 podmanConfig.RuntimePath,
		RuntimeArgs:             runtimeFlags,
		RusageLogFile:           flags.RusageLogFile,
		SaveStages:              flags.SaveStages,
		SBOMScanOptions:         sbomScanOptions,
		SignBy:                  flags.SignBy,
		SignaturePolicyPath:     flags.SignaturePolicy,
		SourcePolicyFile:        flags.SourcePolicyFile,
		Squash:                  flags.Squash,
		StageLabels:             flags.StageLabels,
		SystemContext:           systemContext,
		Target:                  flags.Target,
		TransientMounts:         flags.Volumes,
		TransientRunMounts:      flags.TransientRunMounts,
		UnsetEnvs:               flags.UnsetEnvs,
		UnsetLabels:             flags.UnsetLabels,
		UnsetAnnotations:        flags.UnsetAnnotations,
		MetadataFile:            flags.MetadataFile,
	}

	if c.Flag("created-annotation").Changed {
		opts.CreatedAnnotation = types.NewOptionalBool(flags.CreatedAnnotation)
	}
	if c.Flag("compat-volumes").Changed {
		opts.CompatVolumes = types.NewOptionalBool(flags.CompatVolumes)
	}
	if c.Flag("inherit-labels").Changed {
		opts.InheritLabels = types.NewOptionalBool(flags.InheritLabels)
	}

	if c.Flag("inherit-annotations").Changed {
		opts.InheritAnnotations = types.NewOptionalBool(flags.InheritAnnotations)
	}

	if flags.IgnoreFile != "" {
		excludes, err := imagebuilder.ParseIgnore(flags.IgnoreFile)
		if err != nil {
			return nil, fmt.Errorf("unable to parse ignore file: %w", err)
		}
		opts.Excludes = excludes
	}

	if flags.SourceDateEpoch != "" { // could be explicitly specified, or passed via the environment, tricking .Changed()
		sde, err := strconv.ParseInt(flags.SourceDateEpoch, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing source-date-epoch value %q: %w", flags.SourceDateEpoch, err)
		}
		sourceDateEpoch := time.Unix(sde, 0).UTC()
		opts.SourceDateEpoch = &sourceDateEpoch
	}
	if c.Flag("timestamp").Changed {
		timestamp := time.Unix(flags.Timestamp, 0).UTC()
		opts.Timestamp = &timestamp
	}

	if c.Flag("skip-unused-stages").Changed {
		opts.SkipUnusedStages = types.NewOptionalBool(flags.SkipUnusedStages)
	}

	return &opts, nil
}

// useLayers returns false if BUILDAH_LAYERS is set to "0" or "false"
// otherwise it returns true
func useLayers() string {
	layers := os.Getenv("BUILDAH_LAYERS")
	if strings.ToLower(layers) == "false" || layers == "0" {
		return "false"
	}
	return "true"
}

func getDecryptConfig(decryptionKeys []string) (*encconfig.DecryptConfig, error) {
	decConfig := &encconfig.DecryptConfig{}
	if len(decryptionKeys) > 0 {
		// decryption
		dcc, err := enchelpers.CreateCryptoConfig([]string{}, decryptionKeys)
		if err != nil {
			return nil, fmt.Errorf("invalid decryption keys: %w", err)
		}
		cc := encconfig.CombineCryptoConfigs([]encconfig.CryptoConfig{dcc})
		decConfig = cc.DecryptConfig
	}

	return decConfig, nil
}

func areContainerfilesValid(contextDir string, containerFiles []string) error {
	for _, f := range containerFiles {
		if isURL(f) || f == "/dev/stdin" {
			continue
		}

		// Because currently podman runs the test/bud.bats tests under the buildah project in CI,
		// the following error messages need to be consistent with buildah; otherwise, the podman CI will fail.
		// See: https://github.com/containers/buildah/blob/4c781b59b49d66e07324566555339888113eb7e2/imagebuildah/build.go#L139-L141
		// 	    https://github.com/containers/buildah/blob/4c781b59b49d66e07324566555339888113eb7e2/tests/bud.bats#L3474-L3479
		if utils.IsDir(f) {
			return fmt.Errorf("containerfile: %q cannot be path to a directory", f)
		}

		// If the file is not found, try again with context directory prepended (if not prepended yet)
		// Ref: https://github.com/containers/buildah/blob/4c781b59b49d66e07324566555339888113eb7e2/imagebuildah/build.go#L125-L135
		if utils.FileExists(f) {
			continue
		}
		if !strings.HasPrefix(f, contextDir) {
			if utils.FileExists(filepath.Join(contextDir, f)) {
				continue
			}
		}

		return fmt.Errorf("the specified Containerfile or Dockerfile does not exist, %s: %w", f, syscall.ENOENT)
	}

	return nil
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "git://") ||
		strings.HasPrefix(s, "github.com/")
}
