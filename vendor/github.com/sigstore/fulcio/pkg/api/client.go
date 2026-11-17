//
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

package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

type CertificateResponse struct {
	CertPEM  []byte
	ChainPEM []byte
	SCT      []byte
}

type RootResponse struct {
	ChainPEM []byte
}

type Key struct {
	// +required
	Content   []byte `json:"content"`
	Algorithm string `json:"algorithm,omitempty"`
}

type CertificateRequest struct {
	// +optional
	PublicKey Key `json:"publicKey"`

	// +optional
	SignedEmailAddress []byte `json:"signedEmailAddress"`

	// +optional
	CertificateSigningRequest []byte `json:"certificateSigningRequest"`
}

const (
	signingCertPath = "/api/v1/signingCert"
	rootCertPath    = "/api/v1/rootCert"
)

// SigstorePublicServerURL is the URL of Sigstore's public Fulcio service.
const SigstorePublicServerURL = "https://fulcio.sigstore.dev"

// LegacyClient is the interface for accessing the Fulcio API.
type LegacyClient interface {
	// SigningCert sends the provided CertificateRequest to the /api/v1/signingCert
	// endpoint of a Fulcio API, authenticated with the provided bearer token.
	SigningCert(cr CertificateRequest, token string) (*CertificateResponse, error)
	// RootCert sends a request to get the current CA used by Fulcio.
	RootCert() (*RootResponse, error)
}

// ClientOption is a functional option for customizing static signatures.
type ClientOption func(*clientOptions)

// NewClient creates a new Fulcio API client talking to the provided URL.
func NewClient(url *url.URL, opts ...ClientOption) LegacyClient {
	o := makeOptions(opts...)

	return &client{
		baseURL: url,
		client: &http.Client{
			Transport: createRoundTripper(http.DefaultTransport, o),
			Timeout:   o.Timeout,
		},
	}
}

type client struct {
	baseURL *url.URL
	client  *http.Client
}

var _ LegacyClient = (*client)(nil)

// SigningCert implements Client
func (c *client) SigningCert(cr CertificateRequest, token string) (*CertificateResponse, error) {
	// Construct the API endpoint for this handler
	endpoint := *c.baseURL
	endpoint.Path = path.Join(endpoint.Path, signingCertPath)

	b, err := json.Marshal(cr)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint.String(), bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	// Set the authorization header to our OIDC bearer token.
	req.Header.Set("Authorization", "Bearer "+token)
	// Set the content-type to reflect we're sending JSON.
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s read: %w", endpoint.String(), err)
	}

	// The API should return a 201 Created on success.  If we see anything else,
	// then turn the response body into an error.
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("%s %s returned %s: %q", http.MethodPost, endpoint.String(), resp.Status, body)
	}

	// Extract the SCT from the response header.
	sct, err := base64.StdEncoding.DecodeString(resp.Header.Get("SCT"))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	// Split the cert and the chain
	certBlock, chainPem := pem.Decode(body)
	if certBlock == nil {
		return nil, errors.New("did not find a cert from Fulcio")
	}
	certPem := pem.EncodeToMemory(certBlock)
	return &CertificateResponse{
		CertPEM:  certPem,
		ChainPEM: chainPem,
		SCT:      sct,
	}, nil
}

func (c *client) RootCert() (*RootResponse, error) {
	// Construct the API endpoint for this handler
	endpoint := *c.baseURL
	endpoint.Path = path.Join(endpoint.Path, rootCertPath)

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(string(body))
	}
	return &RootResponse{ChainPEM: body}, nil
}

type clientOptions struct {
	UserAgent string
	Timeout   time.Duration
}

func makeOptions(opts ...ClientOption) *clientOptions {
	o := &clientOptions{
		UserAgent: "",
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

// WithTimeout sets the request timeout for the client
func WithTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.Timeout = timeout
	}
}

// WithUserAgent sets the media type of the signature.
func WithUserAgent(userAgent string) ClientOption {
	return func(o *clientOptions) {
		o.UserAgent = userAgent
	}
}

type roundTripper struct {
	http.RoundTripper
	UserAgent string
}

// RoundTrip implements `http.RoundTripper`
func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", rt.UserAgent)
	return rt.RoundTripper.RoundTrip(req)
}

func createRoundTripper(inner http.RoundTripper, o *clientOptions) http.RoundTripper {
	if inner == nil {
		inner = http.DefaultTransport
	}
	if o.UserAgent == "" {
		// There's nothing to do...
		return inner
	}
	return &roundTripper{
		RoundTripper: inner,
		UserAgent:    o.UserAgent,
	}
}
