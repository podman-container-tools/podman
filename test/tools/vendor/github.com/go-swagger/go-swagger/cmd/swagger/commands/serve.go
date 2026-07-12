// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"path"
	"strconv"

	"github.com/gorilla/handlers"
	"github.com/toqueteos/webbrowser"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
)

// ServeCmd to serve a swagger spec with docs ui.
type ServeCmd struct {
	BasePath string `description:"the base path to serve the spec and UI at"                           long:"base-path"`
	Flavor   string `choice:"redoc"                                                                    choice:"swagger"                                                    default:"redoc" description:"the flavor of docs, can be swagger or redoc" long:"flavor" short:"F"`
	DocURL   string `description:"override the url which takes a url query param to render the doc ui" long:"doc-url"`
	NoOpen   bool   `description:"when present won't open the browser to show the url"                 long:"no-open"`
	NoUI     bool   `description:"when present, only the swagger spec will be served"                  long:"no-ui"`
	Flatten  bool   `description:"when present, flatten the swagger spec before serving it"            long:"flatten"`
	Port     int    `description:"the port to serve this site"                                         env:"PORT"                                                          long:"port"     short:"p"`
	Host     string `default:"0.0.0.0"                                                                 description:"the interface to serve this site, defaults to 0.0.0.0" env:"HOST"      long:"host"`
	Path     string `default:"docs"                                                                    description:"the uri path at which the docs will be served"         long:"path"`
}

// Execute the serve command.
func (s *ServeCmd) Execute(args []string) error {
	if len(args) == 0 {
		return errors.New("specify the spec to serve as argument to the serve command")
	}

	specDoc, err := loads.Spec(args[0])
	if err != nil {
		return err
	}

	if s.Flatten {
		specDoc, err = specDoc.Expanded(&spec.ExpandOptions{
			SkipSchemas:         false,
			ContinueOnError:     true,
			AbsoluteCircularRef: true,
		})
		if err != nil {
			return err
		}
	}

	b, err := json.MarshalIndent(specDoc.Spec(), "", "  ")
	if err != nil {
		return err
	}

	basePath := s.BasePath
	if basePath == "" {
		basePath = "/"
	}

	listener, err := net.Listen("tcp4", net.JoinHostPort(s.Host, strconv.Itoa(s.Port))) //nolint:noctx // that's ok for a demo server
	if err != nil {
		return err
	}
	sh, sp, err := swag.SplitHostPort(listener.Addr().String())
	if err != nil {
		return err
	}
	if sh == "0.0.0.0" {
		sh = "localhost"
	}

	visit := s.DocURL
	handler := http.NotFoundHandler()
	if !s.NoUI {
		if s.Flavor == "redoc" {
			handler = middleware.Redoc(middleware.RedocOpts{
				BasePath: basePath,
				SpecURL:  path.Join(basePath, "swagger.json"),
				Path:     s.Path,
			}, handler)
			visit = fmt.Sprintf("http://%s%s", net.JoinHostPort(sh, strconv.Itoa(sp)), path.Join(basePath, "docs"))
		} else if visit != "" || s.Flavor == "swagger" {
			handler = middleware.SwaggerUI(middleware.SwaggerUIOpts{
				BasePath: basePath,
				SpecURL:  path.Join(basePath, "swagger.json"),
				Path:     s.Path,
			}, handler)
			visit = fmt.Sprintf("http://%s%s", net.JoinHostPort(sh, strconv.Itoa(sp)), path.Join(basePath, s.Path))
		}
	}

	handler = handlers.CORS()(middleware.Spec(basePath, b, handler))
	errFuture := make(chan error)
	go func() {
		docServer := new(http.Server)
		docServer.SetKeepAlivesEnabled(true)
		docServer.Handler = handler

		errFuture <- docServer.Serve(listener)
	}()

	if !s.NoOpen && !s.NoUI {
		err := webbrowser.Open(visit)
		if err != nil {
			return err
		}
	}
	log.Println("serving docs at", visit)
	return <-errFuture
}
