package quadlet

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/systemd/parser"
)

func TestTranslateUnitDependenciesWithTemplateNotInMap(t *testing.T) {
	// Regression test for https://github.com/containers/podman/issues/29819
	// Test that template dependencies not in unitsInfoMap (e.g., rootful templates
	// referenced by rootless units) are translated correctly by generating the
	// instance name from the template and checking if that instance exists.

	// Create a template unit file with a dependency on another template unit
	unitFile := parser.NewUnitFile()
	unitFile.Filename = "test2@instance1.container"
	unitFile.Add("Unit", "Conflicts", "test1@.container")

	// Create unitsInfoMap with only the instantiated dependency
	// (simulating the case where the template is rootful and not in the map,
	// but the instance is symlinked in the rootless directory as per issue #29819)
	unitsInfoMap := make(map[string]*UnitInfo)
	unitsInfoMap["test1@instance1.container"] = &UnitInfo{
		ServiceName:  "test1@instance1",
		ResourceName: "test1-instance1",
	}

	// Translate dependencies - should succeed with instance name
	err := translateUnitDependencies(unitFile, unitsInfoMap)
	require.NoError(t, err)

	// Verify the dependency was translated to the instance service name
	deps := unitFile.LookupAllStrv("Unit", "Conflicts")
	require.Len(t, deps, 1)
	require.Equal(t, "test1@instance1.service", deps[0])
}

func TestTranslateUnitDependenciesWithTemplateTypo(t *testing.T) {
	// Test that typos in template dependencies are caught and rejected.
	// When the generated instance doesn't exist in unitsInfoMap, the function
	// should fail instead of silently accepting the typo.

	// Create a template unit file with a typo in the dependency
	unitFile := parser.NewUnitFile()
	unitFile.Filename = "test2@instance1.container"
	unitFile.Add("Unit", "Conflicts", "testt1@.container") // typo: testt1 instead of test1

	// Create unitsInfoMap without the typoed dependency
	unitsInfoMap := make(map[string]*UnitInfo)
	unitsInfoMap["test1@instance1.container"] = &UnitInfo{
		ServiceName:  "test1@instance1",
		ResourceName: "test1-instance1",
	}

	// Translate dependencies - should fail due to typo
	err := translateUnitDependencies(unitFile, unitsInfoMap)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to translate dependency")
	require.Contains(t, err.Error(), "not found")
}

func TestTranslateUnitDependenciesWithNonTemplateTypo(t *testing.T) {
	// Test that typos in non-template dependencies are caught and rejected.
	// This ensures existing strict behavior is preserved for non-template units.

	// Create a unit file with a typo in a non-template dependency
	unitFile := parser.NewUnitFile()
	unitFile.Filename = "test2.container"
	unitFile.Add("Unit", "Conflicts", "testt1.container") // typo

	// Create unitsInfoMap without the typoed dependency
	unitsInfoMap := make(map[string]*UnitInfo)
	unitsInfoMap["test1.container"] = &UnitInfo{
		ServiceName:  "test1",
		ResourceName: "test1",
	}

	// Translate dependencies - should fail due to typo
	err := translateUnitDependencies(unitFile, unitsInfoMap)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to translate dependency")
}

func TestTranslateUnitDependenciesWithExistingNonTemplate(t *testing.T) {
	// Test that valid non-template dependencies continue to work correctly.
	// This ensures existing behavior is preserved.

	// Create a unit file with a valid non-template dependency
	unitFile := parser.NewUnitFile()
	unitFile.Filename = "test2.container"
	unitFile.Add("Unit", "Conflicts", "test1.container")

	// Create unitsInfoMap with the dependency
	unitsInfoMap := make(map[string]*UnitInfo)
	unitsInfoMap["test1.container"] = &UnitInfo{
		ServiceName:  "test1",
		ResourceName: "test1",
	}

	// Translate dependencies - should succeed
	err := translateUnitDependencies(unitFile, unitsInfoMap)
	require.NoError(t, err)

	// Verify the dependency was translated correctly
	deps := unitFile.LookupAllStrv("Unit", "Conflicts")
	require.Len(t, deps, 1)
	require.Equal(t, "test1.service", deps[0])
}

