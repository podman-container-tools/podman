package system

// MemInfo contains memory statistics of the host system.
type MemInfo struct {
	// Total usable RAM (i.e. physical RAM minus a few reserved bits and the
	// kernel binary code).
	MemTotal int64

	// Amount of free memory.
	MemFree int64

	// An estimate of how much memory is available for starting new
	// applications, without swapping.
	// Set to -1 on platforms where this information is not available.
	MemAvailable int64

	// Total amount of swap space available.
	SwapTotal int64

	// Amount of swap space that is currently unused.
	SwapFree int64
}
