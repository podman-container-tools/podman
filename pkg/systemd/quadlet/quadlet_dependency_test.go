package quadlet

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/systemd/parser"
)

func TestTranslateUnitDependenciesWithTemplateNotInMap(t *testing.T) {
	// Regression test for https://github.com/containers/podman/issues/29819
	// Test that template dependencies not in unitsInfoMap (e.g., rootful templates
	// referenced by rootless units) are translated correctly by computing the
	// service name from the filename.

	// Create a minimal unit file with a dependency on a template unit
	unitFile := parser.NewUnitFile()
	unitFile.Filename = "test2@instance1.container"
	unitFile.Add("Unit", "Conflicts", "test1@.container")

	// Create an empty unitsInfoMap (simulating the case where the template
	// unit is not found in rootless directories)
	unitsInfoMap := make(map[string]*UnitInfo)

	// Translate dependencies - should not fail for template units
	err := translateUnitDependencies(unitFile, unitsInfoMap)
	require.NoError(t, err)

	// Verify the dependency was translated correctly
	deps := unitFile.LookupAllStrv("Unit", "Conflicts")
	require.Len(t, deps, 1)
	require.Equal(t, "test1@.service", deps[0])
}
