package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"time"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

const (
	annotationHookStdout = "run.oci.hooks.stdout"
	annotationHookStderr = "run.oci.hooks.stderr"
)

type RuntimeConfigFilterOptions struct {
	// The hooks to run
	Hooks []spec.Hook
	// The workdir to change when invoking the hook
	Dir string
	// The container config spec to pass into the hook processes and potentially get modified by them
	Config *spec.Spec
	// Timeout for waiting process killed
	PostKillTimeout time.Duration
}

// RuntimeConfigFilterWithOptions calls a series of hooks.  But instead of
// passing container state on their standard input,
// RuntimeConfigFilterWithOptions passes the proposed runtime configuration (and
// reads back a possibly-altered form from their standard output).
func RuntimeConfigFilterWithOptions(ctx context.Context, options RuntimeConfigFilterOptions) (err error) {
	data, err := json.Marshal(options.Config)
	if err != nil {
		return err
	}

	if len(options.Hooks) == 0 {
		return nil
	}

	var stdoutFile, stderrFile *os.File

	if options.Config != nil && options.Config.Annotations != nil {
		if stdoutPath, ok := options.Config.Annotations[annotationHookStdout]; ok {
			f, openErr := os.OpenFile(stdoutPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o700)
			if openErr != nil {
				return fmt.Errorf("opening stdout file for config-filter hook: %w", openErr)
			}
			stdoutFile = f
			defer stdoutFile.Close()
		}

		if stderrPath, ok := options.Config.Annotations[annotationHookStderr]; ok {
			f, openErr := os.OpenFile(stderrPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o700)
			if openErr != nil {
				return fmt.Errorf("opening stderr file for config-filter hook: %w", openErr)
			}
			stderrFile = f
			defer stderrFile.Close()
		}
	}
	for i, hook := range options.Hooks {
		var stdout bytes.Buffer
		var runStdout io.Writer = &stdout
		if stdoutFile != nil {
			runStdout = io.MultiWriter(&stdout, stdoutFile)
		}

		var stderr bytes.Buffer
		var runStderr io.Writer = &stderr
		if stderrFile != nil {
			runStderr = io.MultiWriter(&stderr, stderrFile)
		}

		err = RunWithOptions(ctx, RunOptions{
			Hook:            &hook,
			Dir:             options.Dir,
			State:           data,
			Stdout:          runStdout,
			Stderr:          runStderr,
			PostKillTimeout: options.PostKillTimeout,
		})
		newData := stdout.Bytes()
		if err != nil {
			stderr := stderr.String()
			if len(newData) > 0 || stderr != "" {
				return fmt.Errorf("%w, hook stdout: %s, stderr: %s", err, string(newData), stderr)
			}
			return err
		}

		var newConfig spec.Spec
		err = json.Unmarshal(newData, &newConfig)
		if err != nil {
			logrus.Debugf("invalid JSON from config-filter hook %d:\n%s", i, string(newData))
			return fmt.Errorf("unmarshal output from config-filter hook %d: %w", i, err)
		}

		if !reflect.DeepEqual(options.Config, &newConfig) {
			logrus.Debugf("precreate hook %d made configuration changes from %s to %s", i, data, newData)
		}

		*options.Config = newConfig
		data = newData
	}

	return nil
}
