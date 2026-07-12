package cli

// the cli package contains spf13/cobra related structs that help make up
// the command line for buildah commands. it resides here so other projects
// that vendor in this code can use them too.

import (
	"fmt"
	"maps"
	"os"
	"runtime"
	"strings"

	encconfig "github.com/containers/ocicrypt/config"
	enchelpers "github.com/containers/ocicrypt/helpers"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/spf13/pflag"
	"go.podman.io/buildah/define"
	"go.podman.io/buildah/internal"
	"go.podman.io/buildah/pkg/completion"
	"go.podman.io/buildah/pkg/parse"
	commonComp "go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/config"
	"go.podman.io/storage/pkg/unshare"
)

// LayerResults represents the results of the layer flags
type LayerResults struct {
	ForceRm     bool
	Layers      bool
	SaveStages  bool
	StageLabels bool
}

// UserNSResults represents the results for the UserNS flags
type UserNSResults struct {
	UserNS            string
	GroupAdd          []string
	UserNSUIDMap      []string
	UserNSGIDMap      []string
	UserNSUIDMapUser  string
	UserNSGIDMapGroup string
}

// NameSpaceResults represents the results for Namespace flags
type NameSpaceResults struct {
	Cgroup  string
	IPC     string
	Network string
	// Deprecated: CNIConfigDir is no longer used and is expected to be empty.
	CNIConfigDir string
	// Deprecated: CNIPlugInPath is no longer used and is expected to be empty.
	CNIPlugInPath string
	PID           string
	UTS           string
}

// BudResults represents the results for Build flags
type BudResults struct {
	AllPlatforms           bool
	Annotation             []string
	Authfile               string
	BuildArg               []string
	BuildArgFile           []string
	BuildContext           []string
	CacheFrom              []string
	CacheTo                []string
	CacheTTL               string
	CertDir                string
	Compress               bool
	Creds                  string
	CPPFlags               []string
	CompressionFormat      string
	CompressionLevel       int
	ForceCompressionFormat bool
	DisableCompression     bool
	DisableContentTrust    bool
	IgnoreFile             string
	File                   []string
	Format                 string
	From                   string
	Iidfile                string
	IidfileRaw             string
	InheritLabels          bool
	InheritAnnotations     bool
	Label                  []string
	LayerLabel             []string
	Logfile                string
	LogSplitByPlatform     bool
	Manifest               string
	MetadataFile           string
	NoHostname             bool
	NoHosts                bool
	NoCache                bool
	Timestamp              int64
	OmitHistory            bool
	OCIHooksDir            []string
	Pull                   string
	PullAlways             bool
	PullNever              bool
	Quiet                  bool
	IdentityLabel          bool
	Rm                     bool
	Runtime                string
	RuntimeFlags           []string
	SbomPreset             string
	SbomScannerImage       string
	SbomScannerCommand     []string
	SbomMergeStrategy      string
	SbomOutput             string
	SbomImgOutput          string
	SbomPurlOutput         string
	SbomImgPurlOutput      string
	Secrets                []string
	SSH                    []string
	SignaturePolicy        string
	SignBy                 string
	Squash                 bool
	SkipUnusedStages       bool
	Stdin                  bool
	Tag                    []string
	BuildOutputs           []string
	Target                 string
	TLSVerify              bool
	Jobs                   int
	LogRusage              bool
	RusageLogFile          string
	UnsetEnvs              []string
	UnsetLabels            []string
	UnsetAnnotations       []string
	Envs                   []string
	OSFeatures             []string
	OSVersion              string
	CWOptions              string
	SBOMOptions            []string
	CompatVolumes          bool
	SourceDateEpoch        string
	RewriteTimestamp       bool
	CreatedAnnotation      bool
	SourcePolicyFile       string
	TransientRunMounts     []string
}

// FromAndBugResults represents the results for common flags
// in build and from
type FromAndBudResults struct {
	AddHost        []string
	BlobCache      string
	CapAdd         []string
	CapDrop        []string
	CDIConfigDir   string
	CgroupParent   string
	CPUPeriod      uint64
	CPUQuota       int64
	CPUSetCPUs     string
	CPUSetMems     string
	CPUShares      uint64
	DecryptionKeys []string
	Devices        []string
	DNSSearch      []string
	DNSServers     []string
	DNSOptions     []string
	HTTPProxy      bool
	Isolation      string
	Memory         string
	MemorySwap     string
	Retry          int
	RetryDelay     string
	SecurityOpt    []string
	ShmSize        string
	Ulimit         []string
	Volumes        []string
}

