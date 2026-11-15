// Package tlsdetails implements the containers-tls-details.yaml(5) file format.
//
// Recommended CLI integration is by a --tls-details flag parsed using BaseTLSFromOptionalFile, with the following documentation:
//
//	--tls-details is a path to a containers-tls-details.yaml(5) file, affecting TLS behavior throughout the program.
//
//	If not set, defaults to a reasonable default that may change over time (depending on system’s global policy,
//	version of the program, version of the Go language, and the like).
//
//	Users should generally not use this option unless they have a process to ensure that the configuration will be kept up to date.
package tlsdetails

import (
	"bytes"
	"fmt"
	"os"

	"go.podman.io/image/v5/pkg/cli/basetls"
	"go.yaml.in/yaml/v3"
)

// BaseTLSFromOptionalFile returns a basetls.Config matching a containers-tls-details.yaml file at the specified path.
// If path is "", it returns a valid basetls.Config with no settings (where config.TLSConfig() will return nil).
func BaseTLSFromOptionalFile(path string) (*basetls.Config, error) {
	if path == "" {
		return basetls.NewFromTLSDetails(&basetls.TLSDetailsFile{})
	}
	return BaseTLSFromFile(path)
}

// BaseTLSFromFile returns a basetls.Config matching a containers-tls-details.yaml file at the specified path.
func BaseTLSFromFile(path string) (*basetls.Config, error) {
	details, err := ParseFile(path)
	if err != nil {
		return nil, err
	}
	res, err := basetls.NewFromTLSDetails(details)
	if err != nil {
		return nil, fmt.Errorf("parsing TLS details %q: %w", path, err)
	}
	return res, nil
}

// ParseFile parses a basetls.TLSDetailsFile at the specified path.
//
// Most consumers of the parameter file should use BaseTLSFromFile or BaseTLSFromOptionalFile instead.
func ParseFile(path string) (*basetls.TLSDetailsFile, error) {
	var res basetls.TLSDetailsFile
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(source))
	dec.KnownFields(true)
	if err = dec.Decode(&res); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", path, err)
	}
	return &res, nil
}
