//go:build linux || freebsd

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:staticcheck // ST1001: should not use dot imports
	. "github.com/onsi/gomega"    //nolint:staticcheck // ST1001: should not use dot imports
)

const (
	contentTypeJSON = "application/json"
)

var searchResults = map[string]any{
	"alpine": map[string]any{
		"query":       "alpine",
		"num_results": 25,
		"num_pages":   2,
		"page":        1,
		"page_size":   25,
		"results": []map[string]any{
			{
				"name":        "cilium/alpine-curl",
				"description": "",
				"is_public":   true,
				"href":        "/repository/cilium/alpine-curl",
			},
			{
				"name":        "libpod/alpine",
				"description": "This image is used for testing purposes only. Do NOT use it in production!",
				"is_public":   true,
				"href":        "/repository/libpod/alpine",
				"stars":       11,
				"official":    true,
			},
			{
				"name":         "openshifttest/alpine",
				"description":  nil,
				"is_public":    true,
				"href":         "/repository/openshifttest/alpine",
				"stars":        5,
				"official":     false,
				"is_automated": true,
			},
			{
				"name":        "openshifttest/base-alpine",
				"description": nil,
				"is_public":   true,
				"href":        "/repository/openshifttest/base-alpine",
			},
			{
				"name":        "astronomer/ap-alpine",
				"description": "",
				"is_public":   true,
				"href":        "/repository/astronomer/ap-alpine",
			},
			{
				"name":        "almworks/alpine-curl",
				"description": "",
				"is_public":   true,
				"href":        "/repository/almworks/alpine-curl",
			},
			{
				"name":        "jitesoft/alpine",
				"description": "# Alpine linux",
				"is_public":   true,
				"href":        "/repository/jitesoft/alpine",
			},
			{
				"name":        "dougbtv/alpine",
				"description": nil,
				"is_public":   true,
				"href":        "/repository/dougbtv/alpine",
			},
			{
				"name":        "tccr/alpine",
				"description": nil,
				"is_public":   true,
				"href":        "/repository/tccr/alpine",
			},
			{
				"name":        "aptible/alpine",
				"description": "Alpine base image, borrowed from gliderlabs/alpine",
				"is_public":   true,
				"href":        "/repository/aptible/alpine",
			},
			{
				"name":        "openshifttest/nginx-alpine",
				"description": nil,
				"is_public":   true,
				"href":        "/repository/openshifttest/nginx-alpine",
			},
			{
				"name":        "wire/alpine-git",
				"description": "",
				"is_public":   true,
				"href":        "/repository/wire/alpine-git",
			},
			{
				"name":        "ditto/alpine-non-root",
				"description": "",
				"is_public":   true,
				"href":        "/repository/ditto/alpine-non-root",
			},
			{
				"name":        "kubevirt/alpine-ext-kernel-boot-demo",
				"description": "",
				"is_public":   true,
				"href":        "/repository/kubevirt/alpine-ext-kernel-boot-demo",
			},
			{
				"name":        "ansible/alpine321-test-container",
				"description": "",
				"is_public":   true,
				"href":        "/repository/ansible/alpine321-test-container",
			},
			{
				"name":        "crio/alpine",
				"description": nil,
				"is_public":   true,
				"href":        "/repository/crio/alpine",
			},
			{
				"name":        "ansible/alpine-test-container",
				"description": "",
				"is_public":   true,
				"href":        "/repository/ansible/alpine-test-container",
			},
			{
				"name":        "ansible/alpine322-test-container",
				"description": "",
				"is_public":   true,
				"href":        "/repository/ansible/alpine322-test-container",
			},
			{
				"name":        "bedrock/alpine",
				"description": "",
				"is_public":   true,
				"href":        "/repository/bedrock/alpine",
			},
			{
				"name":        "ansible/alpine3-test-container",
				"description": "",
				"is_public":   true,
				"href":        "/repository/ansible/alpine3-test-container",
			},
			{
				"name":        "openshift-psap-qe/nginx-alpine",
				"description": nil,
				"is_public":   true,
				"href":        "/repository/openshift-psap-qe/nginx-alpine",
			},
			{
				"name":        "startx/alpine",
				"description": "",
				"is_public":   true,
				"href":        "/repository/startx/alpine",
			},
			{
				"name":        "pcc3202/alpine_multi",
				"description": "",
				"is_public":   true,
				"href":        "/repository/pcc3202/alpine_multi",
			},
			{
				"name":        "nvlab/alpine",
				"description": nil,
				"is_public":   true,
				"href":        "/repository/nvlab/alpine",
			},
			{
				"name":        "kubevirt/alpine-container-disk-demo",
				"description": "Part of kubevirt/kubevirt artifacts",
				"is_public":   true,
				"href":        "/repository/kubevirt/alpine-container-disk-demo",
			},
		},
	},
	"busybox": map[string]any{
		"num_results": 2,
		"query":       "busybox",
		"results": []map[string]any{
			{
				"name":         "busybox",
				"description":  "Busybox base image",
				"star_count":   80,
				"is_official":  true,
				"is_automated": false,
			},
			{
				"name":         "progrium/busybox",
				"description":  "Custom busybox build",
				"star_count":   15,
				"is_official":  false,
				"is_automated": true,
			},
		},
	},
	"skopeo/stable:latest": map[string]any{
		"query":       "skopeo/stable:latest",
		"num_results": 3,
		"num_pages":   1,
		"page":        1,
		"page_size":   25,
		"results": []map[string]any{
			{
				"name":        "skopeo/stable",
				"description": "Stable Skopeo Image",
				"is_public":   true,
				"href":        "/repository/skopeo/stable",
			},
			{
				"name":        "skopeo/testing",
				"description": "Testing Skopeo Image",
				"is_public":   true,
				"href":        "/repository/skopeo/testing",
			},
			{
				"name":        "skopeo/upstream",
				"description": "Upstream Skopeo Image",
				"is_public":   true,
				"href":        "/repository/skopeo/upstream",
			},
		},
	},
	"podman/stable": map[string]any{
		"query":       "podman/stable",
		"num_results": 3,
		"num_pages":   1,
		"page":        1,
		"page_size":   25,
		"results": []map[string]any{
			{
				"name":        "podman/stable",
				"description": "Stable Podman Image",
				"is_public":   true,
				"href":        "/repository/podman/stable",
			},
			{
				"name":        "podman/testing",
				"description": "Testing Podman Image",
				"is_public":   true,
				"href":        "/repository/podman/testing",
			},
			{
				"name":        "podman/upstream",
				"description": "Upstream Podman Image",
				"is_public":   true,
				"href":        "/repository/podman/upstream",
			},
		},
	},
	"testdigest_v2s1": map[string]any{
		"query":       "testdigest_v2s1",
		"num_results": 2,
		"num_pages":   1,
		"page":        1,
		"page_size":   25,
		"results": []map[string]any{
			{
				"name":        "libpod/testdigest_v2s1",
				"description": "Test image used by buildah regression tests",
				"is_public":   true,
				"href":        "/repository/libpod/testdigest_v2s1",
			},
			{
				"name":        "libpod/testdigest_v2s1_with_dups",
				"description": "This is a specially crafted test-only image used in buildah CI and gating tests.",
				"is_public":   true,
				"href":        "/repository/libpod/testdigest_v2s1_with_dups",
			},
		},
	},
	"testdigest_v2s2": map[string]any{
		"query":       "testdigest_v2s2",
		"num_results": 1,
		"num_pages":   1,
		"page":        1,
		"page_size":   25,
		"results": []map[string]any{
			{
				"name":        "libpod/testdigest_v2s2",
				"description": "This is a specially crafted test-only image used in buildah CI and gating tests.",
				"is_public":   true,
				"href":        "/repository/libpod/testdigest_v2s2",
			},
		},
	},
}

