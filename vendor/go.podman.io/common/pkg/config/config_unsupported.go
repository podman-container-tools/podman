//go:build !linux

package config

// Capabilities returns the capabilities parses the Add and Drop capability
// list from the default capabilities for the container.
func (c *Config) Capabilities(user string, addCapabilities, dropCapabilities []string) ([]string, error) {
	return nil, nil
}
