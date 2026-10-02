package quadlet

import (
	"io"

	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/systemd/parser"
)

// Options controls how a Kubernetes Pod spec is translated into Quadlet unit files.
type Options struct {
	// NamePrefix is prepended to every generated filename.
	// Defaults to pod.Name.
	NamePrefix string

	// Network is the Quadlet Network= value for the pod unit.
	// Empty string omits the directive (Podman default: pasta for rootless, bridge for root).
	Network string

	// ConfigMaps provides ConfigMap data to resolve configMapKeyRef and configMap volume sources.
	// Key: ConfigMap name.
	ConfigMaps map[string]v1.ConfigMap

	// Secrets provides Secret data to resolve secretKeyRef and secret volume sources.
	// Key: Secret name.
	Secrets map[string]v1.Secret

	// ConfigMapDir is the host directory where ConfigMap volume files will be written.
	// Required when ConfigMaps is non-empty and volume-type configMap sources are present.
	ConfigMapDir string
}

// GeneratedFile is a single file produced by Convert.
//
// Exactly one of Unit or Content is set:
//   - Unit is set for structured Quadlet unit files (.pod, .container, .volume). Use
//     Unit.Write or Unit.ToString to serialize.
//   - Content is set for raw text files (.sh init scripts, .env files).
type GeneratedFile struct {
	// Name is the output filename (e.g. "myapp.pod", "myapp-web.container", "myapp-init.sh").
	Name string
	// Unit is non-nil for .pod, .container and .volume Quadlet unit files.
	Unit *parser.UnitFile
	// Content is non-empty for .sh and .env plain-text files.
	Content string
}

// Write serializes the file to w.
func (f *GeneratedFile) Write(w io.Writer) error {
	if f.Unit != nil {
		return f.Unit.Write(w)
	}
	_, err := io.WriteString(w, f.Content)
	return err
}
