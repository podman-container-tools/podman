package rawversion

// RawVersion is the raw version string.
//
// This indirection is needed to prevent semver packages from bloating
// Quadlet's binary size.
const RawVersion = "6.0.0-dev"
