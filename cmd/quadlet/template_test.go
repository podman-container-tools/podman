package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/systemd/parser"
	"go.podman.io/podman/v6/pkg/systemd/quadlet"
)

func writeTemplateTestFile(t *testing.T, path, contents string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
}

func loadTemplateTestUnits(t *testing.T, dirs []string) []*parser.UnitFile {
	t.Helper()
	oldSeen := seen
	seen = make(map[string]struct{})
	t.Cleanup(func() { seen = oldSeen })
	var units []*parser.UnitFile
	for _, dir := range dirs {
		loaded, err := loadUnitsFromDir(dir)
		require.NoError(t, err)
		units = append(units, loaded...)
	}
	instances, err := loadTemplateInstances(dirs, units)
	require.NoError(t, err)
	return append(units, instances...)
}

func TestTemplateInstanceDropins(t *testing.T) {
	high, low := t.TempDir(), t.TempDir()
	writeTemplateTestFile(t, filepath.Join(high, "app@.container"), "[Container]\nImage=localhost/high\nVolume=./data:/data\n")
	writeTemplateTestFile(t, filepath.Join(low, "app@.container"), "[Container]\nImage=localhost/low\n")
	writeTemplateTestFile(t, filepath.Join(low, "app@.container.d/10-image.conf"), "[Container]\nImage=localhost/template\n")
	writeTemplateTestFile(t, filepath.Join(low, "app@.container.d/20-env.conf"), "[Container]\nEnvironment=SHARED=yes\n")
	writeTemplateTestFile(t, filepath.Join(high, "app@blue.container.d/10-image.conf"), "[Container]\nImage=localhost/blue\nEnvironment=BLUE=yes\n")
	writeTemplateTestFile(t, filepath.Join(low, "app@blue.container.d/10-image.conf"), "[Container]\nImage=localhost/wrong\n")
	writeTemplateTestFile(t, filepath.Join(low, "app@red.container.d/30-env.conf"), "[Container]\nEnvironment=RED=yes\n")
	dirs := []string{high, low}
	units := loadTemplateTestUnits(t, dirs)
	require.Len(t, units, 3)
	for _, unit := range units {
		require.NoError(t, loadUnitDropins(unit, dirs))
	}
	info := generateUnitsInfoMap(units)
	for _, unit := range units {
		t.Run(unit.Filename, func(t *testing.T) {
			assert.Equal(t, filepath.Join(high, "app@.container"), unit.Path)
			env := unit.LookupAll("Container", "Environment")
			image, _ := unit.LookupLast("Container", "Image")
			switch unit.Filename {
			case "app@.container":
				assert.Equal(t, "localhost/template", image)
				assert.Equal(t, []string{"SHARED=yes"}, env)
			case "app@blue.container":
				assert.Equal(t, "localhost/blue", image)
				assert.ElementsMatch(t, []string{"BLUE=yes", "SHARED=yes"}, env)
			case "app@red.container":
				assert.Equal(t, "localhost/template", image)
				assert.ElementsMatch(t, []string{"RED=yes", "SHARED=yes"}, env)
			default:
				t.Fatalf("unexpected unit %s", unit.Filename)
			}
			service, _, err := quadlet.ConvertContainer(unit, info, true)
			require.NoError(t, err)
			exec, _ := service.LookupLast("Service", "ExecStart")
			assert.Contains(t, exec, image)
			assert.Contains(t, exec, filepath.Join(high, "data")+":/data")
			for _, value := range env {
				assert.Contains(t, exec, "--env "+value)
			}
		})
	}
}

func TestTemplateInstanceExplicitUnit(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		name := "file"
		if symlink {
			name = "symlink"
		}
		t.Run(name, func(t *testing.T) {
			high, low := t.TempDir(), t.TempDir()
			writeTemplateTestFile(t, filepath.Join(high, "app@.container"), "[Container]\nImage=localhost/template\n")
			writeTemplateTestFile(t, filepath.Join(high, "app@blue.container.d/override.conf"), "[Container]\nEnvironment=BLUE=yes\n")
			instancePath := filepath.Join(low, "app@blue.container")
			if symlink {
				writeTemplateTestFile(t, filepath.Join(low, "app@.container"), "[Container]\nImage=localhost/explicit\n")
				require.NoError(t, os.Symlink("app@.container", instancePath))
			} else {
				writeTemplateTestFile(t, instancePath, "[Container]\nImage=localhost/explicit\n")
			}
			dirs := []string{high, low}
			units := loadTemplateTestUnits(t, dirs)
			require.Len(t, units, 2)
			instance := units[1]
			assert.Equal(t, "app@blue.container", instance.Filename)
			assert.Equal(t, instancePath, instance.Path)
			require.NoError(t, loadUnitDropins(instance, dirs))
			image, _ := instance.LookupLast("Container", "Image")
			assert.Equal(t, "localhost/explicit", image)
			assert.Equal(t, []string{"BLUE=yes"}, instance.LookupAll("Container", "Environment"))
		})
	}
}

