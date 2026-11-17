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
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	soauth "github.com/sigstore/sigstore/pkg/oauth"
	"golang.org/x/oauth2"
)

const (
	// PublicInstanceGithubAuthSubURL Default connector ids used by `oauth2.sigstore.dev` for Github
	PublicInstanceGithubAuthSubURL = "https://github.com/login/oauth"
	// PublicInstanceGoogleAuthSubURL Default connector ids used by `oauth2.sigstore.dev` for Google
	PublicInstanceGoogleAuthSubURL = "https://accounts.google.com"
	// PublicInstanceMicrosoftAuthSubURL Default connector ids used by `oauth2.sigstore.dev` for Microsoft
	PublicInstanceMicrosoftAuthSubURL = "https://login.microsoftonline.com"
)

// TokenGetter provides a way to get an OIDC ID Token from an OIDC IdP
type TokenGetter interface {
	GetIDToken(provider *oidc.Provider, config oauth2.Config) (*OIDCIDToken, error)
}

// OIDCIDToken represents an OIDC Identity Token
type OIDCIDToken struct {
	RawString string // RawString provides the raw token (a base64-encoded JWT) value
	Subject   string // Subject is the extracted subject from the raw token
}

// init
func init() {
	// set the default HTML page for the DefaultIDTokenGetter
	htmlPage, err := soauth.GetInteractiveSuccessHTML(false, 10)
	if err != nil {
		log.Print("failed to get interactive success html, defaulting to original static page")
	} else {
		DefaultIDTokenGetter.HTMLPage = htmlPage
	}
}

// ConnectorIDOpt requests the value of prov as a the connector_id (either on URL or in form body) on the initial request;
// this is used by Dex
func ConnectorIDOpt(prov string) oauth2.AuthCodeOption {
	return oauth2.SetAuthURLParam("connector_id", prov)
}

// DefaultIDTokenGetter is the default implementation.
// The HTML page and message printed to the terminal can be customized.
var DefaultIDTokenGetter = &InteractiveIDTokenGetter{
	HTMLPage: soauth.InteractiveSuccessHTML,
}

// PublicInstanceGithubIDTokenGetter is a `oauth2.sigstore.dev` flow selecting github as an Idp
// Flow is based on `DefaultIDTokenGetter` fields
var PublicInstanceGithubIDTokenGetter = &InteractiveIDTokenGetter{
	HTMLPage:           DefaultIDTokenGetter.HTMLPage,
	ExtraAuthURLParams: []oauth2.AuthCodeOption{ConnectorIDOpt(PublicInstanceGithubAuthSubURL)},
}

// PublicInstanceGoogleIDTokenGetter is a `oauth2.sigstore.dev` flow selecting github as an Idp
// Flow is based on `DefaultIDTokenGetter` fields
var PublicInstanceGoogleIDTokenGetter = &InteractiveIDTokenGetter{
	HTMLPage:           DefaultIDTokenGetter.HTMLPage,
	ExtraAuthURLParams: []oauth2.AuthCodeOption{ConnectorIDOpt(PublicInstanceGoogleAuthSubURL)},
}

// PublicInstanceMicrosoftIDTokenGetter is a `oauth2.sigstore.dev` flow selecting microsoft as an Idp
// Flow is based on `DefaultIDTokenGetter` fields
var PublicInstanceMicrosoftIDTokenGetter = &InteractiveIDTokenGetter{
	HTMLPage:           DefaultIDTokenGetter.HTMLPage,
	ExtraAuthURLParams: []oauth2.AuthCodeOption{ConnectorIDOpt(PublicInstanceMicrosoftAuthSubURL)},
}

// OIDConnect requests an OIDC Identity Token from the specified issuer using the specified client credentials and TokenGetter
// NOTE: If the redirectURL is empty a listener on localhost:0 is configured with '/auth/callback' as default path.
func OIDConnect(issuer, id, secret, redirectURL string, tg TokenGetter) (*OIDCIDToken, error) {
	// Check if it's a StaticTokenGetter since NewProvider below will make
	// network calls unnecessarily and they are ignored.
	if sg, ok := tg.(*StaticTokenGetter); ok {
		return sg.GetIDToken(nil, oauth2.Config{})
	}
	provider, err := oidc.NewProvider(context.Background(), issuer)
	if err != nil {
		return nil, err
	}
	config := oauth2.Config{
		ClientID:     id,
		ClientSecret: secret,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "email"},
		RedirectURL:  redirectURL,
	}

	return tg.GetIDToken(provider, config)
}

type stringAsBool bool

func (sb *stringAsBool) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case "true", `"true"`, "True", `"True"`:
		*sb = true
	case "false", `"false"`, "False", `"False"`:
		*sb = false
	default:
		return errors.New("invalid value for boolean")
	}
	return nil
}

type claims struct {
	Email    string       `json:"email"`
	Verified stringAsBool `json:"email_verified"`
	Subject  string       `json:"sub"`
}

// SubjectFromToken extracts the subject claim from an OIDC Identity Token
func SubjectFromToken(tok *oidc.IDToken) (string, error) {
	claims := claims{}
	if err := tok.Claims(&claims); err != nil {
		return "", err
	}
	return subjectFromClaims(claims)
}

// SubjectFromUnverifiedToken extracts the subject claim from the raw bytes of
// an OIDC identity token.
func SubjectFromUnverifiedToken(tok []byte) (string, error) {
	claims := claims{}
	if err := json.Unmarshal(tok, &claims); err != nil {
		return "", err
	}
	return subjectFromClaims(claims)
}

func subjectFromClaims(c claims) (string, error) {
	if c.Email != "" {
		if !c.Verified {
			return "", errors.New("not verified by identity provider")
		}
		return c.Email, nil
	}

	if c.Subject == "" {
		return "", errors.New("no subject found in claims")
	}
	return c.Subject, nil
}

// StaticTokenGetter is a token getter that works on a JWT that is already known
type StaticTokenGetter struct {
	RawToken string
}

// https://datatracker.ietf.org/doc/html/rfc7518#section-3.1
var allowedSignatureAlgorithms = []jose.SignatureAlgorithm{
	jose.ES256,
	jose.ES384,
	jose.ES512,
	jose.RS256,
	jose.RS384,
	jose.RS512,
	jose.PS256,
	jose.PS384,
	jose.PS512,
	jose.EdDSA,
	jose.HS256,
	jose.HS384,
	jose.HS512,
}

// GetIDToken extracts an OIDCIDToken from the raw token *without verification*
func (stg *StaticTokenGetter) GetIDToken(_ *oidc.Provider, _ oauth2.Config) (*OIDCIDToken, error) {
	unsafeTok, err := jose.ParseSigned(stg.RawToken, allowedSignatureAlgorithms)
	if err != nil {
		return nil, err
	}
	// THIS LOGIC IS GENERALLY UNSAFE BUT OK HERE
	// We are only parsing the id-token passed directly to a command line tool by a user, so it is trusted locally.
	// We need to extract the email address to attach an additional signed proof to the server.
	// THE SERVER WILL DO REAL VERIFICATION HERE
	unsafePayload := unsafeTok.UnsafePayloadWithoutVerification()
	claims := claims{}
	if err := json.Unmarshal(unsafePayload, &claims); err != nil {
		return nil, err
	}

	subj, err := subjectFromClaims(claims)
	if err != nil {
		return nil, err
	}

	return &OIDCIDToken{
		RawString: stg.RawToken,
		Subject:   subj,
	}, nil
}
