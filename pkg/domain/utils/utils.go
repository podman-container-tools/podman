package utils

import (
	"fmt"
	"net/url"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// DeepCopy does a deep copy of a structure
// Error checking of parameters delegated to json engine
var DeepCopy = func(dst any, src any) error {
	payload, err := json.Marshal(src)
	if err != nil {
		return err
	}

	err = json.Unmarshal(payload, dst)
	if err != nil {
		return err
	}
	return nil
}

func ToLibpodFilters(f url.Values) (filters []string) {
	for k, v := range f {
		filters = append(filters, k+"="+v[0])
	}
	return filters
}

func ToURLValues(f []string) (filters url.Values) {
	filters = make(url.Values)
	for _, v := range f {
		key, val, _ := strings.Cut(v, "=")
		filters.Add(key, val)
	}
	return filters
}

// ParseAnnotations takes a string slice of options, expected to be "key=val" and returns
// a string map where the map index is the key and points to the value
func ParseAnnotations(options []string) (map[string]string, error) {
	annotations := make(map[string]string)
	for _, annotationSpec := range options {
		key, val, hasVal := strings.Cut(annotationSpec, "=")
		if !hasVal {
			return nil, fmt.Errorf("no value given for annotation %q", key)
		}
		annotations[key] = val
	}
	return annotations, nil
}
