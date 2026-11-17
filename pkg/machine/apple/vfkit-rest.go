//go:build darwin

package apple

import (
	"errors"
	"fmt"
	"net/url"
)

// This code is adapted from github.com/crc-org/vfkit/pkg/rest/rest.go as of vkit v0.6.0.
// We don’t want to import that directly because it imports an enormous dependency tree.

// This was taken from  github.com/crc-org/vfkit/pkg/rest.NewEndpoint(input).ToCmdLine()
// and adapted with only the case we use.
func restNewEndpointToCmdLine(input string) ([]string, error) {
	uri, err := url.ParseRequestURI(input)
	if err != nil {
		return nil, err
	}

	switch uri.Scheme {
	case "tcp", "http":
		if len(uri.Host) < 1 {
			return nil, errors.New("invalid TCP uri: missing host")
		}
		if len(uri.Path) > 0 {
			return nil, errors.New("invalid TCP uri: path is forbidden")
		}
		if uri.Port() == "" {
			return nil, errors.New("invalid TCP uri: missing port")
		}
		return []string{"--restful-uri", fmt.Sprintf("tcp://%s%s", uri.Host, uri.Path)}, nil
	default:
		return nil, fmt.Errorf("invalid scheme %s", uri.Scheme)
	}
}
