// Package basetls encapsulates a set of base TLS settings (not keys/certificates)
// configured via containers-tls-details.yaml(5).
//
// CLI integration should generally be done using c/image/pkg/cli/basetls/tlsdetails instead
// of using the TLSDetailsFile directly.
package basetls

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// Config encapsulates user’s choices about base TLS settings, typically
// configured via containers-tls-details.yaml(5).
//
// Most codebases should pass around the resulting *tls.Config, without depending on this subpackage;
// this primarily exists as a separate type to allow passing the configuration around within (version-matched) RPC systems,
// using the MarshalText/UnmarshalText methods.
type Config struct {
	// We keep the text representation because we start with it, and this way we don't have
	// to implement formatting back to text. This is an internal detail, so we can change that later.
	text   TLSDetailsFile
	config *tls.Config // Parsed from .text, both match
}

// TLSDetailsFile contains a set of TLS options.
//
// To consume such a file, most callers should use c/image/pkg/cli/basetls/tlsdetails instead
// of dealing with this type explicitly.
//
// This type is exported primarily to allow creating parameter files programmatically
// (and eventually the tlsdetails subpackage should provide an API to convert this type into
// the appropriate file contents, so that callers don't need to do that manually).
type TLSDetailsFile struct {
	// Keep this in sync with docs/containers-tls-details.yaml.5.md !

	MinVersion   string   `yaml:"minVersion,omitempty"`   // If set, minimum version to use throughout the program.
	CipherSuites []string `yaml:"cipherSuites,omitempty"` // If set, allowed TLS cipher suites to use throughout the program.
	NamedGroups  []string `yaml:"namedGroups,omitempty"`  // If set, allowed TLS named groups to use throughout the program.
}

// NewFromTLSDetails creates a Config from a TLSDetailsFile.
func NewFromTLSDetails(details *TLSDetailsFile) (*Config, error) {
	res := Config{
		text:   TLSDetailsFile{},
		config: &tls.Config{},
	}
	configChanged := false
	for _, fn := range []func(input *TLSDetailsFile) (bool, error){
		res.parseMinVersion,
		res.parseCipherSuites,
		res.parseNamedGroups,
	} {
		changed, err := fn(details)
		if err != nil {
			return nil, err
		}
		if changed {
			configChanged = true
		}
	}

	if !configChanged {
		res.config = nil
	}
	return &res, nil
}

// tlsVersions maps TLS version strings to their crypto/tls constants.
// We could use the `tls.VersionName` names, but those are verbose and contain spaces;
// similarly the OpenShift enum values (“VersionTLS11”) are unergonomic.
var tlsVersions = map[string]uint16{
	"1.0": tls.VersionTLS10,
	"1.1": tls.VersionTLS11,
	"1.2": tls.VersionTLS12,
	"1.3": tls.VersionTLS13,
}

func (c *Config) parseMinVersion(input *TLSDetailsFile) (bool, error) {
	if input.MinVersion == "" {
		return false, nil
	}
	v, ok := tlsVersions[input.MinVersion]
	if !ok {
		return false, fmt.Errorf("unrecognized TLS minimum version %q", input.MinVersion)
	}
	c.text.MinVersion = input.MinVersion
	c.config.MinVersion = v
	return true, nil
}

// cipherSuitesByName returns a map from cipher suite name to its ID.
func cipherSuitesByName() map[string]uint16 {
	// The Go standard library uses IANA names and already contains the mapping (for relevant values)
	// sadly we still need to turn it into a lookup map.
	suites := make(map[string]uint16)
	for _, cs := range tls.CipherSuites() {
		suites[cs.Name] = cs.ID
	}
	for _, cs := range tls.InsecureCipherSuites() {
		suites[cs.Name] = cs.ID
	}
	return suites
}

func (c *Config) parseCipherSuites(input *TLSDetailsFile) (bool, error) {
	if input.CipherSuites == nil {
		return false, nil
	}
	suitesByName := cipherSuitesByName()
	ids := []uint16{}
	for _, name := range input.CipherSuites {
		id, ok := suitesByName[name]
		if !ok {
			return false, fmt.Errorf("unrecognized TLS cipher suite %q", name)
		}
		ids = append(ids, id)
	}
	c.text.CipherSuites = slices.Clone(input.CipherSuites)
	c.config.CipherSuites = ids
	return true, nil
}

// groupsByName maps curve/group names to their tls.CurveID.
// The names match IANA TLS Supported Groups registry.
//
// Yes, the x25519 names differ in capitalization.
// Go’s tls.CurveID has a .String() method, but it
// uses the Go names.
var groupsByName = map[string]tls.CurveID{
	"secp256r1":      tls.CurveP256,
	"secp384r1":      tls.CurveP384,
	"secp521r1":      tls.CurveP521,
	"x25519":         tls.X25519,
	"X25519MLKEM768": tls.X25519MLKEM768,
}

func (c *Config) parseNamedGroups(input *TLSDetailsFile) (bool, error) {
	if input.NamedGroups == nil {
		return false, nil
	}
	ids := []tls.CurveID{}
	for _, name := range input.NamedGroups {
		id, ok := groupsByName[name]
		if !ok {
			return false, fmt.Errorf("unrecognized TLS named group %q", name)
		}
		ids = append(ids, id)
	}
	c.text.NamedGroups = slices.Clone(input.NamedGroups)
	c.config.CurvePreferences = ids
	return true, nil
}

// TLSConfig returns a *tls.Config matching the provided settings.
// If c contains no settings, it returns nil.
// Otherwise, the returned *tls.Config is freshly allocated and the caller can modify it as needed.
func (c *Config) TLSConfig() *tls.Config {
	if c.config == nil {
		return nil
	}
	return c.config.Clone()
}

// marshaledSerialization is the data we use in MarshalText/UnmarshalText,
// marshaled using JSON.
//
// Note that the file format is using YAML, but we use JSON, to minimize dependencies
// in backend code where we don't need comments and the brackets are not annoying users.
type marshaledSerialization struct {
	Version int
	Data    TLSDetailsFile
}

const marshaledSerializationVersion1 = 1

// MarshalText serializes c to a text representation.
//
// The representation is intended to be reasonably stable across updates to c/image,
// but the consumer must not be older than the producer.
func (c Config) MarshalText() ([]byte, error) {
	data := marshaledSerialization{
		Version: marshaledSerializationVersion1,
		Data:    c.text,
	}
	return json.Marshal(data)
}

// UnmarshalText parses the output of MarshalText.
//
// The format is otherwise undocumented and we do not promise ongoing compatibility with producers external to this package.
func (c *Config) UnmarshalText(text []byte) error {
	var data marshaledSerialization

	// In the future, this should be an even stricter parser, e.g. refusing duplicate fields
	// and requiring a case-sensitive field name match.
	decoder := json.NewDecoder(bytes.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&data); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("unexpected extra data after a JSON object")
	}

	if data.Version != marshaledSerializationVersion1 {
		return fmt.Errorf("unsupported version %d", data.Version)
	}
	v, err := NewFromTLSDetails(&data.Data)
	if err != nil {
		return err
	}
	*c = *v
	return nil
}
