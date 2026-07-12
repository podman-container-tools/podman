//go:build !remote

package abi

import (
	"encoding/json"
	"errors"
	"io"

	"go.podman.io/image/v5/manifest"
)

// DecodeOverrideConfig reads a Schema2Config from a Reader, suppressing EOF
// errors.
func DecodeOverrideConfig(reader io.Reader) (*manifest.Schema2Config, error) {
	config := manifest.Schema2Config{}
	if reader != nil {
		err := json.NewDecoder(reader).Decode(&config)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
	}
	return &config, nil
}
