//go:build !remote && (linux || freebsd)

package generate

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/pkg/specgen"
)

var (
	portMappings = []types.PortMapping{{HostPort: 443, ContainerPort: 8080, Protocol: protoUDP}, {HostPort: 22, ContainerPort: 2222, Protocol: protoTCP}}
	networks     = map[string]types.PerNetworkOptions{"test": {
		StaticIPs:     nil,
		Aliases:       nil,
		StaticMAC:     nil,
		InterfaceName: "eth2",
	}}
)

func TestMapSpecCopyPodSpecToInfraContainerSpec(t *testing.T) {
	infraCommand := []string{"top"}
	addedHosts := []string{"otherhost"}
	dnsServers := []net.IP{net.IPv4(10, 0, 0, 1), net.IPv4(8, 8, 8, 8)}
	dnsOptions := []string{"dns option"}
	dnsSearch := []string{"dns search"}
	infraImage := "someimage"
	conmonPidFile := "/var/run/conmon.pid"
	podSpec := specgen.PodSpecGenerator{
		PodBasicConfig: specgen.PodBasicConfig{
			InfraCommand: infraCommand, InfraImage: infraImage,
			InfraConmonPidFile: conmonPidFile,
		},
		PodNetworkConfig: specgen.PodNetworkConfig{
			PortMappings: portMappings, HostAdd: addedHosts, DNSServer: dnsServers, DNSOption: dnsOptions, DNSSearch: dnsSearch,
			Networks: networks, NoManageResolvConf: true, NoManageHosts: true,
		},
		PodCgroupConfig:    specgen.PodCgroupConfig{},
		PodResourceConfig:  specgen.PodResourceConfig{},
		PodStorageConfig:   specgen.PodStorageConfig{},
		PodSecurityConfig:  specgen.PodSecurityConfig{},
		InfraContainerSpec: &specgen.SpecGenerator{},
		ServiceContainerID: "",
	}

	mappedSpec, err := MapSpec(&podSpec)

	assert.NoError(t, err)

	assert.Equal(t, portMappings, mappedSpec.PortMappings)
	assert.Equal(t, infraCommand, mappedSpec.Entrypoint)
	assert.Equal(t, addedHosts, mappedSpec.HostAdd)
	assert.Equal(t, dnsServers, mappedSpec.DNSServers)
	assert.Equal(t, dnsOptions, mappedSpec.DNSOptions)
	assert.Equal(t, dnsSearch, mappedSpec.DNSSearch)
	require.NotNil(t, mappedSpec.UseImageResolvConf)
	assert.True(t, *mappedSpec.UseImageResolvConf)
	assert.Equal(t, networks, mappedSpec.Networks)
	require.NotNil(t, mappedSpec.UseImageHosts)
	assert.True(t, *mappedSpec.UseImageHosts)
	assert.Equal(t, conmonPidFile, mappedSpec.ConmonPidFile)
	assert.Equal(t, infraImage, mappedSpec.Image)
}

func createPodSpec(mode specgen.NamespaceMode) specgen.PodSpecGenerator {
	return specgen.PodSpecGenerator{
		InfraContainerSpec: &specgen.SpecGenerator{},
		PodNetworkConfig: specgen.PodNetworkConfig{
			NetNS: specgen.Namespace{NSMode: mode},
		},
	}
}

func createPodSpecWithNetworks(mode specgen.NamespaceMode) specgen.PodSpecGenerator {
	spec := createPodSpec(mode)
	spec.InfraContainerSpec.Networks = networks
	return spec
}

func createPodSpecWithPortMapping(mode specgen.NamespaceMode) specgen.PodSpecGenerator {
	spec := createPodSpec(mode)
	spec.InfraContainerSpec.PortMappings = portMappings
	return spec
}

func createPodSpecWithNetNsPath(path string) specgen.PodSpecGenerator {
	spec := createPodSpec(specgen.Path)
	spec.NetNS.Value = path
	return spec
}

func TestMapSpecNetworkOptions(t *testing.T) {
	tests := []struct {
		name                   string
		podSpec                specgen.PodSpecGenerator
		expectedNSMode         specgen.NamespaceMode
		expectedNSValue        string
		expectedNetworkOptions map[string][]string
		mustError              bool
	}{
		{
			name:           "Default",
			podSpec:        createPodSpec(specgen.Default),
			expectedNSMode: "",
		},
		{
			name:           "Bridge",
			podSpec:        createPodSpec(specgen.Bridge),
			expectedNSMode: specgen.Bridge,
		},
		{
			name:           "Private",
			podSpec:        createPodSpec(specgen.Private),
			expectedNSMode: specgen.Private,
		},
		{
			name:           "Host",
			podSpec:        createPodSpec(specgen.Host),
			expectedNSMode: specgen.Host,
		},
		{
			name:      "Host but with port mappings",
			podSpec:   createPodSpecWithPortMapping(specgen.Host),
			mustError: true,
		},
		{
			name:      "Host but with networks",
			podSpec:   createPodSpecWithNetworks(specgen.Host),
			mustError: true,
		},
		{
			name:            "Path",
			podSpec:         createPodSpecWithNetNsPath("/var/run/netns/bla"),
			expectedNSMode:  specgen.Path,
			expectedNSValue: "/var/run/netns/bla",
		},
		{
			name:           "NoNetwork",
			podSpec:        createPodSpec(specgen.NoNetwork),
			expectedNSMode: specgen.NoNetwork,
		},
		{
			name:      "NoNetwork but with networks",
			podSpec:   createPodSpecWithNetworks(specgen.NoNetwork),
			mustError: true,
		},
		{
			name:      "NoNetwork but with port mappings",
			podSpec:   createPodSpecWithPortMapping(specgen.NoNetwork),
			mustError: true,
		},
		{
			name:      "FromContainer",
			podSpec:   createPodSpec(specgen.FromContainer),
			mustError: true,
		},
		{
			name:      "FromPod",
			podSpec:   createPodSpec(specgen.FromPod),
			mustError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappedSpec, err := MapSpec(&tt.podSpec)

			if tt.mustError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err, "error is not nil")
				assert.Equal(t, tt.expectedNSMode, mappedSpec.NetNS.NSMode)
				assert.Equal(t, tt.expectedNSValue, mappedSpec.NetNS.Value)
				assert.Equal(t, tt.expectedNetworkOptions, mappedSpec.NetworkOptions)
			}
		})
	}
}