func TestTranslateUnitDependenciesWithNonTemplateUnitReferringToTemplate(t *testing.T) {
	// Test that a non-template unit referring to a template works correctly
	// when the template is in unitsInfoMap. In this case, there's no instance
	// to propagate, so the template itself should be used.

	// Create a non-template unit file with a template dependency
	unitFile := parser.NewUnitFile()
	unitFile.Filename = "test2.container"
	unitFile.Add("Unit", "Conflicts", "test1@.container")

	// Create unitsInfoMap with the template dependency
	unitsInfoMap := make(map[string]*UnitInfo)
	unitsInfoMap["test1@.container"] = &UnitInfo{
		ServiceName:  "test1@",
		ResourceName: "test1-%i",
	}

	// Translate dependencies - should succeed
	err := translateUnitDependencies(unitFile, unitsInfoMap)
	require.NoError(t, err)

	// Verify the dependency was translated to the template service name
	deps := unitFile.LookupAllStrv("Unit", "Conflicts")
	require.Len(t, deps, 1)
	require.Equal(t, "test1@.service", deps[0])
}

func TestTranslateUnitDependenciesWithNonTemplateUnitReferringToTemplateNotInMap(t *testing.T) {
	// Test that a non-template unit referring to a template that's not in unitsInfoMap
	// should fail, since there's no instance to propagate and the template doesn't exist.

	// Create a non-template unit file with a template dependency
	unitFile := parser.NewUnitFile()
	unitFile.Filename = "test2.container"
	unitFile.Add("Unit", "Conflicts", "test1@.container")

	// Create unitsInfoMap without the template dependency
	unitsInfoMap := make(map[string]*UnitInfo)

	// Translate dependencies - should fail
	err := translateUnitDependencies(unitFile, unitsInfoMap)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to translate dependency")
}

func TestTranslateUnitDependenciesWithExplicitInstantiatedDependency(t *testing.T) {
	// Test that when a dependency is explicitly instantiated (e.g., test1@instance2.container),
	// it should be looked up directly without instance propagation, since the user specified
	// a specific instance.

	// Create a template unit file with a dependency on a specific instance
	unitFile := parser.NewUnitFile()
	unitFile.Filename = "test2@instance1.container"
	unitFile.Add("Unit", "Conflicts", "test1@instance2.container")

	// Create unitsInfoMap with the specific instantiated dependency
	unitsInfoMap := make(map[string]*UnitInfo)
	unitsInfoMap["test1@instance2.container"] = &UnitInfo{
		ServiceName:  "test1@instance2",
		ResourceName: "test1-instance2",
	}

	// Translate dependencies - should succeed with instance2 (no propagation)
	err := translateUnitDependencies(unitFile, unitsInfoMap)
	require.NoError(t, err)

	// Verify the dependency was translated to the instance2 service name
	deps := unitFile.LookupAllStrv("Unit", "Conflicts")
	require.Len(t, deps, 1)
	require.Equal(t, "test1@instance2.service", deps[0])
}

func TestExtractInstanceName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "template with instance",
			filename: "test@instance1.container",
			expected: "instance1",
		},
		{
			name:     "template with multi-char instance",
			filename: "test@my-instance.container",
			expected: "my-instance",
		},
		{
			name:     "uninstantiated template",
			filename: "test@.container",
			expected: "",
		},
		{
			name:     "non-template",
			filename: "test.container",
			expected: "",
		},
		{
			name:     "template with empty instance",
			filename: "test@.container",
			expected: "",
		},
		{
			name:     "instance with @ character (systemd allows this)",
			filename: "test@instance@1.container",
			expected: "instance@1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInstanceName(tt.filename)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateInstanceName(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		instanceName string
		expected     string
	}{
		{
			name:         "template with instance",
			templateName: "test@.container",
			instanceName: "instance1",
			expected:     "test@instance1.container",
		},
		{
			name:         "template without instance",
			templateName: "test@.container",
			instanceName: "",
			expected:     "test@.container",
		},
		{
			name:         "template with multi-char instance",
			templateName: "test@.container",
			instanceName: "my-instance",
			expected:     "test@my-instance.container",
		},
		{
			name:         "template without @ suffix",
			templateName: "test.container",
			instanceName: "instance1",
			expected:     "test@instance1.container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateInstanceName(tt.templateName, tt.instanceName)
			require.Equal(t, tt.expected, result)
		})
	}
}
