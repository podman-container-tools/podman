package kube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/kube/quadlet"
	"sigs.k8s.io/yaml"

	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

// quadletJSON is the element type for --format json output.
type quadletJSON struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

var (
	quadletOptions = struct {
		OutputDir  string
		Format     string
		NamePrefix string
		Network    string
		ConfigMaps []string
		Secrets    []string
	}{}

	kubeQuadletCmd = &cobra.Command{
		Use:   "quadlet [options] POD_YAML",
		Short: "Convert a Kubernetes Pod YAML into Quadlet unit files",
		Long: `Reads a Kubernetes Pod YAML file and generates the corresponding Quadlet
unit files (.pod, .container, .volume, .env, .sh) without connecting to a Podman daemon.

A .pod unit is always generated so that all containers share a proper infra
container (like a Kubernetes pause container) rather than one container acting
as an anchor for the others.

The output files can be written to a directory or printed to stdout.  Use
--format json for programmatic consumption.`,
		RunE:              runKubeQuadlet,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.AutocompleteDefault,
		// Override the root PersistentPreRunE so no daemon/storage setup is needed.
		PersistentPreRunE: validate.NoOp,
		Example: `podman kube quadlet pod.yaml
podman kube quadlet --output-dir /etc/containers/systemd pod.yaml
podman kube quadlet --format json pod.yaml | jq .[].name`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: kubeQuadletCmd,
		Parent:  kubeCmd,
	})

	flags := kubeQuadletCmd.Flags()

	outputDirFlagName := "output-dir"
	flags.StringVarP(&quadletOptions.OutputDir, outputDirFlagName, "o", "", "Write generated files to this directory (default: print to stdout)")
	_ = kubeQuadletCmd.RegisterFlagCompletionFunc(outputDirFlagName, completion.AutocompleteDefault)

	formatFlagName := "format"
	flags.StringVar(&quadletOptions.Format, formatFlagName, "text", `Output format: "text" or "json"`)
	_ = kubeQuadletCmd.RegisterFlagCompletionFunc(formatFlagName, completion.AutocompleteNone)

	namePrefixFlagName := "name-prefix"
	flags.StringVar(&quadletOptions.NamePrefix, namePrefixFlagName, "", "Override the name prefix for generated files (default: pod.metadata.name)")
	_ = kubeQuadletCmd.RegisterFlagCompletionFunc(namePrefixFlagName, completion.AutocompleteNone)

	networkFlagName := "network"
	flags.StringVar(&quadletOptions.Network, networkFlagName, "", "Network= value for the pod unit (e.g. podman, host)")
	_ = kubeQuadletCmd.RegisterFlagCompletionFunc(networkFlagName, completion.AutocompleteNone)

	configmapFlagName := "configmap"
	flags.StringArrayVar(&quadletOptions.ConfigMaps, configmapFlagName, nil, "Path to a ConfigMap YAML file (repeatable)")
	_ = kubeQuadletCmd.RegisterFlagCompletionFunc(configmapFlagName, completion.AutocompleteDefault)

	secretFlagName := "secret"
	flags.StringArrayVar(&quadletOptions.Secrets, secretFlagName, nil, "Path to a Secret YAML file (repeatable)")
	_ = kubeQuadletCmd.RegisterFlagCompletionFunc(secretFlagName, completion.AutocompleteDefault)
}

func runKubeQuadlet(_ *cobra.Command, args []string) error {
	if quadletOptions.Format != "text" && quadletOptions.Format != "json" {
		return fmt.Errorf("--format must be \"text\" or \"json\", got %q", quadletOptions.Format)
	}

	pod, err := podFromFile(args[0])
	if err != nil {
		return err
	}

	opts, err := buildQuadletOptions()
	if err != nil {
		return err
	}

	files, err := quadlet.Convert(pod, opts)
	if err != nil {
		return fmt.Errorf("converting pod to Quadlet: %w", err)
	}

	if quadletOptions.OutputDir != "" {
		return writeFiles(files, quadletOptions.OutputDir)
	}

	switch quadletOptions.Format {
	case "json":
		return printJSON(files)
	default:
		return printText(files)
	}
}

