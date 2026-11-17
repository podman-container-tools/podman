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

package oauthflow

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"golang.org/x/oauth2"
)

const (
	// PKCES256 is the SHA256 option required by the PKCE RFC
	PKCES256 = "S256"
)

// PKCE specifies the challenge and value pair required to fulfill RFC7636
type PKCE struct {
	Challenge string
	Method    string
	Value     string
}

// NewPKCE creates a new PKCE challenge for the specified provider per its supported methods (obtained through OIDC discovery endpoint)
func NewPKCE(provider *oidc.Provider) (*PKCE, error) {
	var providerClaims struct {
		CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
	}

	if err := provider.Claims(&providerClaims); err != nil {
		// will only error out if the JSON was malformed, which shouldn't happen at this point
		return nil, err
	}

	var chosenMethod string
	for _, method := range providerClaims.CodeChallengeMethodsSupported {
		// per RFC7636, any server that supports PKCE must support S256
		if method == PKCES256 && chosenMethod != PKCES256 {
			chosenMethod = PKCES256
			break
		} else if method != "plain" {
			fmt.Printf("Unsupported code challenge method in list: '%v'", method)
		}
	}
	if chosenMethod == "" {
		if providerIsAzureBacked(provider) {
			chosenMethod = PKCES256
		} else {
			return nil, fmt.Errorf("PKCE is not supported by OIDC provider '%v'", provider.Endpoint().AuthURL)
		}
	}

	// The value must meet requirements of RFC 7636:
	// (minimum length of 43 characters and a maximum length of 128 characters)
	// 384 bits of entropy (48 bytes) to be encoded in base64 URL-safe without padding yields 64 characters.
	value := cryptoutils.GenerateRandomURLSafeString(384)

	h := sha256.New()
	_, _ = h.Write([]byte(value))
	challenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return &PKCE{
		Challenge: challenge,
		Method:    chosenMethod,
		Value:     value,
	}, nil
}

// AuthURLOpts returns the set of request parameters required during the initial exchange of the OAuth2 flow
func (p *PKCE) AuthURLOpts() []oauth2.AuthCodeOption {
	return []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge_method", p.Method),
		oauth2.SetAuthURLParam("code_challenge", p.Challenge),
	}
}

// TokenURLOpts returns the set of request parameters required during the token request exchange flow
func (p *PKCE) TokenURLOpts() []oauth2.AuthCodeOption {
	return []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_verifier", p.Value),
	}
}

var azureregex = regexp.MustCompile(`^https:\/\/login\.microsoftonline\.(com|us)\/`)

// providerIsAzureBacked returns a boolean indicating whether the provider is Azure-backed;
// Azure supports PKCE but does not advertise it in their OIDC discovery endpoint
func providerIsAzureBacked(p *oidc.Provider) bool {
	// Per https://docs.microsoft.com/en-us/azure/active-directory/develop/authentication-national-cloud#azure-ad-authentication-endpoints
	// if endpoint starts with any of these strings then we should attempt PKCE anyway as their OIDC discovery doc
	// does not advertise supporting PKCE but they actually do

	return p != nil && azureregex.MatchString(p.Endpoint().AuthURL)
}
