package protectedroot

import (
	"encoding/json"
	"errors"
	"fmt"

	securejoin "github.com/cyphar/filepath-securejoin"
)

// PathRoot wraps a directory representing a root to confine paths against,
// so that paths which would otherwise escape the root are interpreted like
// in chroot environments or with openat2 calls using the RESOLVE_IN_ROOT flag.
// The path field is unexported to avoid trivial protection bypasses. The
// zero value with an empty string inside is invalid and methods will panic if
// used on it. Constructors should be used to create valid instances of the type.
type PathRoot struct {
	path string
}

// NewPathRoot constructs a PathRoot from a given path. The path is carried
// as given, without being resolved or verified in any way, and the
// confinement PathRoot provides is relative to wherever it currently
// resolves to, including through symlinks: if the path is, or later becomes,
// a symlink to a different location, or the directory it names is replaced,
// confinement follows it there. It returns an error if the path is empty.
func NewPathRoot(path string) (PathRoot, error) {
	if path == "" {
		return PathRoot{}, errors.New("root path cannot be empty")
	}

	return PathRoot{path: path}, nil
}

// NewOptionalPathRoot constructs a PathRoot from a given path, with the same
// confinement guarantee as NewPathRoot. When the path argument is an empty
// string, it returns nil to represent the absence of a root.
func NewOptionalPathRoot(path string) (*PathRoot, error) {
	if path == "" {
		return nil, nil
	}

	pathRoot, err := NewPathRoot(path)
	if err != nil {
		return nil, err
	}

	return &pathRoot, nil
}

// ensureInitialized acts as a guard to ensure that once PathRoot
// is constructed, it must carry a path.
func (pr PathRoot) ensureInitialized() {
	if pr.path == "" {
		panic("internal error, use of an uninitialized PathRoot")
	}
}

// Join resolves pathToJoin against the root via
// securejoin.SecureJoin, to make sure that the resulting
// path does not escape the root.
//
// This method is not TOCTOU-safe, the returned path may not
// stay confined. When feasible, prefer a TOCTOU-safe alternative
// method instead.
func (pr PathRoot) Join(pathToJoin string) (string, error) {
	pr.ensureInitialized()

	return securejoin.SecureJoin(pr.path, pathToJoin)
}

// PathWithoutProtection returns the underlying path as a plain string.
// It is generally discouraged to use this method and the caller
// should attempt to use a different method when possible instead.
func (pr PathRoot) PathWithoutProtection() string {
	pr.ensureInitialized()

	return pr.path
}

// Format implements fmt.Formatter (deliberately not fmt.Stringer,
// which would be a less visible equivalent of PathWithoutProtection)
// so PathRoot can be used in format strings, e.g. for logging.
func (pr PathRoot) Format(f fmt.State, verb rune) {
	fmt.Fprintf(f, fmt.FormatString(f, verb), pr.path)
}

func (pr PathRoot) MarshalJSON() ([]byte, error) {
	return json.Marshal(pr.path)
}

func (pr *PathRoot) UnmarshalJSON(data []byte) error {
	var path string
	if err := json.Unmarshal(data, &path); err != nil {
		return err
	}

	newPathRoot, err := NewPathRoot(path)
	if err != nil {
		return err
	}

	*pr = newPathRoot
	return nil
}

// Utilities for incremental migration: calls suffixed with
// `Deferred` are used where migrating these call sites
// is considered out of scope for the migration they are part of.
// Other than that, they behave identically to their counterparts.

func NewPathRootDeferred(path string) (PathRoot, error) {
	return NewPathRoot(path)
}

func NewOptionalPathRootDeferred(path string) (*PathRoot, error) {
	return NewOptionalPathRoot(path)
}

func (pr PathRoot) PathWithoutProtectionDeferred() string {
	return pr.PathWithoutProtection()
}