// Mock repository tag data - simplified to just store tag lists
var mockRepoTags = map[string][]string{
	"libpod/alpine": {"3.10.2", "3.2", "latest", "withbogusseccomp", "withseccomp"},
	"podman/stable": {
		"latest", "v1.4.2", "v1.4.4", "v1.5.0", "v1.5.1", "v1.6", "v1.6.2",
		"v1.9.0", "v1.9.1", "v2.0.2", "v2.0.6", "v2.1.1", "v2.2.1", "v3",
		"v3.1.2", "v3.2.0", "v3.2.1", "v3.2.2", "v3.2.3", "v3.3.0", "v3.3.1",
		"v3.4", "v3.4.0", "v3.4.1", "v3.4.2", "v3.4.4", "v3.4.7", "v4",
		"v4.1", "v4.1.0", "v4.1.1", "v4.2", "v4.2.0", "v4.2.1", "v4.3",
		"v4.3.0", "v4.3.1", "v4.4", "v4.4.1", "v4.4.2", "v4.4.4", "v4.5",
		"v4.5.0", "v4.5.1", "v4.6", "v4.6.1", "v4.6.2", "v4.7", "v4.7.0",
		"v4.7.2", "v4.8", "v4.8.0", "v4.8.1", "v4.8.2", "v4.8.3", "v4.9",
		"v4.9.0", "v4.9.3", "v4.9.4", "v4.9.4-immutable", "v4.9-immutable",
		"v4-immutable", "v5", "v5.0", "v5.0.1", "v5.0.1-immutable", "v5.0.2",
		"v5.0.2-immutable", "v5.0.3", "v5.0.3-immutable", "v5.0-immutable",
		"v5.1", "v5.1.0", "v5.1.0-immutable", "v5.1.1", "v5.1.1-immutable",
		"v5.1.2", "v5.1.2-immutable", "v5.1-immutable", "v5.2", "v5.2.0",
		"v5.2.0-immutable", "v5.2.1", "v5.2.1-immutable", "v5.2.2",
		"v5.2.2-immutable", "v5.2.3", "v5.2.3-immutable", "v5.2.5",
		"v5.2.5-immutable", "v5.2-immutable", "v5.3", "v5.3.0",
		"v5.3.0-immutable", "v5.3.1", "v5.3.1-immutable", "v5.3.2",
		"v5.3.2-immutable", "v5.3-immutable", "v5.4",
	},
}

