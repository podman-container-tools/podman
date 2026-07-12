package farm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
	"go.podman.io/buildah/define"
	lplatform "go.podman.io/common/libimage/platform"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra"
)

// Farm represents a group of connections to builders.
type Farm struct {
	name        string
	localEngine entities.ImageEngine            // not nil -> use local engine, too
	builders    map[string]entities.ImageEngine // name -> builder
}

// Schedule is a description of where and how we'll do builds.
type Schedule struct {
	platformBuilders map[string]string // target->connection
}

func newFarmWithBuilders(_ context.Context, name string, cons []config.Connection, localEngine entities.ImageEngine, buildLocal bool) (*Farm, error) {
	farm := &Farm{
		builders:    make(map[string]entities.ImageEngine),
		localEngine: localEngine,
		name:        name,
	}
	var (
		builderMutex sync.Mutex
		builderGroup multierror.Group
	)
	// Set up the remote connections to handle the builds
	for _, con := range cons {
		builderGroup.Go(func() error {
			fmt.Printf("Connecting to %q\n", con.Name)
			engine, err := infra.NewImageEngine(&entities.PodmanConfig{
				EngineMode:   entities.TunnelMode,
				URI:          con.URI,
				Identity:     con.Identity,
				MachineMode:  con.IsMachine,
				FarmNodeName: con.Name,
			})
			if err != nil {
				return fmt.Errorf("initializing image engine at %q: %w", con.URI, err)
			}

			defer fmt.Printf("Builder %q ready\n", con.Name)
			builderMutex.Lock()
			defer builderMutex.Unlock()
			farm.builders[con.Name] = engine
			return nil
		})
	}
	// If local=true then use the local machine for builds as well
	if buildLocal {
		builderGroup.Go(func() error {
			fmt.Println("Setting up local builder")
			defer fmt.Println("Local builder ready")
			builderMutex.Lock()
			defer builderMutex.Unlock()
			farm.builders[entities.LocalFarmImageBuilderName] = localEngine
			return nil
		})
	}
	if builderError := builderGroup.Wait(); builderError != nil {
		if err := builderError.ErrorOrNil(); err != nil {
			return nil, err
		}
	}
	if len(farm.builders) > 0 {
		defer fmt.Printf("Farm %q ready\n", farm.name)
		return farm, nil
	}
	return nil, errors.New("no builders configured")
}

func NewFarm(ctx context.Context, name string, localEngine entities.ImageEngine, buildLocal bool) (*Farm, error) {
	// Get the destinations of the connections specified in the farm
	name, destinations, err := getFarmDestinations(name)
	if err != nil {
		return nil, err
	}

	return newFarmWithBuilders(ctx, name, destinations, localEngine, buildLocal)
}

// NativePlatforms returns a list of the set of platforms for which the farm
// can build images natively.
func (f *Farm) NativePlatforms(ctx context.Context) ([]string, error) {
	nativeMap := make(map[string]struct{})
	platforms := []string{}
	var (
		nativeMutex sync.Mutex
		nativeGroup multierror.Group
	)
	for _, engine := range f.builders {
		nativeGroup.Go(func() error {
			logrus.Debugf("getting native platform of %q\n", engine.FarmNodeName(ctx))
			defer logrus.Debugf("got native platform of %q", engine.FarmNodeName(ctx))
			inspect, err := engine.FarmNodeInspect(ctx)
			if err != nil {
				return err
			}
			nativeMutex.Lock()
			defer nativeMutex.Unlock()
			for _, platform := range inspect.NativePlatforms {
				nativeMap[platform] = struct{}{}
			}
			return nil
		})
	}
	merr := nativeGroup.Wait()
	if merr != nil {
		if err := merr.ErrorOrNil(); err != nil {
			return nil, err
		}
	}

	for platform := range nativeMap {
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)
	return platforms, nil
}

