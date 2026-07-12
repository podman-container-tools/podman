//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"fmt"
	"os"

	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/trust"
	"go.podman.io/storage/pkg/configfile"
)

// policyPathFromConfigfile resolves policy.json the same way as [signature.DefaultPolicy]
// (via [configfile.Read]); overridePath, if non-empty, wins.
func policyPathFromConfigfile(sys *types.SystemContext, overridePath string) (string, error) {
	if overridePath != "" {
		return overridePath, nil
	}
	if sys != nil && sys.SignaturePolicyPath != "" {
		return sys.SignaturePolicyPath, nil
	}

	root := ""
	if sys != nil {
		root = sys.RootForImplicitAbsolutePaths
	}

	policyFiles := configfile.File{
		Name:                         "policy",
		Extension:                    "json",
		DoNotLoadDropInFiles:         true,
		EnvironmentName:              "CONTAINERS_POLICY_JSON",
		RootForImplicitAbsolutePaths: root,
		ErrorIfNotFound:              true,
	}

	for item, err := range configfile.Read(&policyFiles) {
		if err != nil {
			return "", err
		}
		if item != nil {
			return item.Name, nil
		}
	}
	return "", fmt.Errorf("internal error: empty result from configfile.Read while resolving policy path")
}

func (ir *ImageEngine) ShowTrust(_ context.Context, _ []string, options entities.ShowTrustOptions) (*entities.ShowTrustReport, error) {
	var (
		err    error
		report entities.ShowTrustReport
	)
	policyPath, err := policyPathFromConfigfile(ir.Libpod.SystemContext(), options.PolicyPath)
	if err != nil {
		return nil, err
	}
	report.Raw, err = os.ReadFile(policyPath)
	if err != nil {
		return nil, err
	}
	if options.Raw {
		return &report, nil
	}
	report.SystemRegistriesDirPath = trust.RegistriesDirPath(ir.Libpod.SystemContext())
	if len(options.RegistryPath) > 0 {
		report.SystemRegistriesDirPath = options.RegistryPath
	}
	report.Policies, err = trust.PolicyDescription(policyPath, report.SystemRegistriesDirPath)
	if err != nil {
		return nil, fmt.Errorf("could not show trust policies: %w", err)
	}
	return &report, nil
}

func (ir *ImageEngine) SetTrust(_ context.Context, args []string, options entities.SetTrustOptions) error {
	if len(args) != 1 {
		return fmt.Errorf("SetTrust called with unexpected %d args", len(args))
	}
	if options.PolicyPath == "" {
		return fmt.Errorf("signature-policy path must be provided")
	}
	scope := args[0]

	return trust.AddPolicyEntries(options.PolicyPath, trust.AddPolicyEntriesInput{
		Scope:       scope,
		Type:        options.Type,
		PubKeyFiles: options.PubKeysFile,
	})
}
