package define

type Status = string

// Running indicates the qemu vm is running.
const Running Status = "running"

// Stopped indicates the vm has stopped.
const Stopped Status = "stopped"

// Starting indicated the vm is in the process of starting
const Starting Status = "starting"

// Unknown means the state is not known
const Unknown Status = "unknown"
