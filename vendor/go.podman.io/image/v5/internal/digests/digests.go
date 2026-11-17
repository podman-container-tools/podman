// Package digests provides an internal representation of users’ digest use preferences.
//
// Something like this _might_ be eventually made available as a public API:
// before doing so, carefully think whether the API should be modified before we commit to it.

package digests

import (
	"errors"
	"fmt"

	"github.com/opencontainers/go-digest"
)

// Options records users’ preferences for used digest algorithm usage.
// It is a value type and can be copied using ordinary assignment.
//
// It can only be created using one of the provided constructors.
type Options struct {
	initialized bool // To prevent uses that don’t call a public constructor; this is necessary to enforce the .Available() promise.

	// If any of the fields below is set, it is guaranteed to be .Available().

	mustUse     digest.Algorithm // If not "", written digests must use this algorithm.
	prefer      digest.Algorithm // If not "", use this algorithm whenever possible.
	defaultAlgo digest.Algorithm // If not "", use this algorithm if there is no reason to use anything else.
}

// CanonicalDefault is Options which default to using digest.Canonical if there is no reason to use a different algorithm
// (e.g. when there is no pre-existing digest).
//
// The configuration can be customized using .WithPreferred() or .WithDefault().
func CanonicalDefault() Options {
	// This does not set .defaultAlgo so that .WithDefault() can be called (once).
	return Options{
		initialized: true,
	}
}

// MustUse constructs Options which always use algo.
func MustUse(algo digest.Algorithm) (Options, error) {
	// We don’t provide Options.WithMustUse because there is no other option that makes a difference
	// once .mustUse is set.
	if !algo.Available() {
		return Options{}, fmt.Errorf("attempt to use an unavailable digest algorithm %q", algo.String())
	}
	return Options{
		initialized: true,
		mustUse:     algo,
	}, nil
}

// WithPreferred returns a copy of o with a “preferred” algorithm set to algo.
// The preferred algorithm is used whenever possible (but if there is a strict requirement to use something else, it will be overridden).
func (o Options) WithPreferred(algo digest.Algorithm) (Options, error) {
	if err := o.ensureInitialized(); err != nil {
		return Options{}, err
	}
	if o.prefer != "" {
		return Options{}, errors.New("digests.Options already have a 'prefer' algorithm configured")
	}

	if !algo.Available() {
		return Options{}, fmt.Errorf("attempt to use an unavailable digest algorithm %q", algo.String())
	}
	o.prefer = algo
	return o, nil
}

// WithDefault returns a copy of o with a “default” algorithm set to algo.
// The default algorithm is used if there is no reason to use anything else (e.g. when there is no pre-existing digest).
func (o Options) WithDefault(algo digest.Algorithm) (Options, error) {
	if err := o.ensureInitialized(); err != nil {
		return Options{}, err
	}
	if o.defaultAlgo != "" {
		return Options{}, errors.New("digests.Options already have a 'default' algorithm configured")
	}

	if !algo.Available() {
		return Options{}, fmt.Errorf("attempt to use an unavailable digest algorithm %q", algo.String())
	}
	o.defaultAlgo = algo
	return o, nil
}

// ensureInitialized returns an error if o is not initialized.
func (o Options) ensureInitialized() error {
	if !o.initialized {
		return errors.New("internal error: use of uninitialized digests.Options")
	}
	return nil
}

// Situation records the context in which a digest is being chosen.
type Situation struct {
	Preexisting                 digest.Digest // If not "", a pre-existing digest value (frequently one which is cheaper to use than others)
	CannotChangeAlgorithmReason string        // The reason why we must use Preexisting, or "" if we can use other algorithms.
}

// Choose chooses a digest algorithm based on the options and the situation.
func (o Options) Choose(s Situation) (digest.Algorithm, error) {
	if err := o.ensureInitialized(); err != nil {
		return "", err
	}

	if s.CannotChangeAlgorithmReason != "" && s.Preexisting == "" {
		return "", fmt.Errorf("internal error: digests.Situation.CannotChangeAlgorithmReason is set but Preexisting is empty")
	}

	var choice digest.Algorithm // = what we want to use
	switch {
	case o.mustUse != "":
		choice = o.mustUse
	case s.CannotChangeAlgorithmReason != "":
		choice = s.Preexisting.Algorithm()
		if !choice.Available() {
			return "", fmt.Errorf("existing digest uses unimplemented algorithm %s", choice)
		}
	case o.prefer != "":
		choice = o.prefer
	case s.Preexisting != "" && s.Preexisting.Algorithm().Available():
		choice = s.Preexisting.Algorithm()
	case o.defaultAlgo != "":
		choice = o.defaultAlgo
	default:
		choice = digest.Canonical // We assume digest.Canonical is always available.
	}

	if s.CannotChangeAlgorithmReason != "" && choice != s.Preexisting.Algorithm() {
		return "", fmt.Errorf("requested to always use digest algorithm %s but we cannot replace existing digest algorithm %s: %s",
			choice, s.Preexisting.Algorithm(), s.CannotChangeAlgorithmReason)
	}

	return choice, nil
}

// MustUseSet returns an algorithm if o is set to always use a specific algorithm, "" if it is flexible.
func (o Options) MustUseSet() digest.Algorithm {
	// We don’t do .ensureInitialized() because that would require an extra error value just for that.
	// This should not be a part of any public API either way.
	if o.mustUse != "" {
		return o.mustUse
	}
	return ""
}
