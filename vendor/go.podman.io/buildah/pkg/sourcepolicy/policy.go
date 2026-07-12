// Package sourcepolicy implements BuildKit-compatible source policy evaluation
// for controlling and transforming source references during builds.
//
// Source policies allow users to:
//   - Pin base image tags to specific digests at build time
//   - Deny specific sources from being used
//   - Transform source references without modifying Containerfiles or Dockerfiles
//
// The policy file format is compatible with BuildKit's source policy JSON schema.
package sourcepolicy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"go.podman.io/image/v5/docker/reference"
)

// Action represents the action to take when a rule matches.
type Action string

const (
	// ActionAllow explicitly allows a source (no transformation).
	ActionAllow Action = "ALLOW"
	// ActionDeny blocks the source and fails the build.
	ActionDeny Action = "DENY"
	// ActionConvert transforms the source to a different reference.
	ActionConvert Action = "CONVERT"
)

// MatchType represents how the selector identifier should be matched.
type MatchType string

const (
	// MatchTypeExact requires an exact string match.
	MatchTypeExact MatchType = "EXACT"
	// MatchTypeWildcard allows * and ? glob patterns.
	MatchTypeWildcard MatchType = "WILDCARD"
	// MatchTypeRegex allows regular expression patterns (not implemented in MVP).
	MatchTypeRegex MatchType = "REGEX"
)

// Selector specifies which sources a rule applies to.
type Selector struct {
	// Identifier is the source identifier to match.
	// For docker images, this is typically "docker-image://registry/repo:tag".
	Identifier string `json:"identifier"`

	// MatchType specifies how the identifier should be matched.
	// Defaults to EXACT if not specified.
	MatchType MatchType `json:"matchType,omitempty"`
}

// Updates specifies how to transform a matched source.
type Updates struct {
	// Identifier is the new source identifier to use.
	// For CONVERT actions, this replaces the original identifier.
	Identifier string `json:"identifier,omitempty"`

	// Attrs contains additional attributes (e.g., http.checksum).
	// Reserved for future use with HTTP sources.
	Attrs map[string]string `json:"attrs,omitempty"`
}

// Rule represents a single policy rule.
type Rule struct {
	// Action specifies what to do when this rule matches.
	Action Action `json:"action"`

	// Selector specifies which sources this rule applies to.
	Selector Selector `json:"selector"`

	// Updates specifies how to transform the source (for CONVERT action).
	Updates *Updates `json:"updates,omitempty"`
}

// Policy represents a source policy containing multiple rules.
type Policy struct {
	// Rules is the list of policy rules, evaluated in order.
	// First matching rule wins.
	Rules []Rule `json:"rules"`
}

// Decision represents the result of evaluating a source against a policy.
type Decision struct {
	// Action is the action to take (ALLOW, DENY, or CONVERT).
	Action Action

	// TargetRef is the new reference to use (for CONVERT actions).
	TargetRef string

	// Reason provides context for the decision (e.g., which rule matched).
	Reason string
}

// LoadFromFile loads a source policy from a JSON file.
func LoadFromFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading source policy file %q: %w", path, err)
	}

	return Parse(data)
}

// Parse parses a source policy from JSON data.
func Parse(data []byte) (*Policy, error) {
	var policy Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("parsing source policy JSON: %w", err)
	}

	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("validating source policy: %w", err)
	}

	return &policy, nil
}

// Validate checks that the policy is well-formed.
func (p *Policy) Validate() error {
	if len(p.Rules) == 0 {
		// Empty policy is valid - it just means no rules apply
		return nil
	}

	for i, rule := range p.Rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("rule %d: %w", i, err)
		}
	}

	return nil
}

// Validate checks that a rule is well-formed.
func (r *Rule) Validate() error {
	// Validate action
	switch r.Action {
	case ActionAllow, ActionDeny, ActionConvert:
		// Valid actions
	case "":
		return fmt.Errorf("action is required")
	default:
		return fmt.Errorf("unknown action %q (valid: ALLOW, DENY, CONVERT)", r.Action)
	}

	// Validate selector
	if r.Selector.Identifier == "" {
		return fmt.Errorf("selector.identifier is required")
	}

	// Validate match type
	switch r.Selector.MatchType {
	case MatchTypeExact, MatchTypeWildcard, "":
		// Valid match types (empty defaults to EXACT)
	case MatchTypeRegex:
		return fmt.Errorf("REGEX match type is not supported in this version")
	default:
		return fmt.Errorf("unknown matchType %q (valid: EXACT, WILDCARD)", r.Selector.MatchType)
	}

	// Validate updates for CONVERT action
	if r.Action == ActionConvert {
		if r.Updates == nil || r.Updates.Identifier == "" {
			return fmt.Errorf("updates.identifier is required for CONVERT action")
		}
	}

	return nil
}

