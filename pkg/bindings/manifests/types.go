package manifests

import "io"

// InspectOptions are optional options for inspecting manifests
//
//go:generate go run ../generator/generator.go InspectOptions
type InspectOptions struct {
	// Authfile - path to an authentication file.
	Authfile *string
	// SkipTLSVerify - skip https and certificate validation when
	// contacting container registries.
	SkipTLSVerify *bool
}

// CreateOptions are optional options for creating manifests
//
//go:generate go run ../generator/generator.go CreateOptions
type CreateOptions struct {
	All        *bool
	Amend      *bool
	Annotation map[string]string `json:"annotations" schema:"annotations"`
}

// ExistsOptions are optional options for checking
// if a manifest list exists
//
//go:generate go run ../generator/generator.go ExistsOptions
type ExistsOptions struct{}

// AddOptions are optional options for adding manifest lists
//
//go:generate go run ../generator/generator.go AddOptions
type AddOptions struct {
	All *bool

	Annotation map[string]string `json:"annotations" schema:"annotations"`
	Arch       *string
	Features   []string
	OS         *string
	OSVersion  *string
	OSFeatures []string
	Variant    *string

	Images        []string
	Authfile      *string
	Password      *string
	Username      *string
	SkipTLSVerify *bool `schema:"-"`
}

// AddArtifactOptions are optional options for adding artifact manifests
//
//go:generate go run ../generator/generator.go AddArtifactOptions
type AddArtifactOptions struct {
	Annotation map[string]string `json:"annotations" schema:"annotations"`
	Arch       *string
	Features   []string
	OS         *string
	OSVersion  *string
	OSFeatures []string
	Variant    *string

	Type          **string          `json:"artifact_type,omitempty"`
	ConfigType    *string           `json:"artifact_config_type,omitempty"`
	Config        *string           `json:"artifact_config,omitempty"`
	LayerType     *string           `json:"artifact_layer_type,omitempty"`
	ExcludeTitles *bool             `json:"artifact_exclude_titles,omitempty"`
	Subject       *string           `json:"artifact_subject,omitempty"`
	Annotations   map[string]string `json:"artifact_annotations,omitempty"`
	Files         []string          `json:"artifact_files,omitempty"`
}

// RemoveOptions are optional options for removing manifest lists
//
//go:generate go run ../generator/generator.go RemoveOptions
type RemoveOptions struct{}

// ModifyOptions are optional options for modifying manifest lists
//
//go:generate go run ../generator/generator.go ModifyOptions
type ModifyOptions struct {
	// Operation values are "update", "remove" and "annotate". This allows the service to
	// efficiently perform each update on a manifest list.
	Operation *string
	All       *bool // All when true, operate on all images in a manifest list that may be included in Images

	Annotations      map[string]string // Annotations to add to the entries for Images in the manifest list
	IndexAnnotations map[string]string `json:"index_annotations" schema:"index_annotations"` // Annotations to add to the manifest list as a whole
	Arch             *string           // Arch overrides the architecture for the image
	Features         []string          // Feature list for the image
	OS               *string           // OS overrides the operating system for the image
	OSFeatures       []string          `json:"os_features" schema:"os_features"` // OSFeatures overrides the OS features for the image
	OSVersion        *string           `json:"os_version" schema:"os_version"`   // OSVersion overrides the operating system version for the image
	Variant          *string           // Variant overrides the architecture variant for the image

	Images        []string // Images is an optional list of images to add/remove to/from manifest list depending on operation
	Authfile      *string
	Password      *string
	Username      *string
	SkipTLSVerify *bool `schema:"-"`

	ArtifactType          **string          `json:"artifact_type"`           // the ArtifactType in an artifact manifest being created
	ArtifactConfigType    *string           `json:"artifact_config_type"`    // the config.MediaType in an artifact manifest being created
	ArtifactConfig        *string           `json:"artifact_config"`         // the config.Data in an artifact manifest being created
	ArtifactLayerType     *string           `json:"artifact_layer_type"`     // the MediaType for each layer in an artifact manifest being created
	ArtifactExcludeTitles *bool             `json:"artifact_exclude_titles"` // whether or not to include title annotations for each layer in an artifact manifest being created
	ArtifactSubject       *string           `json:"artifact_subject"`        // subject to set in an artifact manifest being created
	ArtifactAnnotations   map[string]string `json:"artifact_annotations"`    // annotations to add to an artifact manifest being created
	ArtifactFiles         *[]string         `json:"artifact_files"`          // an optional list of files to add to a new artifact manifest in the manifest list
	Body                  *io.Reader        `json:"-" schema:"-"`
}
