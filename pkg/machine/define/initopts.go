package define

import (
	"net/url"

	"go.podman.io/image/v5/types"
)

type InitOptions struct {
	PlaybookPath       string
	CPUS               uint64
	DiskSize           uint64
	IgnitionPath       string
	Image              string
	Volumes            []string
	IsDefault          bool
	Memory             uint64
	Swap               uint64
	Name               string
	TimeZone           string
	URI                url.URL
	Username           string
	ReExec             bool
	Rootful            bool
	UID                string // uid of the user that called machine
	UserModeNetworking *bool  // nil = use backend/system default, false = disable, true = enable
	USBs               []string
	SkipTlsVerify      types.OptionalBool
	ImportNativeCA     bool
}
