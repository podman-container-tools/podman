package types

import (
	"fmt"
	"os"

	"go.podman.io/storage/pkg/idtools"
)

// AutoUserNsOptions defines how to automatically create a user namespace.
type AutoUserNsOptions struct {
	// Size defines the size for the user namespace.  If it is set to a
	// value bigger than 0, the user namespace will have exactly this size.
	// If it is not set, some heuristics will be used to find its size.
	Size uint32
	// InitialSize defines the minimum size for the user namespace.
	// The created user namespace will have at least this size.
	InitialSize uint32
	// PasswdFile to use if the container uses a volume.
	PasswdFile string
	// GroupFile to use if the container uses a volume.
	GroupFile string
	// AdditionalUIDMappings specified additional UID mappings to include in
	// the generated user namespace.
	AdditionalUIDMappings []idtools.IDMap
	// AdditionalGIDMappings specified additional GID mappings to include in
	// the generated user namespace.
	AdditionalGIDMappings []idtools.IDMap
}

// IDMappingOptions specifies the caller's desired UID/GID mapping for a
// layer or container.
//
// These options express what the caller wants, the mapping that the
// container's user namespace should use.  They do not describe what is
// stored on disk.  Depending on the graph driver, the store may apply
// the mapping at layer creation time (by chowning files) or defer it to
// mount time (using idmapped mounts or fuse-overlayfs options), but
// that distinction is transparent to the caller.
//
// The resolution order for the effective UID/GID maps is:
//  1. If HostUIDMapping/HostGIDMapping is true, no mapping is used (the
//     corresponding UIDMap/GIDMap is ignored and treated as empty).
//  2. If UIDMap/GIDMap contain at least one entry, those mappings are used.
//  3. Otherwise, if the layer has a parent, the parent's mappings are inherited.
//  4. Otherwise, the Store-level default mappings are used.
type IDMappingOptions struct {
	// HostUIDMapping indicates that no UID mapping should be applied.
	// When true, UIDMap is ignored and files are accessed with host UIDs.
	HostUIDMapping bool
	// HostGIDMapping indicates that no GID mapping should be applied.
	// When true, GIDMap is ignored and files are accessed with host GIDs.
	HostGIDMapping bool
	// UIDMap defines the UID mappings for the user namespace.
	// Only used when HostUIDMapping is false.
	UIDMap []idtools.IDMap
	// GIDMap defines the GID mappings for the user namespace.
	// Only used when HostGIDMapping is false.
	GIDMap         []idtools.IDMap
	AutoUserNs     bool
	AutoUserNsOpts AutoUserNsOptions
}

// ParseIDMapping takes idmappings and subuid and subgid maps and returns a storage mapping
func ParseIDMapping(UIDMapSlice, GIDMapSlice []string, subUIDMap, subGIDMap string) (*IDMappingOptions, error) {
	options := IDMappingOptions{
		HostUIDMapping: true,
		HostGIDMapping: true,
	}
	if subGIDMap == "" && subUIDMap != "" {
		subGIDMap = subUIDMap
	}
	if subUIDMap == "" && subGIDMap != "" {
		subUIDMap = subGIDMap
	}
	if len(GIDMapSlice) == 0 && len(UIDMapSlice) != 0 {
		GIDMapSlice = UIDMapSlice
	}
	if len(UIDMapSlice) == 0 && len(GIDMapSlice) != 0 {
		UIDMapSlice = GIDMapSlice
	}
	if len(UIDMapSlice) == 0 && subUIDMap == "" && os.Getuid() != 0 {
		UIDMapSlice = []string{fmt.Sprintf("0:%d:1", os.Getuid())}
	}
	if len(GIDMapSlice) == 0 && subGIDMap == "" && os.Getuid() != 0 {
		GIDMapSlice = []string{fmt.Sprintf("0:%d:1", os.Getgid())}
	}

	if subUIDMap != "" && subGIDMap != "" {
		mappings, err := idtools.NewIDMappings(subUIDMap, subGIDMap)
		if err != nil {
			return nil, fmt.Errorf("failed to create NewIDMappings for uidmap=%s gidmap=%s: %w", subUIDMap, subGIDMap, err)
		}
		options.UIDMap = mappings.UIDs()
		options.GIDMap = mappings.GIDs()
	}
	parsedUIDMap, err := idtools.ParseIDMap(UIDMapSlice, "UID")
	if err != nil {
		return nil, fmt.Errorf("failed to create ParseUIDMap UID=%s: %w", UIDMapSlice, err)
	}
	parsedGIDMap, err := idtools.ParseIDMap(GIDMapSlice, "GID")
	if err != nil {
		return nil, fmt.Errorf("failed to create ParseGIDMap GID=%s: %w", UIDMapSlice, err)
	}
	options.UIDMap = append(options.UIDMap, parsedUIDMap...)
	options.GIDMap = append(options.GIDMap, parsedGIDMap...)
	if len(options.UIDMap) > 0 {
		options.HostUIDMapping = false
	}
	if len(options.GIDMap) > 0 {
		options.HostGIDMapping = false
	}
	return &options, nil
}
