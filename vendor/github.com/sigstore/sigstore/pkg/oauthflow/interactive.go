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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/pkg/browser"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"golang.org/x/oauth2"
)

const oobRedirectURI = "urn:ietf:wg:oauth:2.0:oob"

var browserOpener = browser.OpenURL

// InteractiveIDTokenGetter is a type to get ID tokens for oauth flows
type InteractiveIDTokenGetter struct {
	HTMLPage           string
	ExtraAuthURLParams []oauth2.AuthCodeOption
	Input              io.Reader
	Output             io.Writer
}

// GetIDToken gets an OIDC ID Token from the specified provider using an interactive browser session
func (i *InteractiveIDTokenGetter) GetIDToken(p *oidc.Provider, cfg oauth2.Config) (*OIDCIDToken, error) {
	// generate random fields and save them for comparison after OAuth2 dance
	stateToken := cryptoutils.GenerateRandomURLSafeString(128)
	nonce := cryptoutils.GenerateRandomURLSafeString(128)

	doneCh := make(chan string)
	errCh := make(chan error)
	// starts listener using the redirect_uri, otherwise starts on ephemeral port
	redirectServer, redirectURL, err := startRedirectListener(stateToken, i.HTMLPage, cfg.RedirectURL, doneCh, errCh)
	if err != nil {
		return nil, fmt.Errorf("starting redirect listener: %w", err)
	}
	defer func() {
		go func() {
			_ = redirectServer.Shutdown(context.Background())
		}()
	}()

	cfg.RedirectURL = redirectURL.String()

	// require that OIDC provider support PKCE to provide sufficient security for the CLI
	pkce, err := NewPKCE(p)
	if err != nil {
		return nil, err
	}

	opts := append(pkce.AuthURLOpts(), oauth2.AccessTypeOnline, oidc.Nonce(nonce))
	if len(i.ExtraAuthURLParams) > 0 {
		opts = append(opts, i.ExtraAuthURLParams...)
	}
	authCodeURL := cfg.AuthCodeURL(stateToken, opts...)
	var code string
	if err := browserOpener(authCodeURL); err != nil {
		// Swap to the out of band flow if we can't open the browser
		fmt.Fprintf(i.GetOutput(), "error opening browser: %v\n", err)
		code = i.doOobFlow(&cfg, stateToken, opts)
	} else {
		fmt.Fprintf(i.GetOutput(), "Your browser will now be opened to:\n%s\n", authCodeURL)
		code, err = getCode(doneCh, errCh)
		if err != nil {
			fmt.Fprintf(i.GetOutput(), "error getting code from local server: %v\n", err)
			code = i.doOobFlow(&cfg, stateToken, opts)
		}
	}
	token, err := cfg.Exchange(context.Background(), code, append(pkce.TokenURLOpts(), oidc.Nonce(nonce))...)
	if err != nil {
		return nil, err
	}

	// requesting 'openid' scope should ensure an id_token is given when exchanging the code for an access token
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("id_token not present")
	}

	// verify nonce, client ID, access token hash before using it
	verifier := p.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	parsedIDToken, err := verifier.Verify(context.Background(), idToken)
	if err != nil {
		return nil, err
	}
	if parsedIDToken.Nonce != nonce {
		return nil, errors.New("nonce does not match value sent")
	}
	if parsedIDToken.AccessTokenHash != "" {
		if err := parsedIDToken.VerifyAccessToken(token.AccessToken); err != nil {
			return nil, err
		}
	}

	email, err := SubjectFromToken(parsedIDToken)
	if err != nil {
		return nil, err
	}

	returnToken := OIDCIDToken{
		RawString: idToken,
		Subject:   email,
	}
	return &returnToken, nil
}

func (i *InteractiveIDTokenGetter) doOobFlow(cfg *oauth2.Config, stateToken string, opts []oauth2.AuthCodeOption) string {
	cfg.RedirectURL = oobRedirectURI

	authURL := cfg.AuthCodeURL(stateToken, opts...)
	fmt.Fprintln(i.GetOutput(), "Go to the following link in a browser:\n\n\t", authURL)
	fmt.Fprintf(i.GetOutput(), "Enter verification code: ")
	var code string
	_, _ = fmt.Fscanf(i.GetInput(), "%s", &code)
	// New line in case read input doesn't move cursor to next line.
	fmt.Fprintln(i.GetOutput())
	return code
}

// GetInput returns the input reader for the token getter. If one is not set,
// it defaults to stdin.
func (i *InteractiveIDTokenGetter) GetInput() io.Reader {
	if i.Input == nil {
		return os.Stdin
	}
	return i.Input
}

// GetOutput returns the output writer for the token getter. If one is not set,
// it defaults to stderr.
func (i *InteractiveIDTokenGetter) GetOutput() io.Writer {
	if i.Output == nil {
		return os.Stderr
	}
	return i.Output
}

func startRedirectListener(state, htmlPage, redirectURL string, doneCh chan string, errCh chan error) (*http.Server, *url.URL, error) {
	var listener net.Listener
	var urlListener *url.URL
	var err error

	if redirectURL == "" {
		listener, err = net.Listen("tcp", "localhost:0")
		if err != nil {
			return nil, nil, err
		}

		addr, ok := listener.Addr().(*net.TCPAddr)
		if !ok {
			return nil, nil, fmt.Errorf("listener addr is not TCPAddr")
		}

		urlListener = &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("localhost:%d", addr.Port),
			Path:   "/auth/callback",
		}
	} else {
		urlListener, err = url.Parse(redirectURL)
		if err != nil {
			return nil, nil, err
		}

		listener, err = net.Listen("tcp", urlListener.Host)
		if err != nil {
			return nil, nil, err
		}
	}

	m := http.NewServeMux()
	s := &http.Server{
		Addr:    urlListener.Host,
		Handler: m,

		// an arbitrary reasonable value to fix gosec lint error
		ReadHeaderTimeout: 2 * time.Second,
	}

	m.HandleFunc(urlListener.Path, func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		// even though these are fetched from the FormValue method,
		// these are supplied as query parameters
		if r.FormValue("state") != state {
			errCh <- errors.New("invalid state token")
			return
		}
		doneCh <- r.FormValue("code")
		fmt.Fprint(w, htmlPage)
	})

	go func() {
		if err := s.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	return s, urlListener, nil
}

func getCode(doneCh chan string, errCh chan error) (string, error) {
	timeoutCh := time.NewTimer(120 * time.Second)
	select {
	case code := <-doneCh:
		return code, nil
	case err := <-errCh:
		return "", err
	case <-timeoutCh.C:
		return "", errors.New("timeout")
	}
}
