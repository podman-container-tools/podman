package types

import (
	"errors"
)

var (
	ErrArtifactUnnamed          = errors.New("artifact is unnamed")
	ErrArtifactNotExist         = errors.New("artifact does not exist")
	ErrArtifactAlreadyExists    = errors.New("artifact already exists")
	ErrArtifactFileExists       = errors.New("file already exists in artifact")
	ErrArtifactBlobTitleInvalid = errors.New("artifact blob title invalid")

	// ErrTaggedAndDigested refers to a reference that is both tagged and digested.
	// For example: quay.io/foo/bar:tag@sha256:1234...
	ErrTaggedAndDigested = errors.New("reference cannot be tagged and digested")
)