// Pagination tags for podman/stable (returned after v5.4 in pagination requests)
// This simulates the specific test case where limit=100 and last=v5.4
var podmanStablePaginatedTags = []string{
	"v5.4.0", "v5.4.0-immutable", "v5.4.1", "v5.4.1-immutable", "v5.4.2",
	"v5.4.2-immutable", "v5.4-immutable", "v5.5", "v5.5.0", "v5.5.0-immutable",
	"v5.5.1", "v5.5.1-immutable", "v5.5.2", "v5.5.2-immutable", "v5.5-immutable",
	"v5.6", "v5.6.0", "v5.6.0-immutable", "v5.6.1", "v5.6.1-immutable", "v5.6.2",
	"v5.6.2-immutable", "v5.6-immutable", "v5-immutable",
}

func writeJSONResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func handleV1Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if decodedQuery, err := url.QueryUnescape(query); err == nil {
		query = decodedQuery
	}

	limitStr := r.URL.Query().Get("n")
	limitNum := -1
	if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
		limitNum = limit
	} else if err != nil {
		http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
		return
	}

	results := searchForResults(query)

	if results != nil {
		response := applyLimitToResults(results, limitNum)
		writeJSONResponse(w, response)
	} else {
		defaultResponse := map[string]any{
			"num_results": 0,
			"query":       query,
			"results":     []any{},
		}
		writeJSONResponse(w, defaultResponse)
	}
}

func searchForResults(query string) map[string]any {
	regexPattern := query
	if strings.Contains(query, "*") {
		regexPattern = strings.ReplaceAll(query, "*", ".*")
	}

	for key, value := range searchResults {
		match, _ := regexp.MatchString(regexPattern, key)
		if match {
			return value.(map[string]any)
		}
	}
	return nil
}

