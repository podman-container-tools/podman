package specgenutil

// PidsLimitForOCI returns the value to store in the OCI runtime spec
// linux.resources.pids.limit field.
//
// Docker documents HostConfig.PidsLimit 0 as unlimited. runc maps OCI limit 0 to
// cgroup pids.max=1, while -1 means unlimited. crun treats 0 as unlimited.
// Normalize 0 to -1 so all runtimes get unlimited pids.
func PidsLimitForOCI(limit int64) int64 {
	if limit == 0 {
		return -1
	}
	return limit
}
