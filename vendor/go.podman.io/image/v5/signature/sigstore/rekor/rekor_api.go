package rekor

// The following code is the essence of the relevant code paths from github.com/sigstore/rekor/pkg/generated/client/...,
// heavily modified since.

// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// createLogEntryConflictError describes a response with status code 409:
// The request conflicts with the current state of the transparency log.
// This typically happens when trying to upload an existing entry again.
type createLogEntryConflictError struct {
	location string
	err      string
}

func (o *createLogEntryConflictError) Error() string {
	return o.err
}

// createLogEntry creates an entry in the transparency log
//
// Creates an entry in the transparency log for a detached signature, public key, and content. Items can be included in the request or fetched by the server when URLs are specified.
func (r *rekorClient) createLogEntry(ctx context.Context, proposedEntry rekorProposedEntry) (rekorLogEntry, error) {
	res, err := r.makeRequest(ctx, http.MethodPost, "/api/v1/log/entries", proposedEntry)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusCreated:
		result := rekorLogEntry{}
		if err := decodeHTTPResponseBodyAsJSON(res, &result); err != nil {
			return nil, err
		}
		return result, nil

	case http.StatusBadRequest:
		result := rekorError{}
		if err := decodeHTTPResponseBodyAsJSON(res, &result); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Rekor /api/v1/log/entries failed: bad request (%d), %+v", res.StatusCode, result)

	case http.StatusConflict:
		result := rekorError{}
		if err := decodeHTTPResponseBodyAsJSON(res, &result); err != nil {
			return nil, err
		}
		return nil, &createLogEntryConflictError{
			location: res.Header.Get("Location"),
			err:      fmt.Sprintf("Rekor /api/v1/log/entries failed with a conflict (%d), %+v", res.StatusCode, result),
		}

	default:
		result := rekorError{}
		if err := decodeHTTPResponseBodyAsJSON(res, &result); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Rekor /api/v1/log/entries failed with unexpected status %d: %+v", res.StatusCode, result)
	}
}

// getLogEntryByUUID gets log entry and information required to generate an inclusion proof for the entry in the transparency log
//
// Returns the entry, root hash, tree size, and a list of hashes that can be used to calculate proof of an entry being included in the transparency log
func (r *rekorClient) getLogEntryByUUID(ctx context.Context, entryUUID string) (rekorLogEntry, error) {
	res, err := r.makeRequest(ctx, http.MethodGet, "/api/v1/log/entries/"+url.PathEscape(entryUUID), nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		result := rekorLogEntry{}
		if err := decodeHTTPResponseBodyAsJSON(res, &result); err != nil {
			return nil, err
		}
		return result, nil

	case http.StatusNotFound: // We don’t care to define a separate error type; we don’t need it ourselves.
		return nil, fmt.Errorf("Rekor /api/v1/log/entries/{entryUUID}: entry not found (%d)", res.StatusCode)

	default:
		result := rekorError{}
		if err := decodeHTTPResponseBodyAsJSON(res, &result); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Rekor /api/v1/log/entries/{entryUUID} failed with unexpected status %d: %+v", res.StatusCode, result)
	}
}
