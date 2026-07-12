package tunnel

import (
	"context"
	"fmt"
	"io"

	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/pkg/bindings/generate"
	"go.podman.io/podman/v6/pkg/bindings/kube"
	"go.podman.io/podman/v6/pkg/bindings/play"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ic *ContainerEngine) GenerateSystemd(_ context.Context, nameOrID string, opts entities.GenerateSystemdOptions) (*entities.GenerateSystemdReport, error) {
	options := new(
		generate.SystemdOptions).
		WithUseName(opts.Name).
		WithContainerPrefix(opts.ContainerPrefix).
		WithNew(opts.New).WithNoHeader(opts.NoHeader).
		WithTemplateUnitFile(opts.TemplateUnitFile).
		WithPodPrefix(opts.PodPrefix).
		WithSeparator(opts.Separator).
		WithWants(opts.Wants).
		WithAfter(opts.After).
		WithRequires(opts.Requires).
		WithAdditionalEnvVariables(opts.AdditionalEnvVariables)

	if opts.StartTimeout != nil {
		options.WithStartTimeout(*opts.StartTimeout)
	}
	if opts.StopTimeout != nil {
		options.WithStopTimeout(*opts.StopTimeout)
	}
	if opts.RestartPolicy != nil {
		options.WithRestartPolicy(*opts.RestartPolicy)
	}
	if opts.RestartSec != nil {
		options.WithRestartSec(*opts.RestartSec)
	}

	return generate.Systemd(ic.ClientCtx, nameOrID, options)
}

// GenerateKube Kubernetes YAML (v1 specification) for nameOrIDs
//
// Note: Caller is responsible for closing returned Reader
func (ic *ContainerEngine) GenerateKube(_ context.Context, nameOrIDs []string, opts entities.GenerateKubeOptions) (*entities.GenerateKubeReport, error) {
	options := new(generate.KubeOptions).WithService(opts.Service).WithType(opts.Type).WithReplicas(opts.Replicas).WithNoTrunc(opts.UseLongAnnotations).WithPodmanOnly(opts.PodmanOnly)
	return generate.Kube(ic.ClientCtx, nameOrIDs, options)
}

func (ic *ContainerEngine) GenerateSpec(_ context.Context, _ *entities.GenerateSpecOptions) (*entities.GenerateSpecReport, error) {
	return nil, fmt.Errorf("GenerateSpec is not supported on the remote API")
}

func (ic *ContainerEngine) PlayKube(_ context.Context, body io.Reader, opts entities.PlayKubeOptions) (*entities.PlayKubeReport, error) {
	options := new(kube.PlayOptions).WithAuthfile(opts.Authfile).WithUsername(opts.Username).WithPassword(opts.Password)
	options.WithCertDir(opts.CertDir).WithQuiet(opts.Quiet).WithSignaturePolicy(opts.SignaturePolicy).WithConfigMaps(opts.ConfigMaps)
	options.WithLogDriver(opts.LogDriver).WithNetwork(opts.Networks).WithSeccompProfileRoot(opts.SeccompProfileRoot)
	options.WithStaticIPs(opts.StaticIPs).WithStaticMACs(opts.StaticMACs).WithWait(opts.Wait).WithServiceContainer(opts.ServiceContainer).WithReplace(opts.Replace)
	if len(opts.LogOptions) > 0 {
		options.WithLogOptions(opts.LogOptions)
	}
	if opts.Annotations != nil {
		options.WithAnnotations(opts.Annotations)
	}
	options.WithNoHostname(opts.NoHostname).WithNoHosts(opts.NoHosts).WithUserns(opts.Userns)
	if s := opts.SkipTLSVerify; s != types.OptionalBoolUndefined {
		options.WithSkipTLSVerify(s == types.OptionalBoolTrue)
	}
	if start := opts.Start; start != types.OptionalBoolUndefined {
		options.WithStart(start == types.OptionalBoolTrue)
	}
	options.WithPublishPorts(opts.PublishPorts)
	options.WithPublishAllPorts(opts.PublishAllPorts)
	options.WithNoTrunc(opts.UseLongAnnotations)
	options.WithNoPodPrefix(opts.NoPodPrefix)
	return play.KubeWithBody(ic.ClientCtx, body, options)
}

func (ic *ContainerEngine) PlayKubeDown(_ context.Context, body io.Reader, options entities.PlayKubeDownOptions) (*entities.PlayKubeReport, error) {
	return play.DownWithBody(ic.ClientCtx, body, kube.DownOptions{Force: &options.Force})
}

func (ic *ContainerEngine) KubeApply(_ context.Context, body io.Reader, opts entities.ApplyOptions) error {
	options := new(kube.ApplyOptions).WithKubeconfig(opts.Kubeconfig).WithCACertFile(opts.CACertFile).WithNamespace(opts.Namespace)
	return kube.ApplyWithBody(ic.ClientCtx, body, options)
}
