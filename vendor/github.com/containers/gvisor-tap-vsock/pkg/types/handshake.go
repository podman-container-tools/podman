package types

type TransportProtocol string

const (
	UDP   TransportProtocol = "udp"
	TCP   TransportProtocol = "tcp"
	UNIX  TransportProtocol = "unix"
	NPIPE TransportProtocol = "npipe"
)

type ExposeRequest struct {
	Local    string            `json:"local"`
	Remote   string            `json:"remote"`
	Protocol TransportProtocol `json:"protocol"`
}

type UnexposeRequest struct {
	Local    string            `json:"local"`
	Protocol TransportProtocol `json:"protocol"`
}

type NotificationMessage struct {
	NotificationType NotificationType `json:"notification_type"`
	MacAddress       string           `json:"mac_address,omitempty"`
}

type NotificationType string

const (
	Ready                 NotificationType = "ready"
	ConnectionEstablished NotificationType = "connection_established"
	HypervisorError       NotificationType = "hypervisor_error"
	ConnectionClosed      NotificationType = "connection_closed"
)
