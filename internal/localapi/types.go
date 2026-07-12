package localapi

import "errors"

// LocalAPIMap is a map of local paths to their target paths in the VM
type LocalAPIMap struct {
	ClientPath string `json:"ClientPath,omitempty"`
	RemotePath string `json:"RemotePath,omitempty"`
}

var ErrPathNotAbsolute = errors.New("path is not absolute")
