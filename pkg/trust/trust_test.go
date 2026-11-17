package trust

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/signature"
)

func TestPolicyDescription(t *testing.T) {
	tempDir := t.TempDir()
	policyPath := filepath.Join(tempDir, "policy.json")

	// Override getGPGIdFromKeyPath because we don't want to bother with (and spend the unit-test time on) generating valid GPG keys, and running the real GPG binary.
	// Instead of reading the files at all, just expect file names like /id1,id2,...,idN.pub
	idReader := func(keyPath string) []string {
		require.True(t, strings.HasPrefix(keyPath, "/"))
		require.True(t, strings.HasSuffix(keyPath, ".pub"))
		return strings.Split(keyPath[1:len(keyPath)-4], ",")
	}

	for _, c := range []struct {
		policy   *signature.Policy
		expected []*Policy
	}{
		{
			&signature.Policy{
				Default: signature.PolicyRequirements{
					signature.NewPRReject(),
				},
				Transports: map[string]signature.PolicyTransportScopes{
					"docker": {
						"quay.io/accepted": {
							signature.NewPRInsecureAcceptAnything(),
						},
						"registry.redhat.io": {
							xNewPRSignedByKeyPath(t, "/redhat.pub", signature.NewPRMMatchRepoDigestOrExact()),
						},
						"registry.access.redhat.com": {
							xNewPRSignedByKeyPaths(t, []string{"/redhat.pub", "/redhat-beta.pub"}, signature.NewPRMMatchRepoDigestOrExact()),
						},
						"quay.io/multi-signed": {
							xNewPRSignedByKeyPath(t, "/1.pub", signature.NewPRMMatchRepoDigestOrExact()),
							xNewPRSignedByKeyPath(t, "/2,3.pub", signature.NewPRMMatchRepoDigestOrExact()),
						},
						"quay.io/sigstore-signed": {
							xNewPRSigstoreSignedKeyPath(t, "/1.pub", signature.NewPRMMatchRepoDigestOrExact()),
							xNewPRSigstoreSignedKeyPath(t, "/2.pub", signature.NewPRMMatchRepoDigestOrExact()),
						},
					},
				},
			},
			[]*Policy{
				{
					Transport: "all",
					Name:      "* (default)",
					RepoName:  "default",
					Type:      "reject",
				},
				{
					Transport: "repository",
					Name:      "quay.io/accepted",
					RepoName:  "quay.io/accepted",
					Type:      "accept",
				},
				{
					Transport:      "repository",
					Name:           "quay.io/multi-signed",
					RepoName:       "quay.io/multi-signed",
					Type:           "signed",
					SignatureStore: "https://quay.example.com/sigstore",
					GPGId:          "1",
				},
				{
					Transport:      "repository",
					Name:           "quay.io/multi-signed",
					RepoName:       "quay.io/multi-signed",
					Type:           "signed",
					SignatureStore: "https://quay.example.com/sigstore",
					GPGId:          "2, 3",
				},
				{
					Transport:      "repository",
					Name:           "quay.io/sigstore-signed",
					RepoName:       "quay.io/sigstore-signed",
					Type:           "sigstoreSigned",
					SignatureStore: "",
					GPGId:          "N/A",
				},
				{
					Transport:      "repository",
					Name:           "quay.io/sigstore-signed",
					RepoName:       "quay.io/sigstore-signed",
					Type:           "sigstoreSigned",
					SignatureStore: "",
					GPGId:          "N/A",
				},
				{
					Transport:      "repository",
					Name:           "registry.access.redhat.com",
					RepoName:       "registry.access.redhat.com",
					Type:           "signed",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					GPGId:          "redhat, redhat-beta",
				},
				{
					Transport:      "repository",
					Name:           "registry.redhat.io",
					RepoName:       "registry.redhat.io",
					Type:           "signed",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					GPGId:          "redhat",
				},
			},
		},
		{
			&signature.Policy{
				Default: signature.PolicyRequirements{
					xNewPRSignedByKeyPath(t, "/1.pub", signature.NewPRMMatchRepoDigestOrExact()),
					xNewPRSignedByKeyPath(t, "/2,3.pub", signature.NewPRMMatchRepoDigestOrExact()),
				},
			},
			[]*Policy{
				{
					Transport:      "all",
					Name:           "* (default)",
					RepoName:       "default",
					Type:           "signed",
					SignatureStore: "",
					GPGId:          "1",
				},
				{
					Transport:      "all",
					Name:           "* (default)",
					RepoName:       "default",
					Type:           "signed",
					SignatureStore: "",
					GPGId:          "2, 3",
				},
			},
		},
	} {
		policyJSON, err := json.Marshal(c.policy)
		require.NoError(t, err)
		err = os.WriteFile(policyPath, policyJSON, 0o600)
		require.NoError(t, err)

		res, err := policyDescriptionWithGPGIDReader(policyPath, "./testdata", idReader)
		require.NoError(t, err)
		assert.Equal(t, c.expected, res)
	}
}

