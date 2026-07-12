package ocipull

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"go.podman.io/buildah/pkg/parse"
	"go.podman.io/image/v5/copy"
	"go.podman.io/image/v5/oci/layout"
	"go.podman.io/image/v5/signature"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/storage/pkg/configfile"
)

// pullOptions includes data to alter certain knobs when pulling a source
// image.
type pullOptions struct {
	// Skip TLS verification when accessing the registry.
	skipTLSVerify types.OptionalBool
	// [username[:password] to use when connecting to the registry.
	credentials string
	// Quiet the progress bars when pushing.
	quiet bool
}

// systemContext returns an appropriate types.SystemContext for options.
func (opts *pullOptions) systemContext() (*types.SystemContext, error) {
	sys := types.SystemContext{
		DockerInsecureSkipTLSVerify: opts.skipTLSVerify,
	}
	if opts.credentials != "" {
		authConf, err := parse.AuthConfig(opts.credentials)
		if err != nil {
			return nil, err
		}
		sys.DockerAuthConfig = authConf
	}
	return &sys, nil
}

// noSignaturePolicy is a default policy if policy.json is not found on
// the host machine.
var noSignaturePolicy string = `{"default":[{"type":"insecureAcceptAnything"}]}`

// pull `imageInput` from a container registry to `sourcePath`.
func pull(ctx context.Context, imageInput types.ImageReference, localDestPath *define.VMFile, options *pullOptions) error {
	destRef, err := layout.ParseReference(localDestPath.GetPath())
	if err != nil {
		return err
	}

	sysCtx, err := options.systemContext()
	if err != nil {
		return err
	}

	policy, err := signature.DefaultPolicy(sysCtx)
	if err != nil {
		if !errors.Is(err, configfile.ErrConfigFileNotFound) {
			return fmt.Errorf("reading signature policy: %w", err)
		}

		// If no policy has been found yet, we use a no signature policy automatically
		logrus.Debug("no signature policy file found: using default allow everything signature policy")
		policy, err = signature.NewPolicyFromBytes([]byte(noSignaturePolicy))
		if err != nil {
			return fmt.Errorf("obtaining signature policy: %w", err)
		}
	}

	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return fmt.Errorf("creating new signature policy context: %w", err)
	}

	copyOpts := copy.Options{
		SourceCtx:      sysCtx,
		DestinationCtx: sysCtx,
	}
	if !options.quiet {
		copyOpts.ReportWriter = os.Stderr
	}
	if _, err := copy.Image(ctx, policyContext, destRef, imageInput, &copyOpts); err != nil {
		return fmt.Errorf("pulling source image: %w", err)
	}

	return nil
}
