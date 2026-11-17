package ginsu

//
// This function is purposely "namespaces" for easy and
// lightweight vendoring.
//

import (
	"bytes"
)

const (
	// KvpValueMaxLen is the maximum real-world length of bytes that can
	// be stored in the value of the wmi key-value pair data exchange
	KvpValueMaxLen = int(990)
)

// Dice takes input and splits it into a string array so it can be
// passed by hyperv and wmi. Each part must be less than the maximum size
// of a kvp value
func Dice(k *bytes.Reader) ([]string, error) {
	var (
		// done is a simple bool indicator that we no longer
		// need to iterate
		done  bool
		parts []string
	)
	for {
		sl := make([]byte, KvpValueMaxLen)
		n, err := k.Read(sl)
		if err != nil {
			return parts, err
		}
		// if we read and the length is less that the max read,
		// then we are at the end
		if n < KvpValueMaxLen {
			sl = sl[0:n]
			done = true
		}
		parts = append(parts, string(sl))
		if done {
			break
		}
	}
	return parts, nil
}
