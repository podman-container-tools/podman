package types

import (
	"net"
	"regexp"
)

type Configuration struct {
	// Print packets on stderr
	Debug bool `yaml:"debug,omitempty"`

	// Record all packets coming in and out in a file that can be read by Wireshark (pcap)
	CaptureFile string `yaml:"capture-file,omitempty"`

	// Length of packet
	// Larger packets means less packets to exchange for the same amount of data (and less protocol overhead)
	MTU int `yaml:"mtu,omitempty"`

	// Network reserved for the virtual network
	Subnet string `yaml:"subnet,omitempty"`

	// IP address of the virtual gateway
	GatewayIP string `yaml:"gatewayIP,omitempty"`

	// MAC address of the virtual gateway
	GatewayMacAddress string `yaml:"gatewayMacAddress,omitempty"`

	// Built-in DNS records that will be served by the DNS server embedded in the gateway
	DNS []Zone `yaml:"dns,omitempty"`

	// List of search domains that will be added in all DHCP replies
	DNSSearchDomains []string `yaml:"dnsSearchDomains,omitempty"`

	// Port forwarding between the machine running the gateway and the virtual network.
	Forwards map[string]string `yaml:"forwards,omitempty"`

	// Address translation of incoming traffic.
	// Useful for reaching the host itself (localhost) from the virtual network.
	NAT map[string]string `yaml:"nat,omitempty"`

	// IPs assigned to the gateway that can answer to ARP requests
	GatewayVirtualIPs []string `yaml:"gatewayVirtualIPs,omitempty"`

	// DHCP static leases. Allow to assign pre-defined IP to virtual machine based on the MAC address
	DHCPStaticLeases map[string]string `yaml:"dhcpStaticLeases,omitempty"`

	// Only for Hyperkit
	// Allow to assign a pre-defined MAC address to an Hyperkit VM
	VpnKitUUIDMacAddresses map[string]string `yaml:"vpnKitUUIDMacAddresses,omitempty"`

	// Protocol to be used. Only for /connect mux
	Protocol Protocol `yaml:"-"`

	// EC2 Metadata Service Access
	Ec2MetadataAccess bool `yaml:"ec2MetadataAccess,omitempty"`
}

type Protocol string

const (
	// HyperKitProtocol is handshake, then 16bits little endian size of packet, then the packet.
	HyperKitProtocol Protocol = "hyperkit"
	// QemuProtocol is 32bits big endian size of the packet, then the packet.
	QemuProtocol Protocol = "qemu"
	// BessProtocol transfers bare L2 packets as SOCK_SEQPACKET.
	BessProtocol Protocol = "bess"
	// StdioProtocol is HyperKitProtocol without the handshake
	StdioProtocol Protocol = "stdio"
	// VfkitProtocol transfers bare L2 packets as SOCK_DGRAM.
	VfkitProtocol Protocol = "vfkit"
)

type Zone struct {
	Name      string   `yaml:"name,omitempty"`
	Records   []Record `yaml:"records,omitempty"`
	DefaultIP net.IP   `yaml:"defaultIP,omitempty"`
}

type Record struct {
	Name   string         `yaml:"name,omitempty"`
	IP     net.IP         `yaml:"ip,omitempty"`
	Regexp *regexp.Regexp `json:",omitempty" yaml:"regexp,omitempty"`
}
