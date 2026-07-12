package define

type Artifact int64

const (
	Qemu Artifact = iota
	HyperV
	AppleHV
	None
)

func (a Artifact) String() string {
	switch a {
	case HyperV:
		return "hyperv"
	case AppleHV:
		return "applehv"
	}
	return "qemu"
}
