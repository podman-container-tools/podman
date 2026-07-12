package seccomp

import (
	"fmt"
	"sort"
)

// ContainerImageLabel is the key of the image annotation embedding a seccomp
// profile.
const ContainerImageLabel = "io.containers.seccomp.profile"

// Policy denotes a seccomp policy.
type Policy int

const (
	// PolicyDefault - if set use SecurityConfig.SeccompProfilePath,
	// otherwise use the default profile.  The SeccompProfilePath might be
	// explicitly set by the user.
	PolicyDefault Policy = iota
	// PolicyImage - if set use SecurityConfig.SeccompProfileFromImage,
	// otherwise follow SeccompPolicyDefault.
	PolicyImage
)

// Map for easy lookups of supported policies.
var supportedPolicies = map[string]Policy{
	"":        PolicyDefault,
	"default": PolicyDefault,
	"image":   PolicyImage,
}

// LookupPolicy looks up the corresponding Policy for the specified
// string. If none is found, an errors is returned including the list of
// supported policies.
//
// Note that an empty string resolved to SeccompPolicyDefault.
func LookupPolicy(s string) (Policy, error) {
	policy, exists := supportedPolicies[s]
	if exists {
		return policy, nil
	}

	// Sort the keys first as maps are non-deterministic.
	keys := []string{}
	for k := range supportedPolicies {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	return -1, fmt.Errorf("invalid seccomp policy %q: valid policies are %+q", s, keys)
}
