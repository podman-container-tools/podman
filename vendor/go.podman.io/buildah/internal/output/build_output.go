package output

import (
	"fmt"
	"strings"
)

type BuildOutputType int

const (
	BuildOutputInvalid  BuildOutputType = 0
	BuildOutputStdout   BuildOutputType = 1 // stream tar to stdout
	BuildOutputLocalDir BuildOutputType = 2
	BuildOutputTar      BuildOutputType = 3
)

// BuildOutputOptions contains the the outcome of parsing the value of a build --output flag
type BuildOutputOption struct {
	Type BuildOutputType
	Path string // Only valid if Type is local dir or tar
}

// GetBuildOutput is responsible for parsing custom build output argument i.e `build --output` flag.
// Takes `buildOutput` as string and returns BuildOutputOption
func GetBuildOutput(buildOutput string) (BuildOutputOption, error) {
	// Support simple values, in the form --output ./mydir
	if !strings.Contains(buildOutput, ",") && !strings.Contains(buildOutput, "=") {
		if buildOutput == "-" {
			// Feature parity with buildkit, output tar to stdout
			// Read more here: https://docs.docker.com/engine/reference/commandline/build/#custom-build-outputs
			return BuildOutputOption{
				Type: BuildOutputStdout,
				Path: "",
			}, nil
		}

		return BuildOutputOption{
			Type: BuildOutputLocalDir,
			Path: buildOutput,
		}, nil
	}

	// Support complex values, in the form --output type=local,dest=./mydir
	typeSelected := BuildOutputInvalid
	pathSelected := ""
	for option := range strings.SplitSeq(buildOutput, ",") {
		key, value, found := strings.Cut(option, "=")
		if !found {
			return BuildOutputOption{}, fmt.Errorf("invalid build output options %q, expected format key=value", buildOutput)
		}
		switch key {
		case "type":
			if typeSelected != BuildOutputInvalid {
				return BuildOutputOption{}, fmt.Errorf("duplicate %q not supported", key)
			}
			switch value {
			case "local":
				typeSelected = BuildOutputLocalDir
			case "tar":
				typeSelected = BuildOutputTar
			default:
				return BuildOutputOption{}, fmt.Errorf("invalid type %q selected for build output options %q", value, buildOutput)
			}
		case "dest":
			if pathSelected != "" {
				return BuildOutputOption{}, fmt.Errorf("duplicate %q not supported", key)
			}
			pathSelected = value
		default:
			return BuildOutputOption{}, fmt.Errorf("unrecognized key %q in build output option: %q", key, buildOutput)
		}
	}

	// Validate there is a type
	if typeSelected == BuildOutputInvalid {
		return BuildOutputOption{}, fmt.Errorf("missing required key %q in build output option: %q", "type", buildOutput)
	}

	// Validate path
	if typeSelected == BuildOutputLocalDir || typeSelected == BuildOutputTar {
		if pathSelected == "" {
			return BuildOutputOption{}, fmt.Errorf("missing required key %q in build output option: %q", "dest", buildOutput)
		}
	} else {
		// Clear path when not needed by type
		pathSelected = ""
	}

	// Handle redirecting stdout for tar output
	if pathSelected == "-" {
		if typeSelected == BuildOutputTar {
			typeSelected = BuildOutputStdout
			pathSelected = ""
		} else {
			return BuildOutputOption{}, fmt.Errorf(`invalid build output option %q, only "type=tar" can be used with "dest=-"`, buildOutput)
		}
	}

	return BuildOutputOption{
		Type: typeSelected,
		Path: pathSelected,
	}, nil
}