func applyLimitToResults(results map[string]any, limitNum int) map[string]any {
	originalBytes, err := json.Marshal(results)
	if err != nil {
		return results
	}
	var resultsCopy map[string]any
	if err := json.Unmarshal(originalBytes, &resultsCopy); err != nil {
		return results
	}

	if limitNum > 0 {
		if resultsArray, ok := resultsCopy["results"].([]any); ok {
			actualLimit := min(len(resultsArray), limitNum)
			resultsCopy["results"] = resultsArray[:actualLimit]
			resultsCopy["num_results"] = actualLimit
		}
	}
	return resultsCopy
}

func handleV2(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/v2/_catalog" {
		handleCatalog(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/tags/list") {
		handleTagsList(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))
}

func parseRepositoryPath(path string) (string, bool) {
	pathParts := strings.Split(strings.TrimPrefix(path, "/v2/"), "/")
	if len(pathParts) < 2 {
		return "", false
	}

	if pathParts[len(pathParts)-1] == "list" && pathParts[len(pathParts)-2] == "tags" {
		repoName := strings.Join(pathParts[:len(pathParts)-2], "/")
		return repoName, true
	}
	return "", false
}

func handleTagsList(w http.ResponseWriter, r *http.Request) {
	repoName, isValidPath := parseRepositoryPath(r.URL.Path)
	if !isValidPath {
		http.Error(w, "Invalid tags list path", http.StatusBadRequest)
		return
	}

	allTags, exists := mockRepoTags[repoName]
	if !exists {
		http.Error(w, fmt.Sprintf("repository %s not found", repoName), http.StatusNotFound)
		return
	}
	query := r.URL.Query()
	limit := -1
	last := query.Get("last")

	if limitStr := query.Get("n"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	paginatedTags := applyPagination(allTags, limit, last, repoName)

	response := map[string]any{
		"name": repoName,
		"tags": paginatedTags,
	}
	writeJSONResponse(w, response)
}

func applyPagination(allTags []string, limit int, last string, repoName string) []string {
	if repoName == "podman/stable" && limit == 100 && last == "v5.4" {
		return podmanStablePaginatedTags
	}

	if limit <= 0 && last == "" {
		return allTags
	}

	startIndex := 0

	if last != "" {
		for i, tag := range allTags {
			if tag == last {
				startIndex = i + 1
				break
			}
		}
	}

	if limit > 0 {
		endIndex := min(startIndex+limit, len(allTags))
		return allTags[startIndex:endIndex]
	}

	return allTags[startIndex:]
}

func handleCatalog(w http.ResponseWriter, _ *http.Request) {
	repositories := make([]string, 0, len(mockRepoTags))
	for repoName := range mockRepoTags {
		repositories = append(repositories, repoName)
	}

	response := map[string]any{
		"repositories": repositories,
	}
	writeJSONResponse(w, response)
}

// CreateMockRegistryServer creates and starts a mock Docker registry server
// Returns: server address, server instance, error channel, and logged requests slice
func CreateMockRegistryServer() (string, *http.Server, chan error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	Expect(err).ToNot(HaveOccurred())
	serverAddr := listener.Addr().String()

	mux := http.NewServeMux()

	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		handleV1Search(w, r)
	})

	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		handleV2(w, r)
	})

	srv := &http.Server{
		Handler:  mux,
		ErrorLog: log.New(io.Discard, "", 0),
	}

	serverErr := make(chan error, 1)
	go func() {
		defer GinkgoRecover()
		serverErr <- srv.Serve(listener)
	}()

	Eventually(func() error {
		resp, err := http.Get("http://" + serverAddr + "/v2/")
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server not ready, status: %d", resp.StatusCode)
		}
		return nil
	}, "5s", "100ms").Should(Succeed())

	return serverAddr, srv, serverErr
}

func CloseMockRegistryServer(srv *http.Server, serverErr chan error) {
	srv.Close()
	Expect(<-serverErr).To(Equal(http.ErrServerClosed))
}