// GetUserNSFlags returns the common flags for usernamespace
func GetUserNSFlags(flags *UserNSResults) pflag.FlagSet {
	usernsFlags := pflag.FlagSet{}
	usernsFlags.StringSliceVar(&flags.GroupAdd, "group-add", nil, "add additional groups to the primary container process. 'keep-groups' allows container processes to use supplementary groups.")
	usernsFlags.StringVar(&flags.UserNS, "userns", "", "'container', `path` of user namespace to join, or 'host'")
	usernsFlags.StringSliceVar(&flags.UserNSUIDMap, "userns-uid-map", []string{}, "`containerUID:hostUID:length` UID mapping to use in user namespace")
	usernsFlags.StringSliceVar(&flags.UserNSGIDMap, "userns-gid-map", []string{}, "`containerGID:hostGID:length` GID mapping to use in user namespace")
	usernsFlags.StringVar(&flags.UserNSUIDMapUser, "userns-uid-map-user", "", "`name` of entries from /etc/subuid to use to set user namespace UID mapping")
	usernsFlags.StringVar(&flags.UserNSGIDMapGroup, "userns-gid-map-group", "", "`name` of entries from /etc/subgid to use to set user namespace GID mapping")
	return usernsFlags
}

// GetUserNSFlagsCompletions returns the FlagCompletions for the userns flags
func GetUserNSFlagsCompletions() commonComp.FlagCompletions {
	flagCompletion := commonComp.FlagCompletions{}
	flagCompletion["group-add"] = commonComp.AutocompleteNone
	flagCompletion["userns"] = completion.AutocompleteNamespaceFlag
	flagCompletion["userns-uid-map"] = commonComp.AutocompleteNone
	flagCompletion["userns-gid-map"] = commonComp.AutocompleteNone
	flagCompletion["userns-uid-map-user"] = commonComp.AutocompleteSubuidName
	flagCompletion["userns-gid-map-group"] = commonComp.AutocompleteSubgidName
	return flagCompletion
}

// GetNameSpaceFlags returns the common flags for a namespace menu
func GetNameSpaceFlags(flags *NameSpaceResults) pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.StringVar(&flags.Cgroup, "cgroupns", "", "'private', or 'host'")
	fs.StringVar(&flags.IPC, string(specs.IPCNamespace), "", "'private', `path` of IPC namespace to join, or 'host'")
	fs.StringVar(&flags.Network, string(specs.NetworkNamespace), "", "'private', 'none', 'ns:path' of network namespace to join, or 'host'")
	fs.StringVar(&flags.PID, string(specs.PIDNamespace), "", "private, `path` of PID namespace to join, or 'host'")
	fs.StringVar(&flags.UTS, string(specs.UTSNamespace), "", "private, :`path` of UTS namespace to join, or 'host'")
	return fs
}

// GetNameSpaceFlagsCompletions returns the FlagCompletions for the namespace flags
func GetNameSpaceFlagsCompletions() commonComp.FlagCompletions {
	flagCompletion := commonComp.FlagCompletions{}
	flagCompletion["cgroupns"] = completion.AutocompleteNamespaceFlag
	flagCompletion[string(specs.IPCNamespace)] = completion.AutocompleteNamespaceFlag
	flagCompletion[string(specs.NetworkNamespace)] = completion.AutocompleteNamespaceFlag
	flagCompletion[string(specs.PIDNamespace)] = completion.AutocompleteNamespaceFlag
	flagCompletion[string(specs.UTSNamespace)] = completion.AutocompleteNamespaceFlag
	return flagCompletion
}

// GetLayerFlags returns the common flags for layers
func GetLayerFlags(flags *LayerResults) pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.BoolVar(&flags.ForceRm, "force-rm", false, "always remove intermediate containers after a build, even if the build is unsuccessful.")
	fs.BoolVar(&flags.Layers, "layers", UseLayers(), "use intermediate layers during build. Use BUILDAH_LAYERS environment variable to override.")
	fs.BoolVar(&flags.SaveStages, "save-stages", false, "save intermediate stage images.")
	fs.BoolVar(&flags.StageLabels, "stage-labels", false, "add metadata labels to intermediate stage images (requires --save-stages).")
	return fs
}

// Note: GetLayerFlagsCompletion is not needed since GetLayerFlags only contains bool flags