func TestDescriptionsOfPolicyRequirements(t *testing.T) {
	// Override getGPGIdFromKeyPath because we don't want to bother with (and spend the unit-test time on) generating valid GPG keys, and running the real GPG binary.
	// Instead of reading the files at all, just expect file names like /id1,id2,...,idN.pub
	idReader := func(keyPath string) []string {
		require.True(t, strings.HasPrefix(keyPath, "/"))
		require.True(t, strings.HasSuffix(keyPath, ".pub"))
		return strings.Split(keyPath[1:len(keyPath)-4], ",")
	}

	template := Policy{
		Transport: "transport",
		Name:      "name",
		RepoName:  "repoName",
	}
	registryConfigs, err := loadAndMergeConfig("./testdata")
	require.NoError(t, err)

	for _, c := range []struct {
		scope    string
		reqs     signature.PolicyRequirements
		expected []*Policy
	}{
		{
			"",
			signature.PolicyRequirements{
				signature.NewPRReject(),
			},
			[]*Policy{
				{
					Transport: "transport",
					Name:      "name",
					RepoName:  "repoName",
					Type:      "reject",
				},
			},
		},
		{
			"quay.io/accepted",
			signature.PolicyRequirements{
				signature.NewPRInsecureAcceptAnything(),
			},
			[]*Policy{
				{
					Transport: "transport",
					Name:      "name",
					RepoName:  "repoName",
					Type:      "accept",
				},
			},
		},
		{
			"registry.redhat.io",
			signature.PolicyRequirements{
				xNewPRSignedByKeyPath(t, "/redhat.pub", signature.NewPRMMatchRepoDigestOrExact()),
			},
			[]*Policy{
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "signed",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					GPGId:          "redhat",
				},
			},
		},
		{
			"registry.access.redhat.com",
			signature.PolicyRequirements{
				xNewPRSignedByKeyPaths(t, []string{"/redhat.pub", "/redhat-beta.pub"}, signature.NewPRMMatchRepoDigestOrExact()),
			},
			[]*Policy{
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "signed",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					GPGId:          "redhat, redhat-beta",
				},
			},
		},
		{
			"quay.io/multi-signed",
			signature.PolicyRequirements{
				xNewPRSignedByKeyPath(t, "/1.pub", signature.NewPRMMatchRepoDigestOrExact()),
				xNewPRSignedByKeyPath(t, "/2,3.pub", signature.NewPRMMatchRepoDigestOrExact()),
			},
			[]*Policy{
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "signed",
					SignatureStore: "https://quay.example.com/sigstore",
					GPGId:          "1",
				},
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "signed",
					SignatureStore: "https://quay.example.com/sigstore",
					GPGId:          "2, 3",
				},
			},
		},
		{
			"quay.io/sigstore-signed",
			signature.PolicyRequirements{
				xNewPRSigstoreSignedKeyPath(t, "/1.pub", signature.NewPRMMatchRepoDigestOrExact()),
				xNewPRSigstoreSignedKeyPath(t, "/2.pub", signature.NewPRMMatchRepoDigestOrExact()),
			},
			[]*Policy{
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "sigstoreSigned",
					SignatureStore: "",
					GPGId:          "N/A",
				},
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "sigstoreSigned",
					SignatureStore: "",
					GPGId:          "N/A",
				},
			},
		},
		{ // Multiple kinds of requirements are represented individually.
			"registry.redhat.io",
			signature.PolicyRequirements{
				signature.NewPRReject(),
				signature.NewPRInsecureAcceptAnything(),
				xNewPRSignedByKeyPath(t, "/redhat.pub", signature.NewPRMMatchRepoDigestOrExact()),
				xNewPRSignedByKeyPaths(t, []string{"/redhat.pub", "/redhat-beta.pub"}, signature.NewPRMMatchRepoDigestOrExact()),
				xNewPRSignedByKeyPath(t, "/1.pub", signature.NewPRMMatchRepoDigestOrExact()),
				xNewPRSignedByKeyPath(t, "/2,3.pub", signature.NewPRMMatchRepoDigestOrExact()),
				xNewPRSigstoreSignedKeyPath(t, "/1.pub", signature.NewPRMMatchRepoDigestOrExact()),
				xNewPRSigstoreSignedKeyPath(t, "/2.pub", signature.NewPRMMatchRepoDigestOrExact()),
			},
			[]*Policy{
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					Type:           "reject",
				},
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					Type:           "accept",
				},
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "signed",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					GPGId:          "redhat",
				},
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "signed",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					GPGId:          "redhat, redhat-beta",
				},
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "signed",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					GPGId:          "1",
				},
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "signed",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					GPGId:          "2, 3",
				},
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "sigstoreSigned",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					GPGId:          "N/A",
				},
				{
					Transport:      "transport",
					Name:           "name",
					RepoName:       "repoName",
					Type:           "sigstoreSigned",
					SignatureStore: "https://registry.redhat.io/containers/sigstore",
					GPGId:          "N/A",
				},
			},
		},
	} {
		reqsJSON, err := json.Marshal(c.reqs)
		require.NoError(t, err)
		var parsedRegs []repoContent
		err = json.Unmarshal(reqsJSON, &parsedRegs)
		require.NoError(t, err)

		res := descriptionsOfPolicyRequirements(parsedRegs, template, registryConfigs, c.scope, idReader)
		assert.Equal(t, c.expected, res)
	}
}
