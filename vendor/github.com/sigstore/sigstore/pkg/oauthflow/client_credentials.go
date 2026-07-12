//
// Copyright 2024 The Sigstore Authors.
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

package oauthflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// CodeURL fetches the client credentials token authorization endpoint URL from the provider's well-known configuration endpoint
func (d *DefaultFlowClientCredentials) CodeURL() (string, error) {
	if d.codeURL != "" {
		return d.codeURL, nil
	}

	wellKnown := strings.TrimSuffix(d.Issuer, "/") + "/.well-known/openid-configuration"
	/* #nosec */
	httpClient := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := httpClient.Get(wellKnown)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", resp.Status, body)
	}

	providerConfig := struct {
		Issuer        string `json:"issuer"`
		TokenEndpoint string `json:"token_endpoint"`
	}{}
	if err = json.Unmarshal(body, &providerConfig); err != nil {
		return "", fmt.Errorf("oidc: failed to decode provider discovery object: %w", err)
	}

	if d.Issuer != providerConfig.Issuer {
		return "", fmt.Errorf("oidc: issuer did not match the issuer returned by provider, expected %q got %q", d.Issuer, providerConfig.Issuer)
	}

	if providerConfig.TokenEndpoint == "" {
		return "", fmt.Errorf("oidc: client credentials token authorization endpoint not returned by provider")
	}

	d.codeURL = providerConfig.TokenEndpoint
	return d.codeURL, nil
}

// DefaultFlowClientCredentials fetches an OIDC Identity token using the Client Credentials Grant flow as specified in RFC8628
type DefaultFlowClientCredentials struct {
	Issuer  string
	codeURL string
}

// NewClientCredentialsFlow creates a new DefaultFlowClientCredentials that retrieves an OIDC Identity Token using a Client Credentials Grant
func NewClientCredentialsFlow(issuer string) *DefaultFlowClientCredentials {
	return &DefaultFlowClientCredentials{
		Issuer: issuer,
	}
}

func (d *DefaultFlowClientCredentials) clientCredentialsFlow(_ *oidc.Provider, clientID, clientSecret, redirectURL string) (string, error) {
	data := url.Values{
		"client_id":     []string{clientID},
		"client_secret": []string{clientSecret},
		"scope":         []string{"openid email"},
		"grant_type":    []string{"client_credentials"},
	}
	if redirectURL != "" {
		// If a redirect uri is provided then use it
		data["redirect_uri"] = []string{redirectURL}
	}

	codeURL, err := d.CodeURL()
	if err != nil {
		return "", err
	}
	/* #nosec */
	resp, err := http.PostForm(codeURL, data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", resp.Status, b)
	}

	tr := tokenResp{}
	if err := json.Unmarshal(b, &tr); err != nil {
		return "", err
	}

	if tr.IDToken != "" {
		fmt.Println("Token received!")
		return tr.IDToken, nil
	}

	return "", fmt.Errorf("unexpected error in client flow: %s", tr.Error)
}

// GetIDToken gets an OIDC ID Token from the specified provider using the Client Credentials Grant flow
func (d *DefaultFlowClientCredentials) GetIDToken(p *oidc.Provider, cfg oauth2.Config) (*OIDCIDToken, error) {
	idToken, err := d.clientCredentialsFlow(p, cfg.ClientID, cfg.ClientSecret, cfg.RedirectURL)
	if err != nil {
		return nil, err
	}
	verifier := p.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	parsedIDToken, err := verifier.Verify(context.Background(), idToken)
	if err != nil {
		return nil, err
	}

	subj, err := SubjectFromToken(parsedIDToken)
	if err != nil {
		return nil, err
	}

	return &OIDCIDToken{
		RawString: idToken,
		Subject:   subj,
	}, nil
}
