package trust

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/signature"
)

func TestAddPolicyEntries(t *testing.T) {
	tempDir := t.TempDir()
	policyPath := filepath.Join(tempDir, "policy.json")

	minimalPolicy := &signature.Policy{
		Default: []signature.PolicyRequirement{
			signature.NewPRInsecureAcceptAnything(),
		},
	}
	minimalPolicyJSON, err := json.Marshal(minimalPolicy)
	require.NoError(t, err)
	err = os.WriteFile(policyPath, minimalPolicyJSON, 0o600)
	require.NoError(t, err)

	// Invalid input:
	for _, invalid := range []AddPolicyEntriesInput{
		{
			Scope:       "default",
			Type:        "accept",
			PubKeyFiles: []string{"/does-not-make-sense"},
		},
		{
			Scope:       "default",
			Type:        "insecureAcceptAnything",
			PubKeyFiles: []string{"/does-not-make-sense"},
		},
		{
			Scope:       "default",
			Type:        "reject",
			PubKeyFiles: []string{"/does-not-make-sense"},
		},
		{
			Scope:       "default",
			Type:        "signedBy",
			PubKeyFiles: []string{}, // A key is missing
		},
		{
			Scope:       "default",
			Type:        "sigstoreSigned",
			PubKeyFiles: []string{}, // A key is missing
		},
		{
			Scope:       "default",
			Type:        "this-is-unknown",
			PubKeyFiles: []string{},
		},
	} {
		err := AddPolicyEntries(policyPath, invalid)
		assert.Error(t, err, "%#v", invalid)
	}

	err = AddPolicyEntries(policyPath, AddPolicyEntriesInput{
		Scope: "default",
		Type:  "reject",
	})
	assert.NoError(t, err)
	err = AddPolicyEntries(policyPath, AddPolicyEntriesInput{
		Scope: "quay.io/accepted",
		Type:  "accept",
	})
	assert.NoError(t, err)
	err = AddPolicyEntries(policyPath, AddPolicyEntriesInput{
		Scope:       "quay.io/multi-signed",
		Type:        "signedBy",
		PubKeyFiles: []string{"/1.pub", "/2.pub"},
	})
	assert.NoError(t, err)
	err = AddPolicyEntries(policyPath, AddPolicyEntriesInput{
		Scope:       "quay.io/sigstore-signed",
		Type:        "sigstoreSigned",
		PubKeyFiles: []string{"/1.pub", "/2.pub"},
	})
	assert.NoError(t, err)

	// Test that the outcome is consumable, and compare it with the expected values.
	parsedPolicy, err := signature.NewPolicyFromFile(policyPath)
	require.NoError(t, err)
	assert.Equal(t, &signature.Policy{
		Default: signature.PolicyRequirements{
			signature.NewPRReject(),
		},
		Transports: map[string]signature.PolicyTransportScopes{
			"docker": {
				"quay.io/accepted": {
					signature.NewPRInsecureAcceptAnything(),
				},
				"quay.io/multi-signed": {
					xNewPRSignedByKeyPath(t, "/1.pub", signature.NewPRMMatchRepoDigestOrExact()),
					xNewPRSignedByKeyPath(t, "/2.pub", signature.NewPRMMatchRepoDigestOrExact()),
				},
				"quay.io/sigstore-signed": {
					xNewPRSigstoreSignedKeyPath(t, "/1.pub", signature.NewPRMMatchRepoDigestOrExact()),
					xNewPRSigstoreSignedKeyPath(t, "/2.pub", signature.NewPRMMatchRepoDigestOrExact()),
				},
			},
		},
	}, parsedPolicy)

	// Test that completely unknown JSON is preserved
	jsonWithUnknownData := `{
    "default": [
        {
            "type": "this is unknown",
			"unknown field": "should be preserved"
        }
    ],
    "transports":
        {
            "docker-daemon":
                {
                    "": [{
						"type":"this is unknown 2",
						"unknown field 2": "should be preserved 2"
						}]
                }
        }
}`
	err = os.WriteFile(policyPath, []byte(jsonWithUnknownData), 0o600)
	require.NoError(t, err)
	err = AddPolicyEntries(policyPath, AddPolicyEntriesInput{
		Scope:       "quay.io/innocuous",
		Type:        "signedBy",
		PubKeyFiles: []string{"/1.pub"},
	})
	require.NoError(t, err)
	updatedJSONWithUnknownData, err := os.ReadFile(policyPath)
	require.NoError(t, err)
	// Decode updatedJSONWithUnknownData so that this test does not depend on details of the encoding.
	// To reduce noise in the constants below:
	type a = []any
	type m = map[string]any
	var parsedUpdatedJSON m
	err = json.Unmarshal(updatedJSONWithUnknownData, &parsedUpdatedJSON)
	require.NoError(t, err)
	assert.Equal(t, m{
		"default": a{
			m{
				"type":          "this is unknown",
				"unknown field": "should be preserved",
			},
		},
		"transports": m{
			"docker-daemon": m{
				"": a{
					m{
						"type":            "this is unknown 2",
						"unknown field 2": "should be preserved 2",
					},
				},
			},
			"docker": m{
				"quay.io/innocuous": a{
					m{
						"type":    "signedBy",
						"keyType": "GPGKeys",
						"keyPath": "/1.pub",
					},
				},
			},
		},
	}, parsedUpdatedJSON)
}

// xNewPRSignedByKeyPath is a wrapper for NewPRSignedByKeyPath which must not fail.
func xNewPRSignedByKeyPath(t *testing.T, keyPath string, signedIdentity signature.PolicyReferenceMatch) signature.PolicyRequirement {
	pr, err := signature.NewPRSignedByKeyPath(signature.SBKeyTypeGPGKeys, keyPath, signedIdentity)
	require.NoError(t, err)
	return pr
}

// xNewPRSignedByKeyPaths is a wrapper for NewPRSignedByKeyPaths which must not fail.
func xNewPRSignedByKeyPaths(t *testing.T, keyPaths []string, signedIdentity signature.PolicyReferenceMatch) signature.PolicyRequirement {
	pr, err := signature.NewPRSignedByKeyPaths(signature.SBKeyTypeGPGKeys, keyPaths, signedIdentity)
	require.NoError(t, err)
	return pr
}

// xNewPRSigstoreSignedKeyPath is a wrapper for NewPRSigstoreSignedKeyPath which must not fail.
func xNewPRSigstoreSignedKeyPath(t *testing.T, keyPath string, signedIdentity signature.PolicyReferenceMatch) signature.PolicyRequirement {
	pr, err := signature.NewPRSigstoreSignedKeyPath(keyPath, signedIdentity)
	require.NoError(t, err)
	return pr
}