// GetBudFlags returns common build flags
func GetBudFlags(flags *BudResults) pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.BoolVar(&flags.AllPlatforms, "all-platforms", false, "attempt to build for all base image platforms")
	fs.String("arch", runtime.GOARCH, "set the ARCH of the image to the provided value instead of the architecture of the host")
	fs.StringArrayVar(&flags.Annotation, "annotation", []string{}, "set metadata for an image (default [])")
	fs.StringVar(&flags.Authfile, "authfile", "", "path of the authentication file.")
	fs.StringArrayVar(&flags.OCIHooksDir, "hooks-dir", []string{}, "set the OCI hooks directory path (may be set multiple times)")
	fs.StringArrayVar(&flags.BuildArg, "build-arg", []string{}, "`argument=value` to supply to the builder")
	fs.StringArrayVar(&flags.BuildArgFile, "build-arg-file", []string{}, "`argfile.conf` containing lines of argument=value to supply to the builder")
	fs.StringArrayVar(&flags.BuildContext, "build-context", []string{}, "`argument=value` to supply additional build context to the builder")
	fs.StringArrayVar(&flags.CacheFrom, "cache-from", []string{}, "remote repository list to utilise as potential cache source.")
	fs.StringArrayVar(&flags.CacheTo, "cache-to", []string{}, "remote repository list to utilise as potential cache destination.")
	fs.StringVar(&flags.CacheTTL, "cache-ttl", "", "only consider cache images under specified duration.")
	fs.StringVar(&flags.CertDir, "cert-dir", "", "use certificates at the specified path to access the registry")
	fs.BoolVar(&flags.Compress, "compress", false, "this is a legacy option, which has no effect on the image")
	fs.BoolVar(&flags.CompatVolumes, "compat-volumes", false, "preserve the contents of VOLUMEs during RUN instructions")
	fs.BoolVar(&flags.InheritLabels, "inherit-labels", true, "inherit the labels from the base image or base stages.")
	fs.BoolVar(&flags.InheritAnnotations, "inherit-annotations", true, "inherit the annotations from the base image or base stages.")
	fs.StringArrayVar(&flags.CPPFlags, "cpp-flag", []string{}, "set additional flag to pass to C preprocessor (cpp)")
	fs.BoolVar(&flags.CreatedAnnotation, "created-annotation", true, `set an "org.opencontainers.image.created" annotation in the image`)
	fs.StringVar(&flags.Creds, "creds", "", "use `[username[:password]]` for accessing the registry")
	fs.StringVarP(&flags.CWOptions, "cw", "", "", "confidential workload `options`")
	fs.StringVar(&flags.CompressionFormat, "compression-format", "", "compression format to use for layers and cache")
	fs.IntVar(&flags.CompressionLevel, "compression-level", 0, "compression level to use for layers and cache")
	fs.BoolVar(&flags.ForceCompressionFormat, "force-compression", false, "use the specified compression algorithm even if the destination contains a differently-compressed variant already")
	fs.BoolVarP(&flags.DisableCompression, "disable-compression", "D", true, "don't compress layers by default")
	fs.BoolVar(&flags.DisableContentTrust, "disable-content-trust", false, "this is a Docker specific option and is a NOOP")
	fs.StringArrayVar(&flags.Envs, "env", []string{}, "set environment variable for the image")
	fs.StringVar(&flags.From, "from", "", "image name used to replace the value in the first FROM instruction in the Containerfile")
	fs.StringVar(&flags.IgnoreFile, "ignorefile", "", "path to an alternate .dockerignore file")
	fs.StringSliceVarP(&flags.File, "file", "f", []string{}, "`pathname or URL` of a Dockerfile")
	fs.StringVar(&flags.Format, "format", DefaultFormat(), "`format` of the built image's manifest and metadata. Use BUILDAH_FORMAT environment variable to override.")
	fs.StringVar(&flags.Iidfile, "iidfile", "", "`file` to write the image ID to")
	fs.StringVar(&flags.IidfileRaw, "iidfile-raw", "", "`file` to write the image ID to (without algorithm prefix)")
	fs.IntVar(&flags.Jobs, "jobs", 1, "how many stages to run in parallel")
	fs.StringArrayVar(&flags.Label, "label", []string{}, "set metadata for an image (default [])")
	fs.StringArrayVar(&flags.LayerLabel, "layer-label", []string{}, "set metadata for an intermediate image (default [])")
	fs.StringVar(&flags.Logfile, "logfile", "", "log to `file` instead of stdout/stderr")
	fs.BoolVar(&flags.LogSplitByPlatform, "logsplit", false, "split logfile to different files for each platform")
	fs.Int("loglevel", 0, "NO LONGER USED, flag ignored, and hidden")
	if err := fs.MarkHidden("loglevel"); err != nil {
		panic(fmt.Sprintf("error marking the loglevel flag as hidden: %v", err))
	}
	fs.BoolVar(&flags.LogRusage, "log-rusage", false, "log resource usage at each build step")
	if err := fs.MarkHidden("log-rusage"); err != nil {
		panic(fmt.Sprintf("error marking the log-rusage flag as hidden: %v", err))
	}
	fs.StringVar(&flags.RusageLogFile, "rusage-logfile", "", "destination file to which rusage should be logged to instead of stdout (= the default).")
	if err := fs.MarkHidden("rusage-logfile"); err != nil {
		panic(fmt.Sprintf("error marking the rusage-logfile flag as hidden: %v", err))
	}
	fs.StringVar(&flags.Manifest, "manifest", "", "add the image to the specified manifest list. Creates manifest list if it does not exist")
	fs.StringVar(&flags.MetadataFile, "metadata-file", "", "`file` to write metadata about the image to")
	fs.BoolVar(&flags.NoCache, "no-cache", false, "do not use existing cached images for the container build. Build from the start with a new set of cached layers.")
	fs.BoolVar(&flags.NoHostname, "no-hostname", false, "do not create new /etc/hostname file for RUN instructions, use the one from the base image.")
	fs.BoolVar(&flags.NoHosts, "no-hosts", false, "do not create new /etc/hosts file for RUN instructions, use the one from the base image.")
	fs.String("os", runtime.GOOS, "set the OS to the provided value instead of the current operating system of the host")
	fs.StringArrayVar(&flags.OSFeatures, "os-feature", []string{}, "set required OS `feature` for the target image in addition to values from the base image")
	fs.StringVar(&flags.OSVersion, "os-version", "", "set required OS `version` for the target image instead of the value from the base image")
	fs.StringVar(&flags.Pull, "pull", "missing", `pull base and SBOM scanner images from the registry. Values:
always:  pull base and SBOM scanner images even if the named images are present in store.
missing: pull base and SBOM scanner images if the named images are not present in store.
never:   only use images present in store if available.
newer:   only pull base and SBOM scanner images when newer images exist on the registry than those in the store.`)
	fs.Lookup("pull").NoOptDefVal = "always" // treat a --pull with no argument like --pull=always
	fs.BoolVar(&flags.PullAlways, "pull-always", false, "pull the image even if the named image is present in store")
	if err := fs.MarkHidden("pull-always"); err != nil {
		panic(fmt.Sprintf("error marking the pull-always flag as hidden: %v", err))
	}
	fs.BoolVar(&flags.PullNever, "pull-never", false, "do not pull the image, use the image present in store if available")
	if err := fs.MarkHidden("pull-never"); err != nil {
		panic(fmt.Sprintf("error marking the pull-never flag as hidden: %v", err))
	}
	fs.BoolVarP(&flags.Quiet, "quiet", "q", false, "refrain from announcing build instructions and image read/write progress")
	fs.BoolVar(&flags.OmitHistory, "omit-history", false, "omit build history information from built image")
	fs.BoolVar(&flags.IdentityLabel, "identity-label", true, "add default identity label")
	fs.BoolVar(&flags.Rm, "rm", true, "remove intermediate containers after a successful build")
	// "runtime" definition moved to avoid name collision in podman build.  Defined in cmd/buildah/build.go.
	fs.StringSliceVar(&flags.RuntimeFlags, "runtime-flag", []string{}, "add global flags for the container runtime")
	fs.StringArrayVar(&flags.TransientRunMounts, "mount", []string{}, "set transient mounts for each RUN instruction, e.g. type=secret,id=mysecret")
	fs.StringVar(&flags.SbomPreset, "sbom", "", "scan working container using `preset` configuration")
	fs.StringVar(&flags.SbomScannerImage, "sbom-scanner-image", "", "scan working container using scanner command from `image`")
	fs.StringArrayVar(&flags.SbomScannerCommand, "sbom-scanner-command", nil, "scan working container using `command` in scanner image")
	fs.StringVar(&flags.SbomMergeStrategy, "sbom-merge-strategy", "", "merge scan results using `strategy`")
	fs.StringVar(&flags.SbomOutput, "sbom-output", "", "save scan results to `file`")
	fs.StringVar(&flags.SbomImgOutput, "sbom-image-output", "", "add scan results to image as `path`")
	fs.StringVar(&flags.SbomPurlOutput, "sbom-purl-output", "", "save scan results to `file``")
	fs.StringVar(&flags.SbomImgPurlOutput, "sbom-image-purl-output", "", "add scan results to image as `path`")
	fs.StringArrayVar(&flags.Secrets, "secret", []string{}, "secret file to expose to the build")
	fs.StringVar(&flags.SignBy, "sign-by", "", "sign the image using a GPG key with the specified `FINGERPRINT`")
	fs.StringVar(&flags.SignaturePolicy, "signature-policy", "", "`pathname` of signature policy file (not usually used)")
	if err := fs.MarkHidden("signature-policy"); err != nil {
		panic(fmt.Sprintf("error marking the signature-policy flag as hidden: %v", err))
	}
	fs.StringVar(&flags.SourcePolicyFile, "source-policy-file", "", "`pathname` of source policy file for controlling source references during build")
	fs.BoolVar(&flags.SkipUnusedStages, "skip-unused-stages", true, "skips stages in multi-stage builds which do not affect the final target")
	sourceDateEpochUsageDefault := ", defaults to current time"
	if v := os.Getenv(internal.SourceDateEpochName); v != "" {
		sourceDateEpochUsageDefault = ""
	}
	fs.StringVar(&flags.SourceDateEpoch, "source-date-epoch", os.Getenv(internal.SourceDateEpochName), "set new timestamps in image info to `seconds` after the epoch"+sourceDateEpochUsageDefault)
	fs.BoolVar(&flags.RewriteTimestamp, "rewrite-timestamp", false, "set timestamps in layers to no later than the value for --source-date-epoch")
	fs.BoolVar(&flags.Squash, "squash", false, "squash all image layers into a single layer")
	fs.StringArrayVar(&flags.SSH, "ssh", []string{}, "SSH agent socket or keys to expose to the build. (format: default|<id>[=<socket>|<key>[,<key>]])")
	fs.BoolVar(&flags.Stdin, "stdin", false, "pass stdin into containers")
	fs.StringArrayVarP(&flags.Tag, "tag", "t", []string{}, "tagged `name` to apply to the built image")
	fs.StringArrayVarP(&flags.BuildOutputs, "output", "o", nil, "output destination (format: type=local,dest=path)")
	fs.StringVar(&flags.Target, "target", "", "set the target build stage to build")
	fs.Int64Var(&flags.Timestamp, "timestamp", 0, "set new timestamps in image info and layer to `seconds` after the epoch, defaults to current times")
	fs.BoolVar(&flags.TLSVerify, "tls-verify", true, "require HTTPS and verify certificates when accessing the registry")
	fs.String("variant", "", "override the `variant` of the specified image")
	fs.StringSliceVar(&flags.UnsetEnvs, "unsetenv", nil, "unset environment variable from final image")
	fs.StringSliceVar(&flags.UnsetLabels, "unsetlabel", nil, "unset label when inheriting labels from base image")
	fs.StringSliceVar(&flags.UnsetAnnotations, "unsetannotation", nil, "unset annotation when inheriting annotations from base image")
	return fs
}

