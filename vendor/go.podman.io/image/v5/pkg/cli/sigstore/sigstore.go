package sigstore

import (
	"errors"
	"fmt"
	"io"
	"net/url"

	"go.podman.io/image/v5/pkg/cli"
	"go.podman.io/image/v5/pkg/cli/sigstore/params"
	"go.podman.io/image/v5/signature/signer"
	"go.podman.io/image/v5/signature/sigstore"
	"go.podman.io/image/v5/signature/sigstore/fulcio"
	"go.podman.io/image/v5/signature/sigstore/rekor"
)

// Options collects data that the caller should provide to NewSignerFromParameterFile.
// The caller should set all fields unless documented otherwise.
type Options struct {
	PrivateKeyPassphrasePrompt func(keyFile string) (string, error) // A function to call to interactively prompt for a passphrase
	Stdin                      io.Reader
	Stdout                     io.Writer
}

// NewSignerFromParameterFile returns a signature.Signer which creates sigstore signatures based a parameter file at the specified path.
//
// The caller must call Close() on the returned Signer.
func NewSignerFromParameterFile(path string, options *Options) (*signer.Signer, error) {
	params, err := params.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("setting up signing using parameter file %q: %w", path, err)
	}
	return newSignerFromParameterData(params, options)
}

// newSignerFromParameterData returns a signature.Signer which creates sigstore signatures based on parameter file contents.
//
// The caller must call Close() on the returned Signer.
func newSignerFromParameterData(params *params.SigningParameterFile, options *Options) (*signer.Signer, error) {
	opts := []sigstore.Option{}
	if params.PrivateKeyFile != "" {
		var getPassphrase func(keyFile string) (string, error)
		switch {
		case params.PrivateKeyPassphraseFile != "":
			getPassphrase = func(_ string) (string, error) {
				return cli.ReadPassphraseFile(params.PrivateKeyPassphraseFile)
			}
		case options.PrivateKeyPassphrasePrompt != nil:
			getPassphrase = options.PrivateKeyPassphrasePrompt
		default: // This shouldn’t happen, the caller is expected to set options.PrivateKeyPassphrasePrompt
			return nil, fmt.Errorf("private key %s specified, but no way to get a passphrase", params.PrivateKeyFile)
		}
		passphrase, err := getPassphrase(params.PrivateKeyFile)
		if err != nil {
			return nil, err
		}
		opts = append(opts, sigstore.WithPrivateKeyFile(params.PrivateKeyFile, []byte(passphrase)))
	}

	if params.Fulcio != nil {
		fulcioOpt, err := fulcioOption(params.Fulcio, options)
		if err != nil {
			return nil, err
		}
		opts = append(opts, fulcioOpt)
	}

	if params.RekorURL != "" {
		rekorURL, err := url.Parse(params.RekorURL)
		if err != nil {
			return nil, fmt.Errorf("parsing rekorURL %q: %w", params.RekorURL, err)
		}
		opts = append(opts, rekor.WithRekor(rekorURL))
	}

	return sigstore.NewSigner(opts...)
}

// fulcioOption returns a sigstore.Option for Fulcio use based on f.
func fulcioOption(f *params.SigningParameterFileFulcio, options *Options) (sigstore.Option, error) {
	if f.FulcioURL == "" {
		return nil, errors.New("missing fulcioURL")
	}
	fulcioURL, err := url.Parse(f.FulcioURL)
	if err != nil {
		return nil, fmt.Errorf("parsing fulcioURL %q: %w", f.FulcioURL, err)
	}

	if f.OIDCMode == params.OIDCModeStaticToken {
		if f.OIDCIDToken == "" {
			return nil, errors.New("missing oidcToken")
		}
		return fulcio.WithFulcioAndPreexistingOIDCIDToken(fulcioURL, f.OIDCIDToken), nil
	}

	if f.OIDCIssuerURL == "" {
		return nil, errors.New("missing oidcIssuerURL")
	}
	oidcIssuerURL, err := url.Parse(f.OIDCIssuerURL)
	if err != nil {
		return nil, fmt.Errorf("parsing oidcIssuerURL %q: %w", f.OIDCIssuerURL, err)
	}
	switch f.OIDCMode {
	case params.OIDCModeDeviceGrant:
		return fulcio.WithFulcioAndDeviceAuthorizationGrantOIDC(fulcioURL, oidcIssuerURL, f.OIDCClientID, f.OIDCClientSecret,
			options.Stdout), nil
	case params.OIDCModeInteractive:
		return fulcio.WithFulcioAndInteractiveOIDC(fulcioURL, oidcIssuerURL, f.OIDCClientID, f.OIDCClientSecret,
			options.Stdin, options.Stdout), nil
	case "":
		return nil, errors.New("missing oidcMode")
	case params.OIDCModeStaticToken:
		return nil, errors.New("internal inconsistency: SigningParameterFileOIDCModeStaticToken was supposed to already be handled")
	default:
		return nil, fmt.Errorf("unknown oidcMode value %q", f.OIDCMode)
	}
}
