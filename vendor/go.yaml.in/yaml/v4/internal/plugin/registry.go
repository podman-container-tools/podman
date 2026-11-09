// Copyright 2026 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Package plugin implements the named plugin registry.
package plugin

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Factory constructs a plugin from its plugin-specific configuration.
type Factory func(map[string]any) (any, error)

// Registration describes one named implementation of a plugin API.
// Version is empty for an unversioned implementation.
type Registration struct {
	API     string
	Name    string
	Version string
	Default bool
	Factory Factory
}

// Registry stores named plugin implementations.
type Registry struct {
	sync.RWMutex
	registrations map[string]map[string]Registration
	defaults      map[string]string
}

var versionPattern = regexp.MustCompile(`^v?([0-9]+\.[0-9]+\.[0-9]+)$`)

// NewRegistry creates a registry containing the supplied implementations.
func NewRegistry(registrations ...Registration) *Registry {
	registry := &Registry{
		registrations: map[string]map[string]Registration{},
		defaults:      map[string]string{},
	}
	for _, registration := range registrations {
		if err := registry.Register(registration); err != nil {
			panic(err)
		}
	}
	return registry
}

// Register adds a named implementation to the registry.
func (r *Registry) Register(registration Registration) error {
	if registration.API == "" || registration.Name == "" ||
		registration.Factory == nil {
		return errors.New("yaml: plugin registration requires an API, name, and factory")
	}
	if registration.Version != "" {
		version, ok := normalizeVersion(registration.Version)
		if !ok {
			return fmt.Errorf("yaml: plugin %q version must be a release version",
				registration.Name)
		}
		registration.Version = version
	}
	r.Lock()
	defer r.Unlock()
	implementations := r.registrations[registration.API]
	if implementations == nil {
		implementations = map[string]Registration{}
		r.registrations[registration.API] = implementations
	}
	if _, exists := implementations[registration.Name]; exists {
		return fmt.Errorf("yaml: plugin %q implementation %q is already registered",
			registration.API, registration.Name)
	}
	if registration.Default || registration.Name == registration.API {
		if current := r.defaults[registration.API]; current != "" {
			return fmt.Errorf(
				"yaml: plugin %q already has default implementation %q",
				registration.API, current)
		}
		r.defaults[registration.API] = registration.Name
	}
	implementations[registration.Name] = registration
	return nil
}

// CreateShortForm resolves a scalar implementation or empty configuration entry.
// Registered names take precedence over entries in the default implementation.
// Versioned selectors always identify registered implementations.
func (r *Registry) CreateShortForm(api, value string) (any, error) {
	cfg, err := ConfigValue(api, value)
	if err != nil {
		return nil, fmt.Errorf("yaml: plugin %q: %w", api, err)
	}
	name := cfg["name"].(string)
	r.RLock()
	_, registered := r.registrations[api][name]
	hasDefault := r.defaults[api] != ""
	r.RUnlock()
	if !registered && hasDefault && cfg["version"] == nil {
		entry := map[string]any{name: map[string]any{}}
		if plugin, err := r.Create(api, entry); err == nil {
			return plugin, nil
		}
	}
	return r.Create(api, cfg)
}

// Create resolves and constructs a configured implementation.
func (r *Registry) Create(api string, cfg map[string]any) (any, error) {
	r.RLock()
	name := r.defaults[api]
	r.RUnlock()
	if raw, found := cfg["name"]; found {
		var ok bool
		name, ok = raw.(string)
		if !ok || name == "" {
			return nil, fmt.Errorf(
				"yaml: plugin %q name must be a non-empty string", api)
		}
	}
	if name == "" {
		return nil, fmt.Errorf(
			"yaml: plugin %q has no default implementation", api)
	}
	requestedVersion := ""
	if raw, found := cfg["version"]; found {
		version, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf(
				"yaml: plugin %q version must be a release version", api)
		}
		requestedVersion, ok = normalizeVersion(version)
		if !ok {
			return nil, fmt.Errorf(
				"yaml: plugin %q version must be a release version", api)
		}
	}

	r.RLock()
	implementations := r.registrations[api]
	if implementations == nil {
		r.RUnlock()
		return nil, fmt.Errorf(
			"yaml: unknown plugin API %q; register its implementation first", api)
	}
	registration, found := implementations[name]
	if !found {
		var names []string
		for implementation := range implementations {
			names = append(names, implementation)
		}
		r.RUnlock()
		sort.Strings(names)
		return nil, fmt.Errorf(
			"yaml: unknown implementation %q for plugin %q; available: %s",
			name, api, strings.Join(names, ", "))
	}
	r.RUnlock()
	if requestedVersion != "" && registration.Version != requestedVersion {
		if registration.Version == "" {
			return nil, fmt.Errorf(
				"yaml: plugin %q implementation %q is unversioned; requested %s",
				api, name, requestedVersion)
		}
		return nil, fmt.Errorf(
			"yaml: plugin %q implementation %q version mismatch: linked %s, requested %s",
			api, name, registration.Version, requestedVersion)
	}
	pluginConfig := make(map[string]any, len(cfg))
	for key, value := range cfg {
		if key != "name" && key != "version" {
			pluginConfig[key] = value
		}
	}
	return registration.Factory(pluginConfig)
}

func normalizeVersion(version string) (string, bool) {
	match := versionPattern.FindStringSubmatch(version)
	if match == nil {
		return "", false
	}
	return match[1], true
}
