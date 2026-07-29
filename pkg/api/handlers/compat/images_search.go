//go:build !remote && (linux || freebsd)

package compat

import (
	"fmt"
	"net/http"

	"github.com/moby/moby/api/types/registry"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	"go.podman.io/podman/v6/pkg/api/handlers/utils/apiutil"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/auth"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/storage"
)

func SearchImages(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)
	query := struct {
		Term      string              `json:"term"`
		Limit     int                 `json:"limit"`
		Filters   map[string][]string `json:"filters"`
		TLSVerify bool                `json:"tlsVerify"`
		ListTags  bool                `json:"listTags"`
	}{
		// This is where you can override the golang default value for one of fields
		TLSVerify: true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	authconf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	var username, password, idToken string
	if authconf != nil {
		username = authconf.Username
		password = authconf.Password
		idToken = authconf.IdentityToken
	}
	// compat v1.45 deprecation: searching for is-automated=true will yield no results, while is-automated=false will be a no-op.
	isAutomatedDeprecated := false
	if _, err := apiutil.SupportedVersion(r, ">=1.45.0"); err == nil {
		if !utils.IsLibpodRequest(r) {
			isAutomatedDeprecated = true
			if vals, ok := query.Filters["is-automated"]; ok {
				switch vals[0] {
				case "true":
					utils.WriteResponse(w, http.StatusOK, []registry.SearchResult{})
					return
				case "false":
					delete(query.Filters, "is-automated")
				}
			}
		}
	}

	filters := []string{}
	for key, val := range query.Filters {
		filters = append(filters, fmt.Sprintf("%s=%s", key, val[0]))
	}

	options := entities.ImageSearchOptions{
		Authfile:      authfile,
		Limit:         query.Limit,
		ListTags:      query.ListTags,
		Password:      password,
		Username:      username,
		IdentityToken: idToken,
		Filters:       filters,
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}
	ir := abi.ImageEngine{Libpod: runtime}
	reports, err := ir.Search(r.Context(), query.Term, options)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !utils.IsLibpodRequest(r) {
		if len(reports) == 0 {
			utils.ImageNotFound(w, query.Term, storage.ErrImageUnknown)
			return
		}
		compatResults := make([]registry.SearchResult, len(reports))
		for i, report := range reports {
			result := registry.SearchResult{
				Name:        report.Name,
				Description: report.Description,
				StarCount:   report.Stars,
				IsOfficial:  toBool(report.Official),
				IsAutomated: toBool(report.Automated),
			}
			if isAutomatedDeprecated {
				//nolint:staticcheck
				result.IsAutomated = false
			}
			compatResults[i] = result
		}
		utils.WriteResponse(w, http.StatusOK, compatResults)
		return
	}

	utils.WriteResponse(w, http.StatusOK, reports)
}

// toBool converts the string representation
// of ImageSearchReport's Automated and Official fields
// to bool that the Docker representation uses.
func toBool(s string) bool {
	return s == entities.ImageSearchTrue
}