// Evaluate checks a source identifier against the policy and returns a decision.
// The first matching rule wins. If no rule matches, returns (Decision{}, false, nil).
func (p *Policy) Evaluate(sourceIdentifier string) (Decision, bool, error) {
	if p == nil || len(p.Rules) == 0 {
		return Decision{}, false, nil
	}

	for i, rule := range p.Rules {
		matched, err := rule.Matches(sourceIdentifier)
		if err != nil {
			return Decision{}, false, fmt.Errorf("evaluating rule %d: %w", i, err)
		}

		if matched {
			decision := Decision{
				Action: rule.Action,
				Reason: fmt.Sprintf("matched rule %d (selector: %q)", i, rule.Selector.Identifier),
			}

			if rule.Action == ActionConvert && rule.Updates != nil {
				decision.TargetRef = rule.Updates.Identifier
			}

			return decision, true, nil
		}
	}

	return Decision{}, false, nil
}

// Matches checks if a source identifier matches this rule's selector.
func (r *Rule) Matches(sourceIdentifier string) (bool, error) {
	matchType := r.Selector.MatchType
	if matchType == "" {
		matchType = MatchTypeWildcard
	}

	switch matchType {
	case MatchTypeExact:
		return r.Selector.Identifier == sourceIdentifier, nil
	case MatchTypeWildcard:
		return matchWildcard(r.Selector.Identifier, sourceIdentifier), nil
	default:
		return false, fmt.Errorf("unsupported match type: %s", matchType)
	}
}

// matchWildcard performs glob-style pattern matching.
// Supports * (matches any sequence of characters) and ? (matches any single character).
func matchWildcard(pattern, str string) bool {
	// Use a simple recursive approach for wildcard matching
	return wildcardMatch(pattern, str)
}

// wildcardMatch implements recursive wildcard matching.
func wildcardMatch(pattern, str string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			// * matches zero or more characters
			// Try matching the rest of the pattern against all possible suffixes
			pattern = pattern[1:]
			if len(pattern) == 0 {
				// Trailing * matches everything
				return true
			}
			// Try matching at each position
			for i := 0; i <= len(str); i++ {
				if wildcardMatch(pattern, str[i:]) {
					return true
				}
			}
			return false
		case '?':
			// ? matches exactly one character
			if len(str) == 0 {
				return false
			}
			pattern = pattern[1:]
			str = str[1:]
		default:
			// Regular character must match exactly
			if len(str) == 0 || pattern[0] != str[0] {
				return false
			}
			pattern = pattern[1:]
			str = str[1:]
		}
	}
	return len(str) == 0
}

// ImageSourceIdentifier creates a BuildKit-style source identifier for a docker image.
// This normalizes image references to the format "docker-image://registry/repo:tag".
func ImageSourceIdentifier(imageRef string) string {
	// If already in docker-image:// format, return as-is
	if strings.HasPrefix(imageRef, "docker-image://") {
		return imageRef
	}

	// Normalize the image reference
	normalized := normalizeImageRef(imageRef)
	return "docker-image://" + normalized
}

// normalizeImageRef normalizes an image reference to include registry and library prefix.
func normalizeImageRef(ref string) string {
	// Handle scratch specially
	if ref == "scratch" {
		return ref
	}

	// Use go.podman.io/image/v5/docker/reference for proper normalization
	named, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		// If parsing fails, return the original reference
		return ref
	}

	return named.String()
}

// ExtractImageRef extracts the image reference from a BuildKit-style source identifier.
// It returns the original identifier if it's not a docker-image:// reference.
func ExtractImageRef(sourceIdentifier string) string {
	const prefix = "docker-image://"
	if strings.HasPrefix(sourceIdentifier, prefix) {
		return sourceIdentifier[len(prefix):]
	}
	return sourceIdentifier
}