// GetBudFlagsCompletions returns the FlagCompletions for the common build flags
func GetBudFlagsCompletions() commonComp.FlagCompletions {
	flagCompletion := commonComp.FlagCompletions{}
	flagCompletion["annotation"] = commonComp.AutocompleteNone
	flagCompletion["arch"] = commonComp.AutocompleteNone
	flagCompletion["authfile"] = commonComp.AutocompleteDefault
	flagCompletion["build-arg"] = commonComp.AutocompleteNone
	flagCompletion["build-arg-file"] = commonComp.AutocompleteDefault
	flagCompletion["build-context"] = commonComp.AutocompleteNone
	flagCompletion["cache-from"] = commonComp.AutocompleteNone
	flagCompletion["cache-to"] = commonComp.AutocompleteNone
	flagCompletion["cache-ttl"] = commonComp.AutocompleteNone
	flagCompletion["cert-dir"] = commonComp.AutocompleteDefault
	flagCompletion["compression-format"] = commonComp.AutocompleteNone
	flagCompletion["compression-level"] = commonComp.AutocompleteNone
	flagCompletion["cpp-flag"] = commonComp.AutocompleteNone
	flagCompletion["creds"] = commonComp.AutocompleteNone
	flagCompletion["cw"] = commonComp.AutocompleteNone
	flagCompletion["env"] = commonComp.AutocompleteNone
	flagCompletion["file"] = commonComp.AutocompleteDefault
	flagCompletion["format"] = commonComp.AutocompleteNone
	flagCompletion["from"] = commonComp.AutocompleteDefault
	flagCompletion["hooks-dir"] = commonComp.AutocompleteNone
	flagCompletion["ignorefile"] = commonComp.AutocompleteDefault
	flagCompletion["iidfile"] = commonComp.AutocompleteDefault
	flagCompletion["iidfile-raw"] = commonComp.AutocompleteDefault
	flagCompletion["jobs"] = commonComp.AutocompleteNone
	flagCompletion["label"] = commonComp.AutocompleteNone
	flagCompletion["layer-label"] = commonComp.AutocompleteNone
	flagCompletion["logfile"] = commonComp.AutocompleteDefault
	flagCompletion["manifest"] = commonComp.AutocompleteDefault
	flagCompletion["metadata-file"] = commonComp.AutocompleteDefault
	flagCompletion["mount"] = commonComp.AutocompleteNone
	flagCompletion["os"] = commonComp.AutocompleteNone
	flagCompletion["os-feature"] = commonComp.AutocompleteNone
	flagCompletion["os-version"] = commonComp.AutocompleteNone
	flagCompletion["output"] = commonComp.AutocompleteNone
	flagCompletion["pull"] = commonComp.AutocompleteDefault
	flagCompletion["runtime-flag"] = commonComp.AutocompleteNone
	flagCompletion["sbom"] = commonComp.AutocompleteNone
	flagCompletion["sbom-scanner-image"] = commonComp.AutocompleteNone
	flagCompletion["sbom-scanner-command"] = commonComp.AutocompleteNone
	flagCompletion["sbom-merge-strategy"] = commonComp.AutocompleteNone
	flagCompletion["sbom-output"] = commonComp.AutocompleteDefault
	flagCompletion["sbom-image-output"] = commonComp.AutocompleteNone
	flagCompletion["sbom-purl-output"] = commonComp.AutocompleteDefault
	flagCompletion["sbom-image-purl-output"] = commonComp.AutocompleteNone
	flagCompletion["secret"] = commonComp.AutocompleteNone
	flagCompletion["sign-by"] = commonComp.AutocompleteNone
	flagCompletion["signature-policy"] = commonComp.AutocompleteNone
	flagCompletion["source-policy-file"] = commonComp.AutocompleteDefault
	flagCompletion["ssh"] = commonComp.AutocompleteNone
	flagCompletion["source-date-epoch"] = commonComp.AutocompleteNone
	flagCompletion["tag"] = commonComp.AutocompleteNone
	flagCompletion["target"] = commonComp.AutocompleteNone
	flagCompletion["timestamp"] = commonComp.AutocompleteNone
	flagCompletion["unsetenv"] = commonComp.AutocompleteNone
	flagCompletion["unsetlabel"] = commonComp.AutocompleteNone
	flagCompletion["unsetannotation"] = commonComp.AutocompleteNone
	flagCompletion["variant"] = commonComp.AutocompleteNone
	return flagCompletion
}

