//go:build linux

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
	type expectedUnit struct {
		filename string
		image    string
		env      []string
	}
	tests := []struct {
		name      string
		highFiles map[string]string
		lowFiles  map[string]string
		emptyDirs []string
		want      []expectedUnit
	}{
		{
			name: "instance drop-ins respect priority and remain isolated",
			highFiles: map[string]string{
				"app@.container":                     "[Container]\nImage=localhost/high\nVolume=./data:/data\n",
				"app@blue.container.d/10-image.conf": "[Container]\nImage=localhost/blue\nEnvironment=BLUE=yes\n",
			},
			lowFiles: map[string]string{
				"app@.container":                     "[Container]\nImage=localhost/low\n",
				"app@.container.d/10-image.conf":     "[Container]\nImage=localhost/template\n",
				"app@.container.d/20-env.conf":       "[Container]\nEnvironment=SHARED=yes\n",
				"app@blue.container.d/10-image.conf": "[Container]\nImage=localhost/wrong\n",
				"app@red.container.d/30-env.conf":    "[Container]\nEnvironment=RED=yes\n",
			},
			want: []expectedUnit{
				{filename: "app@.container", image: "localhost/template", env: []string{"SHARED=yes"}},
				{filename: "app@blue.container", image: "localhost/blue", env: []string{"BLUE=yes", "SHARED=yes"}},
				{filename: "app@red.container", image: "localhost/template", env: []string{"RED=yes", "SHARED=yes"}},
			},
		},
		{
			name: "empty instance directory inherits template drop-ins",
			highFiles: map[string]string{
				"app@.container": "[Container]\nImage=localhost/high\nVolume=./data:/data\n",
			},
			lowFiles: map[string]string{
				"app@.container.d/10-image.conf": "[Container]\nImage=localhost/template\n",
				"app@.container.d/20-env.conf":   "[Container]\nEnvironment=SHARED=yes\n",
			},
			emptyDirs: []string{"app@empty.container.d"},
			want: []expectedUnit{
				{filename: "app@.container", image: "localhost/template", env: []string{"SHARED=yes"}},
				{filename: "app@empty.container", image: "localhost/template", env: []string{"SHARED=yes"}},
			},
		},
		{
			name: "template drop-ins do not create an instance",
			highFiles: map[string]string{
				"app@.container":                 "[Container]\nImage=localhost/high\nVolume=./data:/data\n",
				"app@.container.d/override.conf": "[Container]\nEnvironment=SHARED=yes\n",
			},
			want: []expectedUnit{
				{filename: "app@.container", image: "localhost/high", env: []string{"SHARED=yes"}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			high, low := t.TempDir(), t.TempDir()
			for name, contents := range tc.highFiles {
				writeTemplateTestFile(t, filepath.Join(high, name), contents)
			}
			for name, contents := range tc.lowFiles {
				writeTemplateTestFile(t, filepath.Join(low, name), contents)
			}
			for _, name := range tc.emptyDirs {
				require.NoError(t, os.MkdirAll(filepath.Join(high, name), 0o755))
			}
			dirs := []string{high, low}
			units := loadTemplateTestUnits(t, dirs)
			require.Len(t, units, len(tc.want))
			unitsByName := make(map[string]*parser.UnitFile, len(units))
			for _, unit := range units {
				require.NoError(t, loadUnitDropins(unit, dirs))
				unitsByName[unit.Filename] = unit
			}
			info := generateUnitsInfoMap(units)
			for _, want := range tc.want {
				t.Run(want.filename, func(t *testing.T) {
					require.Contains(t, unitsByName, want.filename)
					unit := unitsByName[want.filename]
					assert.Equal(t, filepath.Join(high, "app@.container"), unit.Path)
					image, _ := unit.LookupLast("Container", "Image")
					assert.Equal(t, want.image, image)
					assert.ElementsMatch(t, want.env, unit.LookupAll("Container", "Environment"))
					service, _, err := quadlet.ConvertContainer(unit, info, true)
					require.NoError(t, err)
					exec, _ := service.LookupLast("Service", "ExecStart")
					assert.Contains(t, exec, want.image)
					assert.Contains(t, exec, filepath.Join(high, "data")+":/data")
					for _, value := range want.env {
						assert.Contains(t, exec, "--env "+value)
					}
				})
			}
		})
	}
}

func TestTemplateInstanceNameParts(t *testing.T) {
	for _, tc := range []struct {
		filename string
		instance string
	}{
		{filename: "app@.container", instance: ""},
		{filename: "app@blue.container", instance: "blue"},
		{filename: "app@..container", instance: "."},
	} {
		t.Run(tc.filename, func(t *testing.T) {
			unit := &parser.UnitFile{Filename: tc.filename}
			base, instance, isTemplate := unit.GetTemplateParts()
			assert.True(t, isTemplate)
			assert.Equal(t, "app", base)
			assert.Equal(t, tc.instance, instance)
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
	for _, name := range []string{"app@.container.d", "app@..container.d", "app.container.d", "missing@blue.container.d", "app@blue.service.d"} {
		writeTemplateTestFile(t, filepath.Join(dir, name, "override.conf"), "[Unit]\nDescription=override\n")
	}
	writeTemplateTestFile(t, filepath.Join(dir, "app@blue.container.d"), "not a directory")
	units := loadTemplateTestUnits(t, []string{dir, filepath.Join(dir, "nonexistent")})
	assert.Len(t, units, 1)
}

func TestTemplateInstanceSymlinkDropinDirectory(t *testing.T) {
	for _, tc := range []struct {
		name    string
		want    int
		wantErr bool
	}{
		{name: "directory", want: 1},
		{name: "file"},
		{name: "missing"},
		{name: "loop", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, targets := t.TempDir(), t.TempDir()
			templatePath := filepath.Join(dir, "app@.container")
			writeTemplateTestFile(t, templatePath, "[Container]\nImage=localhost/template\n")
			template, err := parser.ParseUnitFile(templatePath)
			require.NoError(t, err)

			dropinPath := filepath.Join(dir, "app@blue.container.d")
			target := filepath.Join(targets, "target")
			switch tc.name {
			case "directory":
				writeTemplateTestFile(t, filepath.Join(target, "override.conf"), "[Container]\nEnvironment=BLUE=yes\n")
			case "file":
				writeTemplateTestFile(t, target, "not a directory")
			case "loop":
				target = dropinPath
			}
			require.NoError(t, os.Symlink(target, dropinPath))
			instances, err := loadTemplateInstances([]string{dir}, []*parser.UnitFile{template})
			if tc.wantErr {
				require.ErrorContains(t, err, dropinPath)
			} else {
				require.NoError(t, err)
			}
			require.Len(t, instances, tc.want)
			if tc.want != 0 {
				assert.Equal(t, "app@blue.container", instances[0].Filename)
				require.NoError(t, loadUnitDropins(instances[0], []string{dir}))
				assert.Equal(t, []string{"BLUE=yes"}, instances[0].LookupAll("Container", "Environment"))
			}
		})
	}
}
