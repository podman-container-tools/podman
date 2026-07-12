package util

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// filtersFromRequests extracts the "filters" parameter from the specified
// http.Request.  The parameter can either be a `map[string][]string` as done
// in new versions of Docker and libpod, or a `map[string]map[string]bool` as
// done in older versions of Docker.  We have to do a bit of Yoga to support
// both - just as Docker does as well.
//
// Please refer to https://github.com/containers/podman/issues/6899 for some
// background.
func FiltersFromRequest(r *http.Request) ([]string, error) {
	var (
		compatFilters map[string]map[string]bool
		filters       map[string][]string
		libpodFilters []string
		raw           []byte
	)

	if _, found := r.URL.Query()["filters"]; found {
		raw = []byte(r.Form.Get("filters"))
	} else if _, found := r.URL.Query()["Filters"]; found {
		raw = []byte(r.Form.Get("Filters"))
	} else {
		return []string{}, nil
	}

	// Backwards compat with older versions of Docker.
	if err := json.Unmarshal(raw, &compatFilters); err == nil {
		for filterKey, filterMap := range compatFilters {
			for filterValue, toAdd := range filterMap {
				if toAdd {
					libpodFilters = append(libpodFilters, fmt.Sprintf("%s=%s", filterKey, filterValue))
				}
			}
		}
		return libpodFilters, nil
	}

	if err := json.Unmarshal(raw, &filters); err != nil {
		return nil, err
	}

	for filterKey, filterSlice := range filters {
		for _, filterValue := range filterSlice {
			libpodFilters = append(libpodFilters, fmt.Sprintf("%s=%s", filterKey, filterValue))
		}
	}

	return libpodFilters, nil
}

// NormalizeVolumePruneFilters mutates f in place (non-empty maps); if f is empty/nil it
// returns a new map—always use the return value. If "all" is true/1, all keys are removed;
// else "all" is deleted and anonymous=true is set only when no other keys remain.
func NormalizeVolumePruneFilters(f url.Values) url.Values {
	if len(f) == 0 {
		f = make(url.Values)
	}
	allValue := f.Get("all")
	if strings.EqualFold(allValue, "true") || allValue == "1" {
		// "all" only widens the scope from anonymous-only to every unused
		// volume; it is orthogonal to the other filters. Drop just the "all"
		// key and keep the rest (e.g. label/label!, until) so they continue to
		// constrain which volumes are pruned, matching Docker, which ANDs the
		// "all" scope together with the label filters.
		f.Del("all")
		return f
	}
	f.Del("all")

	if len(f) > 0 {
		return f
	}
	f.Set("anonymous", "true")
	return f
}

// PrepareFilters prepares a *map[string][]string of filters to be later searched
// in lipod and compat API to get desired filters
func PrepareFilters(r *http.Request) (*map[string][]string, error) {
	filtersList, err := FiltersFromRequest(r)
	if err != nil {
		return nil, err
	}
	filterMap := map[string][]string{}
	for _, filter := range filtersList {
		fname, filter, hasFilter := strings.Cut(filter, "=")
		if hasFilter {
			filterMap[fname] = append(filterMap[fname], filter)
		}
	}
	return &filterMap, nil
}
