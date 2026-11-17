//go:build !remote

package generate

import (
	"fmt"
	"os"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/storage/pkg/fileutils"
)

func specConfigureNamespaces(s *specgen.SpecGenerator, g *generate.Generator, rt *libpod.Runtime, pod *libpod.Pod) error {
	// PID
	switch s.PidNS.NSMode {
	case specgen.Path:
		if err := fileutils.Exists(s.PidNS.Value); err != nil {
			return fmt.Errorf("cannot find specified PID namespace path: %w", err)
		}
		if err := g.AddOrReplaceLinuxNamespace(string(spec.PIDNamespace), s.PidNS.Value); err != nil {
			return err
		}
	case specgen.Host:
		if err := g.RemoveLinuxNamespace(string(spec.PIDNamespace)); err != nil {
			return err
		}
	case specgen.Private:
		if err := g.AddOrReplaceLinuxNamespace(string(spec.PIDNamespace), ""); err != nil {
			return err
		}
	}

	// IPC
	switch s.IpcNS.NSMode {
	case specgen.Path:
		if err := fileutils.Exists(s.IpcNS.Value); err != nil {
			return fmt.Errorf("cannot find specified IPC namespace path: %w", err)
		}
		if err := g.AddOrReplaceLinuxNamespace(string(spec.IPCNamespace), s.IpcNS.Value); err != nil {
			return err
		}
	case specgen.Host:
		if err := g.RemoveLinuxNamespace(string(spec.IPCNamespace)); err != nil {
			return err
		}
	case specgen.Private:
		if err := g.AddOrReplaceLinuxNamespace(string(spec.IPCNamespace), ""); err != nil {
			return err
		}
	}

	// UTS
	switch s.UtsNS.NSMode {
	case specgen.Path:
		if err := fileutils.Exists(s.UtsNS.Value); err != nil {
			return fmt.Errorf("cannot find specified UTS namespace path: %w", err)
		}
		if err := g.AddOrReplaceLinuxNamespace(string(spec.UTSNamespace), s.UtsNS.Value); err != nil {
			return err
		}
	case specgen.Host:
		if err := g.RemoveLinuxNamespace(string(spec.UTSNamespace)); err != nil {
			return err
		}
	case specgen.Private:
		if err := g.AddOrReplaceLinuxNamespace(string(spec.UTSNamespace), ""); err != nil {
			return err
		}
	}

	hostname := s.Hostname
	if hostname == "" {
		switch {
		case s.UtsNS.NSMode == specgen.FromPod:
			hostname = pod.Hostname()
		case s.UtsNS.NSMode == specgen.FromContainer:
			utsCtr, err := rt.LookupContainer(s.UtsNS.Value)
			if err != nil {
				return fmt.Errorf("looking up container to share uts namespace with: %w", err)
			}
			hostname = utsCtr.Hostname()
		case (s.NetNS.NSMode == specgen.Host && hostname == "") || s.UtsNS.NSMode == specgen.Host:
			tmpHostname, err := os.Hostname()
			if err != nil {
				return fmt.Errorf("unable to retrieve hostname of the host: %w", err)
			}
			hostname = tmpHostname
		default:
			logrus.Debug("No hostname set; container's hostname will default to runtime default")
		}
	}

	g.RemoveHostname()
	if s.Hostname != "" || s.UtsNS.NSMode != specgen.Host {
		// Set the hostname in the OCI configuration only if specified by
		// the user or if we are creating a new UTS namespace.
		// TODO: Should we be doing this for pod or container shared
		// namespaces?
		g.SetHostname(hostname)
	}
	if _, ok := s.Env["HOSTNAME"]; !ok && s.Hostname != "" {
		g.AddProcessEnv("HOSTNAME", hostname)
	}

	// User
	if _, err := specgen.SetupUserNS(s.IDMappings, s.UserNS, g); err != nil {
		return err
	}

	// Cgroup
	switch s.CgroupNS.NSMode {
	case specgen.Path:
		if err := fileutils.Exists(s.CgroupNS.Value); err != nil {
			return fmt.Errorf("cannot find specified cgroup namespace path: %w", err)
		}
		if err := g.AddOrReplaceLinuxNamespace(string(spec.CgroupNamespace), s.CgroupNS.Value); err != nil {
			return err
		}
	case specgen.Host:
		if err := g.RemoveLinuxNamespace(string(spec.CgroupNamespace)); err != nil {
			return err
		}
	case specgen.Private:
		if err := g.AddOrReplaceLinuxNamespace(string(spec.CgroupNamespace), ""); err != nil {
			return err
		}
	}

	// Net
	switch s.NetNS.NSMode {
	case specgen.Path:
		if err := fileutils.Exists(s.NetNS.Value); err != nil {
			return fmt.Errorf("cannot find specified network namespace path: %w", err)
		}
		if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), s.NetNS.Value); err != nil {
			return err
		}
	case specgen.Host:
		if err := g.RemoveLinuxNamespace(string(spec.NetworkNamespace)); err != nil {
			return err
		}
	case specgen.Private, specgen.NoNetwork:
		if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), ""); err != nil {
			return err
		}
	}

	if g.Config.Annotations == nil {
		g.Config.Annotations = make(map[string]string)
	}
	if s.PublishExposedPorts != nil && *s.PublishExposedPorts {
		g.Config.Annotations[define.InspectAnnotationPublishAll] = define.InspectResponseTrue
	}

	return nil
}

func needPostConfigureNetNS(s *specgen.SpecGenerator) bool {
	return !s.UserNS.IsHost()
}
