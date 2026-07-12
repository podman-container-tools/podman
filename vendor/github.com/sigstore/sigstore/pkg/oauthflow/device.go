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

// Package oauthflow implements OAuth/OIDC support for device and token flows
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

const (
	// SigstoreDeviceURL specifies the Device Code endpoint for the public good Sigstore service
	//
	// Deprecated: this constant (while correct) should not be used
	/* #nosec */
	SigstoreDeviceURL = "https://oauth2.sigstore.dev/auth/device/code"
	// SigstoreTokenURL specifies the Token endpoint for the public good Sigstore service
	//
	// Deprecated: this constant (while correct) should not be used
	/* #nosec */
	SigstoreTokenURL = "https://oauth2.sigstore.dev/auth/device/token"
)

type deviceResp struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval"`
	ExpiresIn               int    `json:"expires_in"`
}

type tokenResp struct {
	IDToken string `json:"id_token"`
	Error   string `json:"error"`
}

// DeviceFlowTokenGetter fetches an OIDC Identity token using the Device Code Grant flow as specified in RFC8628
type DeviceFlowTokenGetter struct {
	MessagePrinter func(string)
	Sleeper        func(time.Duration)
	Issuer         string
	codeURL        string
}

// NewDeviceFlowTokenGetter creates a new DeviceFlowTokenGetter that retrieves an OIDC Identity Token using a Device Code Grant
//
// Deprecated: NewDeviceFlowTokenGetter is deprecated; use NewDeviceFlowTokenGetterForIssuer() instead
func NewDeviceFlowTokenGetter(issuer, codeURL, _ string) *DeviceFlowTokenGetter {
	return &DeviceFlowTokenGetter{
		MessagePrinter: func(s string) { fmt.Println(s) },
		Sleeper:        time.Sleep,
		Issuer:         issuer,
		codeURL:        codeURL,
	}
}

// NewDeviceFlowTokenGetterForIssuer creates a new DeviceFlowTokenGetter that retrieves an OIDC Identity Token using a Device Code Grant
func NewDeviceFlowTokenGetterForIssuer(issuer string) *DeviceFlowTokenGetter {
	return &DeviceFlowTokenGetter{
		MessagePrinter: func(s string) { fmt.Println(s) },
		Sleeper:        time.Sleep,
		Issuer:         issuer,
	}
}

func (d *DeviceFlowTokenGetter) deviceFlow(p *oidc.Provider, clientID, redirectURL string) (string, error) {
	// require that OIDC provider support PKCE to provide sufficient security for the CLI
	pkce, err := NewPKCE(p)
	if err != nil {
		return "", err
	}

	data := url.Values{
		"client_id":             []string{clientID},
		"scope":                 []string{"openid email"},
		"code_challenge_method": []string{pkce.Method},
		"code_challenge":        []string{pkce.Challenge},
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

	parsed := deviceResp{}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return "", err
	}
	uri := parsed.VerificationURIComplete
	if uri == "" {
		uri = parsed.VerificationURI
	}
	d.MessagePrinter(fmt.Sprintf("Enter the verification code %s in your browser at: %s", parsed.UserCode, uri))
	d.MessagePrinter(fmt.Sprintf("Code will be valid for %d seconds", parsed.ExpiresIn))

	for {
		// Some providers use a secret here, we don't need for sigstore oauth one so leave it off.
		data := url.Values{
			"grant_type":    []string{"urn:ietf:params:oauth:grant-type:device_code"},
			"client_id":     []string{clientID},
			"device_code":   []string{parsed.DeviceCode},
			"scope":         []string{"openid email"},
			"code_verifier": []string{pkce.Value},
		}

		/* #nosec */
		resp, err := http.PostForm(p.Endpoint().TokenURL, data)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		tr := tokenResp{}
		if err := json.Unmarshal(b, &tr); err != nil {
			return "", err
		}

		if tr.IDToken != "" {
			d.MessagePrinter("Token received!")
			return tr.IDToken, nil
		}
		switch tr.Error {
		case "access_denied", "expired_token":
			return "", fmt.Errorf("error obtaining token: %s", tr.Error)
		case "authorization_pending":
			d.Sleeper(time.Duration(parsed.Interval) * time.Second)
		case "slow_down":
			// Add ten seconds if we got told to slow down
			d.Sleeper(time.Duration(parsed.Interval)*time.Second + 10*time.Second)
		default:
			return "", fmt.Errorf("unexpected error in device flow: %s", tr.Error)
		}
	}
}

// GetIDToken gets an OIDC ID Token from the specified provider using the device code grant flow
func (d *DeviceFlowTokenGetter) GetIDToken(p *oidc.Provider, cfg oauth2.Config) (*OIDCIDToken, error) {
	idToken, err := d.deviceFlow(p, cfg.ClientID, cfg.RedirectURL)
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

// CodeURL fetches the device authorization endpoint URL from the provider's well-known configuration endpoint
func (d *DeviceFlowTokenGetter) CodeURL() (string, error) {
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
		Issuer         string `json:"issuer"`
		DeviceEndpoint string `json:"device_authorization_endpoint"`
	}{}
	if err = json.Unmarshal(body, &providerConfig); err != nil {
		return "", fmt.Errorf("oidc: failed to decode provider discovery object: %w", err)
	}

	if d.Issuer != providerConfig.Issuer {
		return "", fmt.Errorf("oidc: issuer did not match the issuer returned by provider, expected %q got %q", d.Issuer, providerConfig.Issuer)
	}

	if providerConfig.DeviceEndpoint == "" {
		return "", fmt.Errorf("oidc: device authorization endpoint not returned by provider")
	}

	d.codeURL = providerConfig.DeviceEndpoint
	return d.codeURL, nil
}
