package entities

import (
	"net/url"

	"go.podman.io/common/pkg/ssh"
)

// ScpTransferImageOptions provide options for securely copying images to and from a remote host
type ScpTransferImageOptions struct {
	// Remote determines if this entity is operating on a remote machine
	Remote bool `json:"remote,omitempty"`
	// File is the input/output file for the save and load Operation
	File string `json:"file,omitempty"`
	// Quiet Determines if the save and load operation will be done quietly
	Quiet bool `json:"quiet,omitempty"`
	// Image is the image the user is providing to save and load
	Image string `json:"image,omitempty"`
	// User is used in conjunction with Transfer to determine if a valid user was given to save from/load into
	User string `json:"user,omitempty"`
	// Tag is the name to be used for the image on the destination
	Tag string `json:"tag,omitempty"`
}

type ScpLoadReport = ImageLoadReport

type ScpExecuteTransferOptions struct {
	// ParentFlags are the arguments to apply to the parent podman command when called via ssh
	ParentFlags []string
	// Quiet Determines if the save and load operation will be done quietly
	Quiet bool
	// SSHMode is the specified ssh.EngineMode which should be used
	SSHMode ssh.EngineMode
	// SaveFormat is the format for podman save (oci-archive or docker-archive). Empty means default of podman save (docker-archive).
	SaveFormat string
}

type ScpExecuteTransferReport struct {
	// LoadReport provides results from calling podman load
	LoadReport *ScpLoadReport
	// Source contains data relating to the source of the image to transfer
	Source *ScpTransferImageOptions
	// Dest contains data relating to the destination of the image to transfer
	Dest *ScpTransferImageOptions
	// ParentFlags are the arguments to apply to the parent podman command when called via ssh
	ParentFlags []string
	// SaveFormat is the format to use for podman save when transferring (oci-archive or docker-archive)
	SaveFormat string
}

type ScpTransferOptions struct {
	// ParentFlags are the arguments to apply to the parent podman command when called.
	ParentFlags []string
	// SaveFormat is the format for podman save (oci-archive or docker-archive)
	SaveFormat string
}

type ScpTransferReport struct{}

type ScpLoadToRemoteOptions struct {
	// Dest contains data relating to the destination of the image to transfer
	Dest ScpTransferImageOptions
	// LocalFile is a path to a local file containing saved image data to transfer
	LocalFile string
	// Tag is the name of the tag to be given to the loaded image (unused)
	Tag string
	// URL points to the remote location for loading to
	URL *url.URL
	// Iden is a path to an optional identity file with ssh key
	Iden string
	// SSHMode is the specified ssh.EngineMode which should be used
	SSHMode ssh.EngineMode
}

type ScpLoadToRemoteReport struct {
	// Response contains any additional information from the executed load command
	Response string
	// ID is the identifier of the loaded image
	ID string
}

type ScpSaveToRemoteOptions struct {
	Image string
	// LocalFile is a path to a local file to copy the saved image to
	LocalFile string
	// Tag is the name of the tag to be given to the saved image (unused)
	Tag string
	// URL points to the remote location for saving from
	URL *url.URL
	// Iden is a path to an optional identity file with ssh key
	Iden string
	// SSHMode is the specified ssh.EngineMode which should be used
	SSHMode ssh.EngineMode
	// Format is the save format (oci-archive or docker-archive). Empty means default of podman save (docker-archive).
	Format string
}

type ScpSaveToRemoteReport struct{}