// Schedule takes a list of platforms and returns a list of connections which
// can be used to build for those platforms.  It always prefers native builders
// over emulated builders, but will assign a builder which can use emulation
// for a platform if no suitable native builder is available.
//
// If platforms is an empty list, all available native platforms will be
// scheduled.
//
// TODO: add (Priority,Weight *int) a la RFC 2782 to destinations that we know
// of, and factor those in when assigning builds to nodes in here.
func (f *Farm) Schedule(ctx context.Context, platforms []string) (Schedule, error) {
	var (
		err       error
		infoGroup multierror.Group
		infoMutex sync.Mutex
	)
	// If we weren't given a list of target platforms, generate one.
	if len(platforms) == 0 {
		platforms, err = f.NativePlatforms(ctx)
		if err != nil {
			return Schedule{}, fmt.Errorf("reading list of available native platforms: %w", err)
		}
	}

	platformBuilders := make(map[string]string)
	native := make(map[string]string)
	emulated := make(map[string]string)
	var localPlatform string
	// Make notes of which platforms we can build for natively, and which
	// ones we can build for using emulation.
	for name, engine := range f.builders {
		infoGroup.Go(func() error {
			inspect, err := engine.FarmNodeInspect(ctx)
			if err != nil {
				return err
			}
			infoMutex.Lock()
			defer infoMutex.Unlock()
			for _, n := range inspect.NativePlatforms {
				if _, assigned := native[n]; !assigned {
					native[n] = name
				}
				if name == entities.LocalFarmImageBuilderName {
					localPlatform = n
				}
			}
			for _, e := range inspect.EmulatedPlatforms {
				if _, assigned := emulated[e]; !assigned {
					emulated[e] = name
				}
			}
			return nil
		})
	}
	merr := infoGroup.Wait()
	if merr != nil {
		if err := merr.ErrorOrNil(); err != nil {
			return Schedule{}, err
		}
	}
	// Assign a build to the first node that could build it natively, and
	// if there isn't one, the first one that can build it with the help of
	// emulation, and if there aren't any, error out.
	for _, platform := range platforms {
		if builder, ok := native[platform]; ok {
			platformBuilders[platform] = builder
		} else if builder, ok := emulated[platform]; ok {
			platformBuilders[platform] = builder
		} else {
			return Schedule{}, fmt.Errorf("no builder capable of building for platform %q available", platform)
		}
	}
	// If local is set, prioritize building on local
	if localPlatform != "" {
		platformBuilders[localPlatform] = entities.LocalFarmImageBuilderName
	}
	schedule := Schedule{
		platformBuilders: platformBuilders,
	}
	return schedule, nil
}

