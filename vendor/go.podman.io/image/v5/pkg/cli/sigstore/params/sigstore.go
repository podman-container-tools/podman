package params

import (
	"bytes"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

// SigningParameterFile collects parameters used for creating sigstore signatures.
//
// To consume such a file, most callers should use c/image/pkg/cli/sigstore instead
// of dealing with this type explicitly using ParseFile.
//
// This type is exported primarily to allow creating parameter files programmatically
// (and eventually this subpackage should provide an API to convert this type into
// the appropriate file contents, so that callers don’t need to do that manually).
type SigningParameterFile struct {
	// Keep this in sync with docs/containers-sigstore-signing-params.yaml.5.md !

	PrivateKeyFile           string `yaml:"privateKeyFile,omitempty"`           // If set, sign using a private key stored in this file.
	PrivateKeyPassphraseFile string `yaml:"privateKeyPassphraseFile,omitempty"` // A file that contains the passprase required for PrivateKeyFile.

	Fulcio *SigningParameterFileFulcio `yaml:"fulcio,omitempty"` // If set, sign using a short-lived key and a Fulcio-issued certificate.

	RekorURL string `yaml:"rekorURL,omitempty"` // If set, upload the signature to the specified Rekor server, and include a log inclusion proof in the signature.
}

// SigningParameterFileFulcio is a subset of SigningParameterFile dedicated to Fulcio parameters.
type SigningParameterFileFulcio struct {
	// Keep this in sync with docs/containers-sigstore-signing-params.yaml.5.md !

	FulcioURL string `yaml:"fulcioURL,omitempty"` // URL of the Fulcio server. Required.

	// How to obtain the OIDC ID token required by Fulcio. Required.
	OIDCMode OIDCMode `yaml:"oidcMode,omitempty"`

	// oidcMode = staticToken
	OIDCIDToken string `yaml:"oidcIDToken,omitempty"`

	// oidcMode = deviceGrant || interactive
	OIDCIssuerURL    string `yaml:"oidcIssuerURL,omitempty"` //
	OIDCClientID     string `yaml:"oidcClientID,omitempty"`
	OIDCClientSecret string `yaml:"oidcClientSecret,omitempty"`
}

type OIDCMode string

const (
	// OIDCModeStaticToken means the parameter file contains an user-provided OIDC ID token value.
	OIDCModeStaticToken OIDCMode = "staticToken"
	// OIDCModeDeviceGrant specifies the OIDC ID token should be obtained using a device authorization grant (RFC 8628).
	OIDCModeDeviceGrant OIDCMode = "deviceGrant"
	// OIDCModeInteractive specifies the OIDC ID token should be obtained interactively (automatically opening a browser,
	// or interactively prompting the user.)
	OIDCModeInteractive OIDCMode = "interactive"
)

// ParseFile parses a SigningParameterFile at the specified path.
//
// Most consumers of the parameter file should use c/image/pkg/cli/sigstore to obtain a *signer.Signer instead.
func ParseFile(path string) (*SigningParameterFile, error) {
	var res SigningParameterFile
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