// GetFromAndBudFlags returns from and build flags
func GetFromAndBudFlags(flags *FromAndBudResults, usernsResults *UserNSResults, namespaceResults *NameSpaceResults) (pflag.FlagSet, error) {
	fs := pflag.FlagSet{}
	defaultContainerConfig, err := config.Default()
	if err != nil {
		return fs, fmt.Errorf("failed to get default container config: %w", err)
	}

	fs.StringSliceVar(&flags.AddHost, "add-host", []string{}, "add a custom host-to-IP mapping (`host:ip`) (default [])")
	fs.StringVar(&flags.BlobCache, "blob-cache", "", "assume image blobs in the specified directory will be available for pushing")
	if err := fs.MarkHidden("blob-cache"); err != nil {
		panic(fmt.Sprintf("error marking net flag as hidden: %v", err))
	}
	fs.StringSliceVar(&flags.CapAdd, "cap-add", []string{}, "add the specified capability when running (default [])")
	fs.StringSliceVar(&flags.CapDrop, "cap-drop", []string{}, "drop the specified capability when running (default [])")
	fs.StringVar(&flags.CDIConfigDir, "cdi-config-dir", "", "`directory` of CDI configuration files")
	_ = fs.MarkHidden("cdi-config-dir")
	fs.StringVar(&flags.CgroupParent, "cgroup-parent", "", "optional parent cgroup for the container")
	fs.Uint64Var(&flags.CPUPeriod, "cpu-period", 0, "limit the CPU CFS (Completely Fair Scheduler) period")
	fs.Int64Var(&flags.CPUQuota, "cpu-quota", 0, "limit the CPU CFS (Completely Fair Scheduler) quota")
	fs.Uint64VarP(&flags.CPUShares, "cpu-shares", "c", 0, "CPU shares (relative weight)")
	fs.StringVar(&flags.CPUSetCPUs, "cpuset-cpus", "", "CPUs in which to allow execution (0-3, 0,1)")
	fs.StringVar(&flags.CPUSetMems, "cpuset-mems", "", "memory nodes (MEMs) in which to allow execution (0-3, 0,1). Only effective on NUMA systems.")
	fs.StringSliceVar(&flags.DecryptionKeys, "decryption-key", nil, "key needed to decrypt the image")
	fs.StringArrayVar(&flags.Devices, "device", defaultContainerConfig.Containers.Devices.Get(), "additional devices to provide")
	fs.StringSliceVar(&flags.DNSSearch, "dns-search", defaultContainerConfig.Containers.DNSSearches.Get(), "set custom DNS search domains")
	fs.StringSliceVar(&flags.DNSServers, "dns", defaultContainerConfig.Containers.DNSServers.Get(), "set custom DNS servers or disable it completely by setting it to 'none', which prevents the automatic creation of `/etc/resolv.conf`.")
	fs.StringSliceVar(&flags.DNSOptions, "dns-option", defaultContainerConfig.Containers.DNSOptions.Get(), "set custom DNS options")
	fs.BoolVar(&flags.HTTPProxy, "http-proxy", true, "pass through HTTP Proxy environment variables")
	fs.StringVar(&flags.Isolation, "isolation", DefaultIsolation(), "`type` of process isolation to use. Use BUILDAH_ISOLATION environment variable to override.")
	fs.StringVarP(&flags.Memory, "memory", "m", "", "memory limit (format: <number>[<unit>], where unit = b, k, m or g)")
	fs.StringVar(&flags.MemorySwap, "memory-swap", "", "swap limit equal to memory plus swap: '-1' to enable unlimited swap")
	fs.IntVar(&flags.Retry, "retry", int(defaultContainerConfig.Engine.Retry), "number of times to retry in case of failure when performing push/pull")
	fs.StringVar(&flags.RetryDelay, "retry-delay", defaultContainerConfig.Engine.RetryDelay, "delay between retries in case of push/pull failures")
	fs.String("arch", runtime.GOARCH, "set the ARCH of the image to the provided value instead of the architecture of the host")
	fs.String("os", runtime.GOOS, "prefer `OS` instead of the running OS when pulling images")
	fs.StringSlice("platform", []string{parse.DefaultPlatform()}, "set the `OS/ARCH[/VARIANT]` of the image to the provided value instead of the current operating system and architecture of the host (for example \"linux/arm\")")
	fs.String("variant", "", "override the `variant` of the specified image")
	fs.StringArrayVar(&flags.SecurityOpt, "security-opt", []string{}, "security options (default [])")
	fs.StringVar(&flags.ShmSize, "shm-size", defaultContainerConfig.Containers.ShmSize, "size of '/dev/shm'. The format is `<number><unit>`.")
	fs.StringSliceVar(&flags.Ulimit, "ulimit", defaultContainerConfig.Containers.DefaultUlimits.Get(), "ulimit options")
	fs.StringArrayVarP(&flags.Volumes, "volume", "v", defaultContainerConfig.Volumes(), "bind mount a volume into the container")

	// Add in the usernamespace and namespaceflags
	usernsFlags := GetUserNSFlags(usernsResults)
	namespaceFlags := GetNameSpaceFlags(namespaceResults)
	fs.AddFlagSet(&usernsFlags)
	fs.AddFlagSet(&namespaceFlags)

	return fs, nil
}

