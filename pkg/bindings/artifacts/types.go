package artifacts

import "io"

// PullOptions are optional options for pulling images
//
//go:generate go run ../generator/generator.go PullOptions
type PullOptions struct {
	// Authfile is the path to the authentication file.
	Authfile *string `schema:"-"`
	// Password for authenticating against the registry.
	Password *string `schema:"-"`
	// ProgressWriter is a writer where pull progress are sent.
	ProgressWriter *io.Writer `schema:"-"`
	// Quiet can be specified to suppress pull progress when pulling.
	Quiet *bool
	// Retry number of times to retry pull in case of failure
	Retry *uint
	// RetryDelay between retries in case of pull failures
	RetryDelay *string
	// SkipTLSVerify to skip HTTPS and certificate verification.
	TlsVerify *bool
	// Username for authenticating against the registry.
	Username *string `schema:"-"`
}

// PushOptions are optional options for pushing images
//
//go:generate go run ../generator/generator.go PushOptions
type PushOptions struct {
	// Authfile is the path to the authentication file.
	Authfile *string `schema:"-"`
	// Password for authenticating against the registry.
	Password *string `schema:"-"`
	// Quiet can be specified to suppress pull progress when pulling.
	Quiet *bool
	// Retry number of times to retry pull in case of failure
	Retry *uint
	// RetryDelay between retries in case of pull failures
	RetryDelay *string
	// SkipTLSVerify to skip HTTPS and certificate verification.
	TlsVerify *bool
	// Username for authenticating against the registry.
	Username *string `schema:"-"`
}

// RemoveOptions are optional options for removing images
//
//go:generate go run ../generator/generator.go RemoveOptions
type RemoveOptions struct {
	// Remove all artifacts
	All *bool
	// Artifacts is a list of Artifact IDs or names to remove
	Artifacts []string
	// Ignore errors if IDs or names are not defined
	Ignore *bool
}

// AddOptions are optional options for removing images
//
//go:generate go run ../generator/generator.go AddOptions
type AddOptions struct {
	Annotations      []string
	ArtifactMIMEType *string
	Append           *bool
	FileMIMEType     *string
	Replace          *bool
}

// ExtractOptions
//
//go:generate go run ../generator/generator.go ExtractOptions
type ExtractOptions struct {
	// Title annotation value to extract only a single blob matching that name.
	// Conflicts with Digest. Optional.
	Title *string
	// Digest of the blob to extract.
	// Conflicts with Title. Optional.
	Digest *string
	// ExcludeTitle option allows single blobs to be exported
	// with their title/filename empty. Optional.
	// Default: False
	ExcludeTitle *bool
}

// ListOptions
//
//go:generate go run ../generator/generator.go ListOptions
type ListOptions struct{}

// InspectOptions
//
//go:generate go run ../generator/generator.go InspectOptions
type InspectOptions struct{}