// Build runs a build using the specified targetplatform:service map.  If all
// builds succeed, it copies the resulting images from the remote hosts to the
// local service and builds a manifest list with the specified reference name.
func (f *Farm) Build(ctx context.Context, schedule Schedule, options entities.BuildOptions, reference string, _ entities.ImageEngine) error {
	switch options.OutputFormat {
	default:
		return fmt.Errorf("unknown output format %q requested", options.OutputFormat)
	case "", define.OCIv1ImageManifest:
		options.OutputFormat = define.OCIv1ImageManifest
	case define.Dockerv2ImageManifest:
	}

	// Build the list of jobs.
	var jobs sync.Map
	type job struct {
		platform string
		os       string
		arch     string
		variant  string
		builder  entities.ImageEngine
	}
	for platform, builderName := range schedule.platformBuilders { // prepare to build
		builder, ok := f.builders[builderName]
		if !ok {
			return fmt.Errorf("unknown builder %q", builderName)
		}
		var rawOS, rawArch, rawVariant string
		p := strings.Split(platform, "/")
		if len(p) > 0 && p[0] != "" {
			rawOS = p[0]
		}
		if len(p) > 1 {
			rawArch = p[1]
		}
		if len(p) > 2 {
			rawVariant = p[2]
		}
		os, arch, variant := lplatform.Normalize(rawOS, rawArch, rawVariant)
		jobs.Store(builderName, job{
			platform: platform,
			os:       os,
			arch:     arch,
			variant:  variant,
			builder:  builder,
		})
	}

	listBuilderOptions := listBuilderOptions{
		cleanup:       options.Cleanup,
		iidFile:       options.IIDFile,
		iidFileRaw:    options.IIDFileRaw,
		authfile:      options.Authfile,
		skipTLSVerify: options.SkipTLSVerify,
	}
	manifestListBuilder := newManifestListBuilder(reference, f.localEngine, listBuilderOptions)

	// Start builds in parallel and wait for them all to finish.
	var (
		buildResults sync.Map
		buildGroup   multierror.Group
	)
	type buildResult struct {
		report  entities.BuildReport
		builder entities.ImageEngine
	}
	for platform, builder := range schedule.platformBuilders {
		outReader, outWriter := io.Pipe()
		errReader, errWriter := io.Pipe()
		go func() {
			defer outReader.Close()
			reader := bufio.NewReader(outReader)
			writer := options.Out
			if writer == nil {
				writer = os.Stdout
			}
			line, err := reader.ReadString('\n')
			for err == nil {
				line = strings.TrimSuffix(line, "\n")
				fmt.Fprintf(writer, "[%s@%s] %s\n", platform, builder, line)
				line, err = reader.ReadString('\n')
			}
		}()
		go func() {
			defer errReader.Close()
			reader := bufio.NewReader(errReader)
			writer := options.Err
			if writer == nil {
				writer = os.Stderr
			}
			line, err := reader.ReadString('\n')
			for err == nil {
				line = strings.TrimSuffix(line, "\n")
				fmt.Fprintf(writer, "[%s@%s] %s\n", platform, builder, line)
				line, err = reader.ReadString('\n')
			}
		}()
		buildGroup.Go(func() error {
			var j job
			defer outWriter.Close()
			defer errWriter.Close()
			c, ok := jobs.Load(builder)
			if !ok {
				return fmt.Errorf("unknown connection for %q (shouldn't happen)", builder)
			}
			if j, ok = c.(job); !ok {
				return fmt.Errorf("unexpected connection type for %q (shouldn't happen)", builder)
			}
			buildOptions := options
			buildOptions.Platforms = []struct{ OS, Arch, Variant string }{{j.os, j.arch, j.variant}}
			buildOptions.Out = outWriter
			buildOptions.Err = errWriter
			fmt.Printf("Starting build for %v at %q\n", buildOptions.Platforms, builder)
			buildReport, err := j.builder.Build(ctx, options.ContainerFiles, buildOptions)
			if err != nil {
				return fmt.Errorf("building for %q on %q: %w", j.platform, builder, err)
			}
			fmt.Printf("finished build for %v at %q: built %s\n", buildOptions.Platforms, builder, buildReport.ID)
			buildResults.Store(platform, buildResult{
				report:  *buildReport,
				builder: j.builder,
			})
			return nil
		})
	}
	buildErrors := buildGroup.Wait()
	if err := buildErrors.ErrorOrNil(); err != nil {
		return fmt.Errorf("building: %w", err)
	}

	// Assemble the final result.
	perArchBuilds := make(map[entities.BuildReport]entities.ImageEngine)
	buildResults.Range(func(_, v any) bool {
		result, ok := v.(buildResult)
		if !ok {
			fmt.Fprintf(os.Stderr, "report %v not a build result?\n", v)
			return false
		}
		perArchBuilds[result.report] = result.builder
		return true
	})
	location, err := manifestListBuilder.build(ctx, perArchBuilds)
	if err != nil {
		return err
	}
	fmt.Printf("Saved list to %q\n", location)
	return nil
}

func getFarmDestinations(name string) (string, []config.Connection, error) {
	cfg, err := config.Default()
	if err != nil {
		return "", nil, err
	}

	if name == "" {
		if name, cons, err := cfg.GetDefaultFarmConnections(); err == nil {
			// Use default farm if is there is one
			return name, cons, nil
		}
		// If no farm name is given, then grab all the service destinations available
		cons, err := cfg.GetAllConnections()
		return name, cons, err
	}
	cons, err := cfg.GetFarmConnections(name)
	return name, cons, err
}
