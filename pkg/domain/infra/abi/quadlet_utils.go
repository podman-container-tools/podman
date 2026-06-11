//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/systemd/parser"
	systemdquadlet "go.podman.io/podman/v6/pkg/systemd/quadlet"
	"go.podman.io/podman/v6/pkg/util"
)

type QuadletFilter func(q *entities.ListQuadlet) bool

func generateQuadletFilter(filter string, filterValues []string) (QuadletFilter, error) {
	switch filter {
	case "name":
		return func(q *entities.ListQuadlet) bool {
			res := util.StringMatchRegexSlice(q.Name, filterValues)
			return res
		}, nil
	case "status":
		return func(q *entities.ListQuadlet) bool {
			res := util.StringMatchRegexSlice(q.Status, filterValues)
			return res
		}, nil
	case "app":
		return func(q *entities.ListQuadlet) bool {
			res := util.StringMatchRegexSlice(q.App, filterValues)
			return res
		}, nil
	case "pod":
		return func(q *entities.ListQuadlet) bool {
			res := util.StringMatchRegexSlice(q.Pod, filterValues)
			return res
		}, nil
	default:
		return nil, fmt.Errorf("%s is not a valid filter", filter)
	}
}

func generateQuadletFilters(filters []string) (QuadletFilter, error) {
	// Create filter functions
	filterFuncs := make([]QuadletFilter, 0, len(filters))
	filterMap := make(map[string][]string)
	for _, f := range filters {
		fname, filter, hasFilter := strings.Cut(f, "=")
		if !hasFilter {
			return nil, fmt.Errorf("invalid filter %q", f)
		}
		filterMap[fname] = append(filterMap[fname], filter)
	}
	for fname, filter := range filterMap {
		filterFunc, err := generateQuadletFilter(fname, filter)
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, filterFunc)
	}

	return func(q *entities.ListQuadlet) bool {
		for _, filterFunc := range filterFuncs {
			if !filterFunc(q) {
				return false
			}
		}
		return true
	}, nil
}