// GetFromAndBudFlagsCompletions returns the FlagCompletions for the from and build flags
func GetFromAndBudFlagsCompletions() commonComp.FlagCompletions {
	flagCompletion := commonComp.FlagCompletions{}
	flagCompletion["arch"] = commonComp.AutocompleteNone
	flagCompletion["add-host"] = commonComp.AutocompleteNone
	flagCompletion["blob-cache"] = commonComp.AutocompleteNone
	flagCompletion["cap-add"] = commonComp.AutocompleteCapabilities
	flagCompletion["cap-drop"] = commonComp.AutocompleteCapabilities
	flagCompletion["cgroup-parent"] = commonComp.AutocompleteDefault // FIXME: This would be a path right?!
	flagCompletion["cpu-period"] = commonComp.AutocompleteNone
	flagCompletion["cpu-quota"] = commonComp.AutocompleteNone
	flagCompletion["cpu-shares"] = commonComp.AutocompleteNone
	flagCompletion["cpuset-cpus"] = commonComp.AutocompleteNone
	flagCompletion["cpuset-mems"] = commonComp.AutocompleteNone
	flagCompletion["decryption-key"] = commonComp.AutocompleteNone
	flagCompletion["device"] = commonComp.AutocompleteDefault
	flagCompletion["dns-search"] = commonComp.AutocompleteNone
	flagCompletion["dns"] = commonComp.AutocompleteNone
	flagCompletion["dns-option"] = commonComp.AutocompleteNone
	flagCompletion["isolation"] = commonComp.AutocompleteNone
	flagCompletion["memory"] = commonComp.AutocompleteNone
	flagCompletion["memory-swap"] = commonComp.AutocompleteNone
	flagCompletion["os"] = commonComp.AutocompleteNone
	flagCompletion["platform"] = commonComp.AutocompleteNone
	flagCompletion["retry"] = commonComp.AutocompleteNone
	flagCompletion["retry-delay"] = commonComp.AutocompleteNone
	flagCompletion["security-opt"] = commonComp.AutocompleteNone
	flagCompletion["shm-size"] = commonComp.AutocompleteNone
	flagCompletion["ulimit"] = commonComp.AutocompleteNone
	flagCompletion["volume"] = commonComp.AutocompleteDefault
	flagCompletion["variant"] = commonComp.AutocompleteNone

	// Add in the usernamespace and namespace flag completions
	userNsComp := GetUserNSFlagsCompletions()
	maps.Copy(flagCompletion, userNsComp)
	namespaceComp := GetNameSpaceFlagsCompletions()
	maps.Copy(flagCompletion, namespaceComp)

	return flagCompletion
}

