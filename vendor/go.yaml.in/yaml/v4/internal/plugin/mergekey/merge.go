// Copyright 2026 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Package mergekey implements the default Merge-Key algorithm.
// It is independent of node representation and value construction.
package mergekey

import (
	"errors"
	"fmt"
)

// Config controls recognition, repeated directives, and merge conflicts.
// Empty Multiple inherits the loader's duplicate-key setting.
// Empty Conflicts means first.
type Config struct {
	Disable   bool
	Multiple  string
	Conflicts string
}

// Validate checks a configuration without changing it.
func (c Config) Validate() error {
	if c.Multiple != "" && c.Multiple != "error" && c.Multiple != "combine" {
		return errors.New("merge-key: multiple must be error or combine")
	}
	if c.Conflicts != "" && c.Conflicts != "first" &&
		c.Conflicts != "last" && c.Conflicts != "error" {
		return errors.New("merge-key: conflicts must be first, last, or error")
	}
	return nil
}

// Entry associates a constructed key with a host-owned mapping entry.
type Entry[T any] struct {
	Key   any
	Value T
}

// ConflictError identifies the later entry in an overlap between mappings.
type ConflictError struct {
	Key   any
	Group int
	Index int
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("merge-key: conflicting key %#v", e.Key)
}

// Merge selects entries from the target followed by the ordered sources.
// Ordinary duplicates within one mapping retain their original order.
func Merge[T any](groups [][]Entry[T], conflicts string) ([]T, error) {
	owners := make(map[any]int)
	for group, entries := range groups {
		for index, entry := range entries {
			owner, found := owners[entry.Key]
			if !found || owner == group {
				owners[entry.Key] = group
				continue
			}
			switch conflicts {
			case "last":
				owners[entry.Key] = group
			case "error":
				return nil, &ConflictError{entry.Key, group, index}
			}
		}
	}
	var result []T
	for group, entries := range groups {
		for _, entry := range entries {
			owner, found := owners[entry.Key]
			// Non-reflexive keys such as NaN cannot conflict with any key.
			if !found || owner == group {
				result = append(result, entry.Value)
			}
		}
	}
	return result, nil
}
