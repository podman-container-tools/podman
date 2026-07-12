package kube

import (
	"net"
)

// PlayOptions are optional options for replaying kube YAML files
//
//go:generate go run ../generator/generator.go PlayOptions
type PlayOptions struct {
	// Annotations - Annotations to add to Pods
	Annotations map[string]string
	// Authfile - path to an authentication file.
	Authfile *string
	// CertDir - to a directory containing TLS certifications and keys.
	CertDir *string
	// Username for authenticating against the registry.
	Username *string
	// Password for authenticating against the registry.
	Password *string
	// Network - name of the networks to connect to.
	Network *[]string
	// NoHostname - do not generate /etc/hostname file in pod's containers
	NoHostname *bool
	// NoHosts - do not generate /etc/hosts file in pod's containers
	NoHosts *bool
	// Quiet - suppress output when pulling images.
	Quiet *bool
	// SignaturePolicy - path to a signature-policy file.
	SignaturePolicy *string
	// SkipTLSVerify - skip https and certificate validation when
	// contacting container registries.
	SkipTLSVerify *bool `schema:"-"`
	// SeccompProfileRoot - path to a directory containing seccomp
	// profiles.
	SeccompProfileRoot *string
	// StaticIPs - Static IP address used by the pod(s).
	StaticIPs *[]net.IP
	// StaticMACs - Static MAC address used by the pod(s).
	StaticMACs *[]net.HardwareAddr
	// ConfigMaps - slice of pathnames to kubernetes configmap YAMLs.
	ConfigMaps *[]string
	// LogDriver for the container. For example: journald
	LogDriver *string
	// LogOptions for the container. For example: journald
	LogOptions *[]string
	// Replace - replace existing pods and containers
	Replace *bool
	// Start - don't start the pod if false
	Start *bool
	// NoTrunc - use annotations that were not truncated to the
	// Kubernetes maximum of 63 characters
	NoTrunc *bool
	// Userns - define the user namespace to use.
	Userns *string
	// Force - remove volumes on --down
	Force *bool
	// PublishPorts - configure how to expose ports configured inside the K8S YAML file
	PublishPorts []string
	// PublishAllPorts - whether to publish all ports defined in the K8S YAML file
	// (containerPort, hostPort) otherwise only hostPort will be published
	PublishAllPorts *bool
	// Wait - indicates whether to return after having created the pods
	Wait             *bool
	ServiceContainer *bool
	NoPodPrefix      *bool
}

// ApplyOptions are optional options for applying kube YAML files to a k8s cluster
//
//go:generate go run ../generator/generator.go ApplyOptions
type ApplyOptions struct {
	// Kubeconfig - path to the cluster's kubeconfig file.
	Kubeconfig *string
	// Namespace - namespace to deploy the workload in on the cluster.
	Namespace *string
	// CACertFile - the path to the CA cert file for the Kubernetes cluster.
	CACertFile *string
	// File - the path to the Kubernetes yaml to deploy.
	File *string
	// Service - creates a service for the container being deployed.
	Service *bool
}

// DownOptions are optional options for tearing down kube YAML files to a k8s cluster
//
//go:generate go run ../generator/generator.go DownOptions
type DownOptions struct {
	// Force - remove volumes on --down
	Force *bool
}