// UseLayers returns true if BUILDAH_LAYERS is set to "1" or "true"
// otherwise it returns false
func UseLayers() bool {
	layers := os.Getenv("BUILDAH_LAYERS")
	if strings.ToLower(layers) == "true" || layers == "1" {
		return true
	}
	return false
}

// DefaultFormat returns the default image format
func DefaultFormat() string {
	format := os.Getenv("BUILDAH_FORMAT")
	if format != "" {
		return format
	}
	return define.OCI
}

// DefaultIsolation returns the default image format
func DefaultIsolation() string {
	isolation := os.Getenv("BUILDAH_ISOLATION")
	if isolation != "" {
		return isolation
	}
	if unshare.IsRootless() {
		return "rootless"
	}
	return define.OCI
}

// DefaultHistory returns the default add-history setting
func DefaultHistory() bool {
	history := os.Getenv("BUILDAH_HISTORY")
	if strings.ToLower(history) == "true" || history == "1" {
		return true
	}
	return false
}

func VerifyFlagsArgsOrder(args []string) error {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("no options (%s) can be specified after the image or container name", arg)
		}
	}
	return nil
}

// AliasFlags is a function to handle backwards compatibility with old flags
func AliasFlags(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "net":
		name = "network"
	case "override-arch":
		name = "arch"
	case "override-os":
		name = "os"
	case "purge":
		name = "rm"
	case "raw-iidfile":
		name = "iidfile-raw"
	case "tty":
		name = "terminal"
	}
	return pflag.NormalizedName(name)
}

