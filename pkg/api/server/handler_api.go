//go:build !remote && (linux || freebsd)

package server

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/version"
)

type BufferedResponseWriter struct {
	b *bufio.Writer
	w http.ResponseWriter
}

// APIHandler is a wrapper to enhance HandlerFunc's and remove redundant code
func (s *APIServer) APIHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Wrapper to hide some boilerplate
		s.apiWrapper(h, w, r, false)
	}
}

// An API Handler to help historical clients with broken parsing that expect
// streaming JSON payloads to be reliably messaged framed (full JSON record
// always fits in each read())
func (s *APIServer) StreamBufferedAPIHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Wrapper to hide some boilerplate
		s.apiWrapper(h, w, r, true)
	}
}

func (s *APIServer) apiWrapper(h http.HandlerFunc, w http.ResponseWriter, r *http.Request, buffer bool) {
	if err := r.ParseForm(); err != nil {
		logrus.WithFields(logrus.Fields{
			"X-Reference-Id": r.Header.Get("X-Reference-Id"),
		}).Info("Failed Request: unable to parse form: " + err.Error())
	}

	cv := version.APIVersion[version.Compat][version.CurrentAPI]
	w.Header().Set("API-Version", fmt.Sprintf("%d.%d", cv.Major, cv.Minor))

	lv := version.APIVersion[version.Libpod][version.CurrentAPI].String()
	w.Header().Set("Libpod-API-Version", lv)
	w.Header().Set("Server", "Libpod/"+lv+" ("+runtime.GOOS+")")

	if s.CorsHeaders != "" {
		w.Header().Set("Access-Control-Allow-Origin", s.CorsHeaders)
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, X-Registry-Auth, Connection, Upgrade, X-Registry-Config")
		w.Header().Set("Access-Control-Allow-Methods", "HEAD, GET, POST, DELETE, PUT, OPTIONS")
	}

	if buffer {
		bw := newBufferedResponseWriter(w)
		defer bw.b.Flush()
		w = bw
	}

	h(w, r)
}

// VersionedPath prepends the version parsing code
// any handler may override this default when registering URL(s)
func VersionedPath(p string) string {
	return "/v{version:[0-9][0-9A-Za-z.-]*}" + p
}

func (w *BufferedResponseWriter) Header() http.Header {
	return w.w.Header()
}

func (w *BufferedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	_ = w.b.Flush()
	if wrapped, ok := w.w.(http.Hijacker); ok {
		return wrapped.Hijack()
	}

	return nil, nil, errors.New("ResponseWriter does not support hijacking")
}

func (w *BufferedResponseWriter) Write(b []byte) (int, error) {
	return w.b.Write(b)
}

func (w *BufferedResponseWriter) WriteHeader(statusCode int) {
	w.w.WriteHeader(statusCode)
}

func (w *BufferedResponseWriter) Flush() {
	_ = w.b.Flush()
	if wrapped, ok := w.w.(http.Flusher); ok {
		wrapped.Flush()
	}
}

func newBufferedResponseWriter(rw http.ResponseWriter) *BufferedResponseWriter {
	return &BufferedResponseWriter{
		bufio.NewWriterSize(rw, 8192),
		rw,
	}
}
