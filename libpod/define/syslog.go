package define

import (
	"fmt"
	"strings"
)

// NormalizeSyslogOptionKeys rewrites Syslog-* keys to the lowercase names conmon
// accepts. Other keys are left unchanged. Safe to call with a nil map.
// Keys that differ only by case are rejected, including when the values match,
// so the result does not depend on map iteration order.
func NormalizeSyslogOptionKeys(opts map[string]string) error {
	seen := make(map[string]struct{}, len(opts))
	for key := range opts {
		lower := strings.ToLower(key)
		if !strings.HasPrefix(lower, "syslog-") {
			continue
		}
		if _, ok := seen[lower]; ok {
			return fmt.Errorf("duplicate log opt %q for syslog log driver: %w", lower, ErrInvalidArg)
		}
		seen[lower] = struct{}{}
	}
	for key, value := range opts {
		lower := strings.ToLower(key)
		if lower != key && strings.HasPrefix(lower, "syslog-") {
			delete(opts, key)
			opts[lower] = value
		}
	}
	return nil
}
