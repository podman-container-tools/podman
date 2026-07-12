package rekor

// The following code is the essence of the relevant code paths from github.com/go-openapi/runtime,
// heavily modified since.

// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path"
)

// makeRequest makes a http request to the requested requestPath, and returns the received response.
func (r *rekorClient) makeRequest(ctx context.Context, method, requestPath string, bodyContent any) (*http.Response, error) {
	var body io.Reader
	headers := http.Header{}

	headers.Set("Accept", "application/json")
	if bodyContent != nil {
		buf := bytes.NewBuffer(nil)
		body = buf
		headers.Set("Content-Type", "application/json")
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(bodyContent); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, path.Join(r.basePath, requestPath), body)
	if err != nil {
		return nil, err
	}
	// Only Scheme and Host are used from rekorURL.
	// Really this should probabbly use r.rekorURL.JoinPath(requestPath) (which, notably, correctly deals with path escaping),
	// and pass that to NewRequestWithContext, but this use of path.Join is consistent with go-openapi/runtime v0.24.1 .
	req.URL.Scheme = r.rekorURL.Scheme
	req.URL.Host = r.rekorURL.Host
	req.Header = headers

	res, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	// Note that we don’t care to even read the Content-Type: header; we blindly assume the format is the requested JSON.
	return res, nil
}

// decodeHTTPResponseBodyAsJSON decodes the body of a HTTP response in a manner compatible with github.com/go-openapi/runtime.
func decodeHTTPResponseBodyAsJSON(res *http.Response, data any) error {
	dec := json.NewDecoder(res.Body)
	dec.UseNumber()
	err := dec.Decode(data)
	if err == io.EOF {
		// This seems unwanted at a first glance; go-swagger added it in https://github.com/go-swagger/go-swagger/issues/192 , it’s unclear
		// whether it’s correct or still necessary.
		err = nil
	}
	return err
}
