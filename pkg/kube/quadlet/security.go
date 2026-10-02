package quadlet

import (
	"fmt"

	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/systemd/parser"
	"go.podman.io/podman/v6/pkg/systemd/quadlet"
)

// applySecurityContext maps security context fields onto unit.
// Container-level fields take precedence over pod-level fields (K8s semantics).
// supplementalGroups and sysctls are applied by the caller (Convert) at the
// pod level since they affect multiple units.
func applySecurityContext(unit *parser.UnitFile, csc *v1.SecurityContext, psc *v1.PodSecurityContext) {
	applyCapabilities(unit, csc)
	applyPrivileged(unit, csc)
	applyReadOnly(unit, csc)
	applyNoNewPrivileges(unit, csc)
	applyUserGroup(unit, csc, psc)
	applySeLinux(unit, csc, psc)
	applySeccomp(unit, csc, psc)
	applyProcMount(unit, csc)
}

func applyCapabilities(unit *parser.UnitFile, csc *v1.SecurityContext) {
	if csc == nil || csc.Capabilities == nil {
		return
	}
	for _, cap := range csc.Capabilities.Add {
		unit.Add(quadlet.ContainerGroup, quadlet.KeyAddCapability, string(cap))
	}
	for _, cap := range csc.Capabilities.Drop {
		unit.Add(quadlet.ContainerGroup, quadlet.KeyDropCapability, string(cap))
	}
}

func applyPrivileged(unit *parser.UnitFile, csc *v1.SecurityContext) {
	if csc == nil || csc.Privileged == nil || !*csc.Privileged {
		return
	}
	unit.Add(quadlet.ContainerGroup, quadlet.KeyPodmanArgs, "--privileged")
}

func applyReadOnly(unit *parser.UnitFile, csc *v1.SecurityContext) {
	if csc == nil || csc.ReadOnlyRootFilesystem == nil || !*csc.ReadOnlyRootFilesystem {
		return
	}
	unit.Set(quadlet.ContainerGroup, quadlet.KeyReadOnly, "true")
}

func applyNoNewPrivileges(unit *parser.UnitFile, csc *v1.SecurityContext) {
	if csc == nil || csc.AllowPrivilegeEscalation == nil {
		return
	}
	if !*csc.AllowPrivilegeEscalation {
		unit.Set(quadlet.ContainerGroup, quadlet.KeyNoNewPrivileges, "true")
	}
}

// applyUserGroup resolves User= from container-level runAsUser/runAsGroup,
// falling back to pod-level when the container does not set its own.
func applyUserGroup(unit *parser.UnitFile, csc *v1.SecurityContext, psc *v1.PodSecurityContext) {
	var uid, gid *int64

	if csc != nil {
		uid = csc.RunAsUser
		gid = csc.RunAsGroup
	}
	if uid == nil && psc != nil {
		uid = psc.RunAsUser
	}
	if gid == nil && psc != nil {
		gid = psc.RunAsGroup
	}

	if uid == nil && gid == nil {
		return
	}

	var userStr string
	if uid != nil {
		userStr = fmt.Sprintf("%d", *uid)
	}
	if gid != nil {
		userStr = fmt.Sprintf("%s:%d", userStr, *gid)
	}
	unit.Set(quadlet.ContainerGroup, quadlet.KeyUser, userStr)
}

// applySeLinux maps SELinux options, resolving container-level over pod-level.
// SecurityLabelUser and SecurityLabelRole have no native Quadlet key; they go
// via PodmanArgs --security-opt.
func applySeLinux(unit *parser.UnitFile, csc *v1.SecurityContext, psc *v1.PodSecurityContext) {
	var sel *v1.SELinuxOptions
	if csc != nil && csc.SELinuxOptions != nil {
		sel = csc.SELinuxOptions
	} else if psc != nil && psc.SELinuxOptions != nil {
		sel = psc.SELinuxOptions
	}
	if sel == nil {
		return
	}

	if sel.User != "" {
		unit.Add(quadlet.ContainerGroup, quadlet.KeyPodmanArgs,
			fmt.Sprintf("--security-opt label=user:%s", sel.User))
	}
	if sel.Role != "" {
		unit.Add(quadlet.ContainerGroup, quadlet.KeyPodmanArgs,
			fmt.Sprintf("--security-opt label=role:%s", sel.Role))
	}
	if sel.Type != "" {
		unit.Set(quadlet.ContainerGroup, quadlet.KeySecurityLabelType, sel.Type)
	}
	if sel.Level != "" {
		unit.Set(quadlet.ContainerGroup, quadlet.KeySecurityLabelLevel, sel.Level)
	}
	if sel.FileType != "" {
		unit.Set(quadlet.ContainerGroup, quadlet.KeySecurityLabelFileType, sel.FileType)
	}
}

// applySeccomp maps seccompProfile, resolving container-level over pod-level.
func applySeccomp(unit *parser.UnitFile, csc *v1.SecurityContext, psc *v1.PodSecurityContext) {
	var profile *v1.SeccompProfile
	if csc != nil && csc.SeccompProfile != nil {
		profile = csc.SeccompProfile
	} else if psc != nil && psc.SeccompProfile != nil {
		profile = psc.SeccompProfile
	}
	if profile == nil {
		return
	}

	switch profile.Type {
	case v1.SeccompProfileTypeLocalhost:
		if profile.LocalhostProfile != nil {
			unit.Set(quadlet.ContainerGroup, quadlet.KeySeccompProfile, *profile.LocalhostProfile)
		}
	case v1.SeccompProfileTypeRuntimeDefault:
		unit.Set(quadlet.ContainerGroup, quadlet.KeySeccompProfile, "")
	case v1.SeccompProfileTypeUnconfined:
		unit.Set(quadlet.ContainerGroup, quadlet.KeySeccompProfile, "unconfined")
	}
}

func applyProcMount(unit *parser.UnitFile, csc *v1.SecurityContext) {
	if csc == nil || csc.ProcMount == nil {
		return
	}
	if *csc.ProcMount == v1.UnmaskedProcMount {
		unit.Set(quadlet.ContainerGroup, quadlet.KeyUnmask, "ALL")
	}
}

// applySupplementalGroups emits GroupAdd= for each pod-level supplemental group.
// Called for every regular container unit.
func applySupplementalGroups(unit *parser.UnitFile, psc *v1.PodSecurityContext) {
	if psc == nil {
		return
	}
	for _, gid := range psc.SupplementalGroups {
		unit.Add(quadlet.ContainerGroup, quadlet.KeyGroupAdd, fmt.Sprintf("%d", gid))
	}
}
