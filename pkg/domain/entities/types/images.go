package types

import (
	"time"

	"go.podman.io/podman/v6/pkg/inspect"
	"go.podman.io/podman/v6/pkg/trust"
)

// swagger:model LibpodImageSummary
type ImageSummary struct {
	ID          string `json:"Id"`
	ParentId    string
	RepoTags    []string
	RepoDigests []string
	Created     int64
	Size        int64
	SharedSize  int
	VirtualSize int64 `json:",omitempty"`
	Labels      map[string]string
	Containers  int
	ReadOnly    bool `json:",omitempty"`
	Dangling    bool `json:",omitempty"`

	// Podman extensions
	Arch    string   `json:",omitempty"`
	Digest  string   `json:",omitempty"`
	History []string `json:",omitempty"`
	// IsManifestList is a ptr so we can distinguish between a true
	// json empty response and false.  the docker compat side needs to return
	// empty; where as the libpod side needs a value of true or false
	IsManifestList *bool    `json:",omitempty"`
	Names          []string `json:",omitempty"`
	Os             string   `json:",omitempty"`
}

func (i *ImageSummary) Id() string {
	return i.ID
}

func (i *ImageSummary) IsReadOnly() bool {
	return i.ReadOnly
}

func (i *ImageSummary) IsDangling() bool {
	return i.Dangling
}

type ImageInspectReport struct {
	*inspect.ImageData
}

type ImageTreeReport struct {
	Tree string // TODO: Refactor move presentation work out of server
}

type ImageLoadReport struct {
	Names []string
}

type ImageImportReport struct {
	Id string
}

// ImageSearchTrue is the string representation of true for
// the Official and Automated fields in ImageSearchReport.
const ImageSearchTrue = "[OK]"

// ImageSearchReport is the response from searching images.
type ImageSearchReport struct {
	// Index is the image index (e.g., "docker.io" or "quay.io")
	Index string
	// Name is the canonical name of the image (e.g., "docker.io/library/alpine").
	Name string
	// Description of the image.
	Description string
	// Stars is the number of stars of the image.
	Stars int
	// Official indicates if it's an official image.
	// ImageSearchTrue represents true and "" is false.
	Official string
	// Automated indicates if the image was created by an automated build.
	// ImageSearchTrue represents true and "" is false.
	Automated string
	// Tag is the repository tag
	Tag string
}

// ShowTrustReport describes the results of show trust
type ShowTrustReport struct {
	Raw                     []byte
	SystemRegistriesDirPath string
	JSONOutput              []byte
	Policies                []*trust.Policy
}

// ImageMountReport describes the response from image mount
type ImageMountReport struct {
	Id           string
	Name         string
	Repositories []string
	Path         string
}

// ImageUnmountReport describes the response from umounting an image
type ImageUnmountReport struct {
	Err error
	Id  string
}

// FarmInspectReport describes the response from farm inspect
type FarmInspectReport struct {
	NativePlatforms   []string
	EmulatedPlatforms []string
	OS                string
	Arch              string
	Variant           string
}

// ImageRemoveReport is the response for removing one or more image(s) from storage
// and images what was untagged vs actually removed.
type ImageRemoveReport struct {
	// Deleted images.
	Deleted []string `json:",omitempty"`
	// Untagged images. Can be longer than Deleted.
	Untagged []string `json:",omitempty"`
	// ExitCode describes the exit codes as described in the `podman rmi`
	// man page.
	ExitCode int
}

type ImageHistoryLayer struct {
	ID        string    `json:"id"`
	Created   time.Time `json:"created"`
	CreatedBy string    `json:",omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Size      int64     `json:"size"`
	Comment   string    `json:"comment,omitempty"`
}

type ImageHistoryReport struct {
	Layers []ImageHistoryLayer
}

// swagger:alias
type ImagePullStatus string

const (
	ImagePullStatusPulling ImagePullStatus = "pulling"
	ImagePullStatusSuccess ImagePullStatus = "success"
	ImagePullStatusError   ImagePullStatus = "error"
)

type ImagePullReport struct {
	// Status contains the status of the image pull.
	// Populated when streaming is enabled.
	//
	// Possible values:
	//
	// "pulling": image pull is in progress
	//
	// "success": image pull has completed successfully
	//
	// "error": image pull has encountered an error
	Status ImagePullStatus `json:"status,omitempty"`
	// Stream used to provide output from c/image.
	// Populated when streaming is enabled and image status is "pulling".
	Stream string `json:"stream,omitempty"`
	// Error contains text of errors from c/image.
	// Populated when streaming is enabled and image status is "error".
	Error string `json:"error,omitempty"`
	// Images contains the ID's of the images pulled
	Images []string `json:"images,omitempty"`
	// ID contains image id (retained for backwards compatibility)
	ID string `json:"id,omitempty"`
	// Progress contains the information about the progress of the artifact pull.
	// Populated when streaming is enabled, image status is "pulling",
	// and there is pull progress to report.
	Progress *ArtifactPullProgress `json:"pullProgress,omitempty"`
}

// swagger:alias
type ArtifactPullStatus string

const (
	ArtifactPullStatusPulling ArtifactPullStatus = "pulling"
	ArtifactPullStatusSuccess ArtifactPullStatus = "success"
	ArtifactPullStatusSkipped ArtifactPullStatus = "skipped"
)

// Information about the progress of the artifact pull.
// Populated when streaming is enabled, image status is "pulling",
// and there is pull progress to report.
type ArtifactPullProgress struct {
	// Status contains the status of the artifact pull.
	//
	// Possible values:
	//
	// "pulling": artifact pull is in progress
	//
	// "success": artifact pull has completed successfully
	//
	// "skipped": artifact pull has been skipped because the artifact is already available at the destination
	Status ArtifactPullStatus `json:"status,omitempty"`
	// Current is the number of bytes of the current artifact that have been
	// transferred so far.
	// Populated when artifact status is "pulling" or "success".
	Current uint64 `json:"current,omitempty"`
	// Total is the total size of the artifact in bytes. A value of -1
	// indicates that the total size is unknown.
	// Populated when artifact status is "pulling" or "success".
	Total int64 `json:"total,omitempty"`
	// ProgressComponentID is the unique identifier for the artifact being pulled.
	// A value of "" indicates that the progress component ID is unknown.
	ProgressComponentID string `json:"progressComponentID,omitempty"`
}

type ImagePushStream struct {
	// ManifestDigest is the digest of the manifest of the pushed image.
	ManifestDigest string `json:"manifestdigest,omitempty"`
	// Stream used to provide push progress
	Stream string `json:"stream,omitempty"`
	// Error contains text of errors from pushing
	Error string `json:"error,omitempty"`
}