// podFromFile reads and unmarshals a Kubernetes Pod from a YAML file.
func podFromFile(path string) (*v1.Pod, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}

	// Detect kind before committing to Pod decode.
	var meta struct {
		Kind string `json:"kind"`
	}
	if err := yaml.Unmarshal(raw, &meta); err != nil {
		return nil, fmt.Errorf("parsing YAML from %q: %w", path, err)
	}
	if meta.Kind != "Pod" {
		return nil, fmt.Errorf("%q has kind %q; only \"Pod\" is supported", path, meta.Kind)
	}

	var pod v1.Pod
	if err := yaml.Unmarshal(raw, &pod); err != nil {
		return nil, fmt.Errorf("parsing Pod from %q: %w", path, err)
	}
	return &pod, nil
}

// buildQuadletOptions constructs Options from CLI flags and any provided
// ConfigMap/Secret YAML files.
func buildQuadletOptions() (quadlet.Options, error) {
	opts := quadlet.Options{
		NamePrefix: quadletOptions.NamePrefix,
		Network:    quadletOptions.Network,
	}

	if len(quadletOptions.ConfigMaps) > 0 {
		opts.ConfigMaps = make(map[string]v1.ConfigMap)
		for _, cmPath := range quadletOptions.ConfigMaps {
			cm, err := loadConfigMap(cmPath)
			if err != nil {
				return opts, err
			}
			opts.ConfigMaps[cm.Name] = cm
		}
	}

	if len(quadletOptions.Secrets) > 0 {
		opts.Secrets = make(map[string]v1.Secret)
		for _, secPath := range quadletOptions.Secrets {
			sec, err := loadSecret(secPath)
			if err != nil {
				return opts, err
			}
			opts.Secrets[sec.Name] = sec
		}
	}

	return opts, nil
}

func loadConfigMap(path string) (v1.ConfigMap, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return v1.ConfigMap{}, fmt.Errorf("reading ConfigMap %q: %w", path, err)
	}
	var cm v1.ConfigMap
	if err := yaml.Unmarshal(raw, &cm); err != nil {
		return v1.ConfigMap{}, fmt.Errorf("parsing ConfigMap %q: %w", path, err)
	}
	return cm, nil
}

func loadSecret(path string) (v1.Secret, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return v1.Secret{}, fmt.Errorf("reading Secret %q: %w", path, err)
	}
	var sec v1.Secret
	if err := yaml.Unmarshal(raw, &sec); err != nil {
		return v1.Secret{}, fmt.Errorf("parsing Secret %q: %w", path, err)
	}
	return sec, nil
}

// writeFiles writes each GeneratedFile to outputDir.
func writeFiles(files []*quadlet.GeneratedFile, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory %q: %w", outputDir, err)
	}
	for _, f := range files {
		dest := filepath.Join(outputDir, f.Name)
		var content []byte
		if f.Unit != nil {
			var buf bytes.Buffer
			if err := f.Unit.Write(&buf); err != nil {
				return fmt.Errorf("serializing %q: %w", f.Name, err)
			}
			content = buf.Bytes()
		} else {
			content = []byte(f.Content)
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return fmt.Errorf("writing %q: %w", dest, err)
		}
		fmt.Println(dest)
	}
	return nil
}

// printText prints each file separated by a header line.
func printText(files []*quadlet.GeneratedFile) error {
	for i, f := range files {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("=== %s ===\n", f.Name)
		if f.Unit != nil {
			var buf bytes.Buffer
			if err := f.Unit.Write(&buf); err != nil {
				return fmt.Errorf("serializing %q: %w", f.Name, err)
			}
			fmt.Print(buf.String())
		} else {
			fmt.Print(f.Content)
		}
	}
	return nil
}

// printJSON emits a JSON array of {name, content} objects.
func printJSON(files []*quadlet.GeneratedFile) error {
	out := make([]quadletJSON, 0, len(files))
	for _, f := range files {
		var content string
		if f.Unit != nil {
			var buf bytes.Buffer
			if err := f.Unit.Write(&buf); err != nil {
				return fmt.Errorf("serializing %q: %w", f.Name, err)
			}
			content = buf.String()
		} else {
			content = f.Content
		}
		out = append(out, quadletJSON{Name: f.Name, Content: content})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
