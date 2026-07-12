package define

// VMState can be used to describe the current state of a VM
// as well as used to request a state change
type VMState struct {
	State string `json:"state"`
}

type StateChange string

const (
	Resume   StateChange = "Resume"
	Pause    StateChange = "Pause"
	Stop     StateChange = "Stop"
	HardStop StateChange = "HardStop"
)
