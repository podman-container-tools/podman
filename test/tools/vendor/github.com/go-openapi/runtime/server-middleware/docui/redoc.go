// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package docui

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path"
)

// UseRedoc creates a middleware to serve a documentation site for a swagger spec using [Redoc].
//
// By default, the page loads Redoc v3. Use [WithRedocV2] to keep Redoc v2.
//
// [Redoc]: https://redocly.com/docs/redoc
func UseRedoc(opts ...Option) func(next http.Handler) http.Handler {
	pth, assets := redocSetup(opts)

	return func(next http.Handler) http.Handler {
		return serveUI(pth, assets, next)
	}
}

// Redoc creates a [http.Handler] to serve a documentation site for a swagger spec using [Redoc].
//
// By default, the UI is served at route "/docs"
//
// This allows for altering the spec before starting the [http] listener.
//
// By default, the page loads Redoc v3. Use [WithRedocV2] to keep Redoc v2.
//
// [Redoc]: https://redocly.com/docs/redoc
func Redoc(next http.Handler, opts ...Option) http.Handler {
	pth, assets := redocSetup(opts)

	return serveUI(pth, assets, next)
}

func redocSetup(opts []Option) (pth string, assets []byte) {
	o := optionsWithDefaults(opts)

	// defaults for redoc, resolved after all options so that WithRedocV2 applies in any order
	if o.Template == "" {
		o.Template = redocTemplate
		if o.redocV2 {
			o.Template = redocV2Template
		}
	}
	if o.AssetsURL == "" {
		o.AssetsURL = redocV3URL
		if o.redocV2 {
			o.AssetsURL = redocV2URL
		}
	}

	pth = path.Join(o.BasePath, o.Path)
	tmpl := template.Must(template.New("redoc").Parse(o.Template))
	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, o); err != nil {
		panic(fmt.Errorf("cannot execute template: %w", err))
	}

	return pth, buf.Bytes()
}

const (
	redocV3URL = "https://cdn.jsdelivr.net/npm/redoc@3/bundles/redoc.standalone.js"
	redocV2URL = "https://cdn.jsdelivr.net/npm/redoc@2/bundles/redoc.standalone.js"

	// redocTemplate loads the Redoc v3 bundle, which is an ES module.
	redocTemplate = `<!DOCTYPE html>
<html>
  <head>
    <title>{{ .Title }}</title>
		<!-- needed for adaptive design -->
		<meta charset="utf-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1">
		<link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">

    <!--
    ReDoc doesn't change outer page styles
    -->
    <style>
      body {
        margin: 0;
        padding: 0;
      }
    </style>
  </head>
  <body>
    <redoc spec-url='{{ .SpecURL }}'></redoc>
    <script type="module" src="{{ .AssetsURL }}"> </script>
  </body>
</html>
`

	// redocV2Template loads the Redoc v2 bundle, which is a classic UMD script.
	//
	// The v2 bundle fails when loaded as a module: its wrapper reads properties of the
	// top-level "this", which is undefined in a module.
	redocV2Template = `<!DOCTYPE html>
<html>
  <head>
    <title>{{ .Title }}</title>
		<!-- needed for adaptive design -->
		<meta charset="utf-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1">
		<link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">

    <!--
    ReDoc doesn't change outer page styles
    -->
    <style>
      body {
        margin: 0;
        padding: 0;
      }
    </style>
  </head>
  <body>
    <redoc spec-url='{{ .SpecURL }}'></redoc>
    <script src="{{ .AssetsURL }}"> </script>
  </body>
</html>
`
)