// LookupEnvVarReferences returns a copy of specs with keys and values resolved
// from environ. Strings are in "key=value" form, the same as [os.Environ].
//
//   - When a string in specs lacks "=", it is treated as a key and the value
//     is retrieved from environ. When the key is missing from environ, neither
//     the key nor value are returned.
//
//   - When a string in specs lacks "=" and ends with "*", it is treated as
//     a key prefix and any keys with the same prefix in environ are returned.
//
//   - When a string in specs is exactly "*", all keys and values in environ
//     are returned.
func LookupEnvVarReferences(specs, environ []string) []string {
	result := make([]string, 0, len(specs))

	for _, spec := range specs {
		if key, _, ok := strings.Cut(spec, "="); ok {
			result = append(result, spec)
		} else if key == "*" {
			result = append(result, environ...)
		} else {
			prefix := key + "="
			if strings.HasSuffix(key, "*") {
				prefix = strings.TrimSuffix(key, "*")
			}

			for _, spec := range environ {
				if strings.HasPrefix(spec, prefix) {
					result = append(result, spec)
				}
			}
		}
	}

	return result
}

// DecryptConfig translates decryptionKeys into a DescriptionConfig structure
func DecryptConfig(decryptionKeys []string) (*encconfig.DecryptConfig, error) {
	var decryptConfig *encconfig.DecryptConfig
	if len(decryptionKeys) > 0 {
		// decryption
		dcc, err := enchelpers.CreateCryptoConfig([]string{}, decryptionKeys)
		if err != nil {
			return nil, fmt.Errorf("invalid decryption keys: %w", err)
		}
		cc := encconfig.CombineCryptoConfigs([]encconfig.CryptoConfig{dcc})
		decryptConfig = cc.DecryptConfig
	}

	return decryptConfig, nil
}

// EncryptConfig translates encryptionKeys into a EncriptionsConfig structure
func EncryptConfig(encryptionKeys []string, encryptLayers []int) (*encconfig.EncryptConfig, *[]int, error) {
	var encLayers *[]int
	var encConfig *encconfig.EncryptConfig

	if len(encryptionKeys) > 0 {
		// encryption
		encLayers = &encryptLayers
		ecc, err := enchelpers.CreateCryptoConfig(encryptionKeys, []string{})
		if err != nil {
			return nil, nil, fmt.Errorf("invalid encryption keys: %w", err)
		}
		cc := encconfig.CombineCryptoConfigs([]encconfig.CryptoConfig{ecc})
		encConfig = cc.EncryptConfig
	}
	return encConfig, encLayers, nil
}

// GetFormat translates format string into either docker or OCI format constant
func GetFormat(format string) (string, error) {
	switch format {
	case define.OCI:
		return define.OCIv1ImageManifest, nil
	case define.DOCKER:
		return define.Dockerv2ImageManifest, nil
	default:
		return "", fmt.Errorf("unrecognized image type %q", format)
	}
}
