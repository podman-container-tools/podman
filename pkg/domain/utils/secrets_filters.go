package utils

import (
	"fmt"
	"strings"

	"go.podman.io/common/pkg/secrets"
	"go.podman.io/podman/v6/pkg/util"
)

func IfPassesSecretsFilter(s secrets.Secret, filters map[string][]string) (bool, error) {
	result := true
	for key, filterValues := range filters {
		switch strings.ToLower(key) {
		case "name":
			result = util.StringMatchRegexSlice(s.Name, filterValues)
		case "id":
			result = util.StringMatchRegexSlice(s.ID, filterValues)
		default:
			return false, fmt.Errorf("invalid filter %q", key)
		}
	}
	return result, nil
}