func TestTemplateInstanceNetwork(t *testing.T) {
	dir := t.TempDir()
	writeTemplateTestFile(t, filepath.Join(dir, "net@.network"), "[Network]\nDriver=bridge\n")
	writeTemplateTestFile(t, filepath.Join(dir, "net@abc.network.d/override.conf"), "[Network]\nLabel=instance=abc\n")
	units := loadTemplateTestUnits(t, []string{dir})
	require.Len(t, units, 2)
	instance := units[1]
	require.NoError(t, loadUnitDropins(instance, []string{dir}))
	service, _, err := quadlet.ConvertNetwork(instance, generateUnitsInfoMap(units), true)
	require.NoError(t, err)
	assert.Equal(t, "net@abc-network.service", service.Filename)
	exec, _ := service.LookupLast("Service", "ExecStart")
	assert.Contains(t, exec, "--label instance=abc")
}

func TestTemplateInstanceSeparateSearchPaths(t *testing.T) {
	for _, templateFirst := range []bool{false, true} {
		name := "template in lower priority directory"
		if templateFirst {
			name = "template in higher priority directory"
		}
		t.Run(name, func(t *testing.T) {
			templates, dropins := t.TempDir(), t.TempDir()
			writeTemplateTestFile(t, filepath.Join(templates, "app@.container"), "[Container]\nImage=localhost/template\n")
			writeTemplateTestFile(t, filepath.Join(dropins, "app@blue.container.d/override.conf"), "[Container]\nEnvironment=BLUE=yes\n")
			dirs := []string{dropins, templates}
			if templateFirst {
				dirs = []string{templates, dropins}
			}
			units := loadTemplateTestUnits(t, dirs)
			require.Len(t, units, 2)
			assert.Equal(t, "app@blue.container", units[1].Filename)
			assert.Equal(t, filepath.Join(templates, "app@.container"), units[1].Path)
			require.NoError(t, loadUnitDropins(units[1], dirs))
			assert.Equal(t, []string{"BLUE=yes"}, units[1].LookupAll("Container", "Environment"))
		})
	}
}

func TestTemplateInstanceDiscovery(t *testing.T) {
	for ext := range quadlet.SupportedExtensions {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			writeTemplateTestFile(t, filepath.Join(dir, "unit@"+ext), "[Unit]\nDescription=template\n")
			writeTemplateTestFile(t, filepath.Join(dir, "unit@one"+ext+".d/override.conf"), "[Unit]\nDescription=instance\n")
			units := loadTemplateTestUnits(t, []string{dir})
			require.Len(t, units, 2)
			assert.Equal(t, "unit@one"+ext, units[1].Filename)
		})
	}
}

func TestTemplateInstanceIgnoresUnrelatedDropins(t *testing.T) {
	dir := t.TempDir()
	writeTemplateTestFile(t, filepath.Join(dir, "app@.container"), "[Container]\nImage=localhost/template\n")
	for _, name := range []string{"app@.container.d", "app.container.d", "missing@blue.container.d", "app@blue.service.d"} {
		writeTemplateTestFile(t, filepath.Join(dir, name, "override.conf"), "[Unit]\nDescription=override\n")
	}
	writeTemplateTestFile(t, filepath.Join(dir, "app@blue.container.d"), "not a directory")
	units := loadTemplateTestUnits(t, []string{dir, filepath.Join(dir, "nonexistent")})
	assert.Len(t, units, 1)
}

func TestTemplateInstanceSymlinkDropinDirectory(t *testing.T) {
	dir, dropins := t.TempDir(), t.TempDir()
	writeTemplateTestFile(t, filepath.Join(dir, "app@.container"), "[Container]\nImage=localhost/template\n")
	writeTemplateTestFile(t, filepath.Join(dropins, "override.conf"), "[Container]\nEnvironment=BLUE=yes\n")
	require.NoError(t, os.Symlink(dropins, filepath.Join(dir, "app@blue.container.d")))
	units := loadTemplateTestUnits(t, []string{dir})
	require.Len(t, units, 2)
	require.NoError(t, loadUnitDropins(units[1], []string{dir}))
	assert.Equal(t, []string{"BLUE=yes"}, units[1].LookupAll("Container", "Environment"))
}