func getQuadlets(dir string) ([]string, error) {
	reports := make([]string, 0)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				logrus.Warnf("Error descending into path %s: %v", path, err)
			}
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		if systemdquadlet.IsExtSupported(d.Name()) {
			reports = append(reports, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return reports, nil
}

func getAllQuadlets(ctx context.Context, conn *dbus.Conn) ([]*entities.ListQuadlet, error) {
	reports := make([]*entities.ListQuadlet, 0)

	// Service name -> quadlet
	partialReports := make(map[string]entities.ListQuadlet)

	// Get the root paths of all quadlets available to the current user
	quadletDirs := systemdquadlet.GetUnitDirs(rootless.IsRootless(), false)

	concreteServiceNames := make([]string, 0)
	templateServiceNames := make([]string, 0)

	// for every quadlet dir, let's get the quadlets
	for _, dir := range quadletDirs {
		quadlets, err := getQuadlets(dir)
		if err != nil {
			return nil, err
		}

		// for every quadlet we found, let's get the corresponding service name
		for _, quadlet := range quadlets {
			basename := filepath.Base(quadlet)
			app := ""

			// Let's compare how "nested" the quadlet is
			// if he is not at the root directory we use the first directory as app name
			rel, err := filepath.Rel(dir, quadlet)
			if err == nil && rel != basename {
				app = strings.Split(rel, string(filepath.Separator))[0]
			}

			report := entities.ListQuadlet{
				Name: basename,
				Path: quadlet,
				App:  app,
			}

			serviceName, unit, err := getQuadletServiceNameAndUnit(quadlet)
			if err != nil {
				report.Status = err.Error()
				reports = append(reports, &report)
				continue
			}

			if pod, ok := unit.Lookup(systemdquadlet.ContainerGroup, systemdquadlet.KeyPod); ok {
				report.Pod = pod
			}

			if systemdquadlet.IsTemplateUnitFileName(serviceName) {
				templateServiceNames = append(templateServiceNames, serviceName)
			} else {
				concreteServiceNames = append(concreteServiceNames, serviceName)
			}
			partialReports[serviceName] = report
		}
	}

	// Get status of concrete systemd units with given names.
	if len(concreteServiceNames) > 0 {
		statuses, err := conn.ListUnitsByNamesContext(ctx, concreteServiceNames)
		if err != nil {
			return nil, fmt.Errorf("querying systemd for unit status: %w", err)
		}
		if len(statuses) != len(concreteServiceNames) {
			logrus.Warnf("Queried for %d services but received %d responses", len(concreteServiceNames), len(statuses))
		}

		for _, unitStatus := range statuses {
			report, ok := partialReports[unitStatus.Name]
			if !ok {
				logrus.Errorf("Unexpected unit returned by systemd - was not searching for %s", unitStatus.Name)
			}
			logrus.Debugf("Unit %s has status %s %s %s", unitStatus.Name, unitStatus.LoadState, unitStatus.ActiveState, unitStatus.SubState)
			report.UnitName = unitStatus.Name

			// Unit is not loaded
			if unitStatus.LoadState != "loaded" {
				report.Status = entities.QuadletStatusNotLoaded
			} else {
				report.Status = fmt.Sprintf("%s/%s", unitStatus.ActiveState, unitStatus.SubState)
			}
			reports = append(reports, &report)
			delete(partialReports, unitStatus.Name)
		}
	}

	// Uninstantiated template units need to be handled separately, because
	// ListUnitsBy* functions do not return anything for templates.
	if len(templateServiceNames) > 0 {
		unitFiles, err := conn.ListUnitFilesByPatternsContext(ctx, []string{}, templateServiceNames)
		if err != nil {
			return nil, fmt.Errorf("querying systemd for unit file: %w", err)
		}
		unitFilesFound := make(map[string]struct{})
		for _, unitFile := range unitFiles {
			unitFilesFound[filepath.Base(unitFile.Path)] = struct{}{}
		}
		for _, templateServiceName := range templateServiceNames {
			report := partialReports[templateServiceName]

			report.UnitName = templateServiceName

			if _, ok := unitFilesFound[templateServiceName]; ok {
				report.Status = entities.QuadletStatusLoadedTemplate
			} else {
				report.Status = entities.QuadletStatusNotLoaded
			}

			reports = append(reports, &report)
			delete(partialReports, templateServiceName)
		}
	}

	// This should not happen.
	// Systemd will give us output for everything we sent to them, even if it's not a valid unit.
	// We can find them with LoadState, as we do above.
	// Handle it anyways because it's easy enough to do.
	for _, report := range partialReports {
		report.Status = entities.QuadletStatusNotLoaded
		reports = append(reports, &report)
	}

	return reports, nil
}

func removeQuadlet(ctx context.Context, conn *dbus.Conn, quadlet *entities.ListQuadlet, force bool) error {
	if quadlet.Status == "active/running" {
		if !force {
			return fmt.Errorf("quadlet %s is running and force is not set, refusing to remove: %w", quadlet.Name, define.ErrQuadletRunning)
		}
		logrus.Debugf("Going to stop systemd unit %s (Quadlet %s)", quadlet.Name, quadlet.Path)

		ch := make(chan string)
		if _, err := conn.StopUnitContext(ctx, quadlet.UnitName, "replace", ch); err != nil {
			return fmt.Errorf("stopping quadlet %s: %w", quadlet.Name, err)
		}

		logrus.Debugf("Waiting for systemd unit %s to stop", quadlet.Name)
		stopResult := <-ch
		if stopResult != "done" && stopResult != "skipped" {
			return fmt.Errorf("unable to stop quadlet %s: %s", quadlet.Name, stopResult)
		}
	}
	if quadlet.Status == entities.QuadletStatusLoadedTemplate {
		if err := stopLoadedTemplateInstances(ctx, conn, quadlet, force); err != nil {
			return err
		}
	}

	return os.Remove(quadlet.Path)
}

// stopLoadedTemplateInstances handles the running instances of a template.
// If force is set, the instances are stopped and error is returned on failure.
// If force is not set, stopping is refused and error is returned on running instances.
func stopLoadedTemplateInstances(ctx context.Context, conn *dbus.Conn, quadlet *entities.ListQuadlet, force bool) error {
	extension := filepath.Ext(quadlet.UnitName)

	// The regular expression in this pattern matches the template itself as well, but the
	// ListUnitsBy* functions do not return anything for templates.
	instancePattern := strings.TrimSuffix(quadlet.UnitName, extension) + "*" + extension
	instances, err := conn.ListUnitsByPatternsContext(ctx, []string{}, []string{instancePattern})
	if err != nil {
		return fmt.Errorf("querying systemd for instances of %q: %w", quadlet.UnitName, err)
	}

	var runningInstanceErrors []error
	for _, instanceStatus := range instances {
		properties, err := conn.GetUnitPropertiesContext(ctx, instanceStatus.Name)
		if err != nil {
			logrus.Errorf("getting unit properties for %q: %v", instanceStatus.Name, err)
			continue
		}

		sourcePath, ok := properties["SourcePath"].(string)
		if !ok || sourcePath == "" {
			logrus.Warnf("source path not found for unit %q", instanceStatus.Name)
			continue
		}

		if filepath.Clean(sourcePath) != filepath.Clean(quadlet.Path) {
			// Nothing to do here if the unit is not associated with this quadlet
			continue
		}

		if instanceStatus.LoadState != "loaded" {
			// Nothing to do here if the instance is not loaded
			continue
		}

		if instanceStatus.ActiveState == "active" {
			if !force {
				runningInstanceErrors = append(runningInstanceErrors, fmt.Errorf("template %q has running instance %q and force is not set, refusing to remove: %w", quadlet.Name, instanceStatus.Name, define.ErrQuadletRunning))
				continue
			}
			logrus.Debugf("Going to stop systemd unit %q (Instance of template %q)", instanceStatus.Name, quadlet.Name)

			ch := make(chan string)
			if _, err := conn.StopUnitContext(ctx, instanceStatus.Name, "replace", ch); err != nil {
				runningInstanceErrors = append(runningInstanceErrors, fmt.Errorf("stopping instance %q of template %q: %w", instanceStatus.Name, quadlet.Name, err))
				continue
			}

			logrus.Debugf("Waiting for systemd unit %q to stop", instanceStatus.Name)
			stopResult := <-ch
			if stopResult != "done" && stopResult != "skipped" {
				runningInstanceErrors = append(runningInstanceErrors, fmt.Errorf("unable to stop instance %q of template %q: %q", instanceStatus.Name, quadlet.Name, stopResult))
			}
		}
	}

	if len(runningInstanceErrors) > 0 {
		return errors.Join(runningInstanceErrors...)
	}

	return nil
}

// getQuadletServiceNameAndUnit parses a Quadlet file and returns both the
// generated systemd service name and the parsed unit file.
func getQuadletServiceNameAndUnit(quadletPath string) (string, *parser.UnitFile, error) {
	unit, err := parser.ParseUnitFile(quadletPath)
	if err != nil {
		return "", nil, fmt.Errorf("parsing Quadlet file %s: %w", quadletPath, err)
	}

	serviceName, err := systemdquadlet.GetUnitServiceName(unit)
	if err != nil {
		return "", nil, fmt.Errorf("generating service name for Quadlet %s: %w", filepath.Base(quadletPath), err)
	}
	return serviceName + ".service", unit, nil
}

func getApplicationPath(app string) (string, error) {
	// Get the root paths of all quadlets available to the current user
	quadletDirs := systemdquadlet.GetUnitDirs(rootless.IsRootless(), false)

	// for every quadlet dir, let's get the quadlets
	for _, dir := range quadletDirs {
		// Avoiding using filepath.join with "app", as it may lead to
		// path traversal attack
		files, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}

		if err != nil {
			return "", err
		}

		for _, file := range files {
			if file.IsDir() && file.Name() == app {
				return filepath.Join(dir, file.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("application %s not found", app)
}

func validateApplicationName(baseDir string, application string) error {
	if !filepath.IsAbs(baseDir) {
		return fmt.Errorf("base directory must be an absolute path")
	}

	if len(application) == 0 {
		return fmt.Errorf("application name cannot be empty")
	}

	if strings.Contains(application, string(os.PathSeparator)) {
		return fmt.Errorf("invalid application name")
	}

	if application == "." || application == ".." {
		return fmt.Errorf("invalid application name")
	}

	joined := filepath.Join(baseDir, application)

	fullAbs, err := filepath.Abs(joined)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(fullAbs, baseDir+string(os.PathSeparator)) {
		return fmt.Errorf("invalid application name, must be a subdirectory of the base directory")
	}

	if systemdquadlet.IsExtSupported(application) {
		return fmt.Errorf("invalid application name, names can't end end with a quadlet extension")
	}

	return nil
}
