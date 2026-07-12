//go:build !remote && (linux || freebsd)

package server

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
)

// panicHandler captures panics from endpoint handlers and logs stack trace
func panicHandler() mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// http.Server hides panics from handlers, we want to record them and fix the cause
			defer func() {
				err := recover()
				if err != nil {
					buf := make([]byte, 1<<20)
					n := runtime.Stack(buf, true)
					logrus.Warnf("Recovering from API service endpoint handler panic: %v, %s", err, buf[:n])
					// Try to inform client things went south... won't work if handler already started writing response body
					utils.InternalServerError(w, fmt.Errorf("%v", err))
				}
			}()

			h.ServeHTTP(w, r)
		})
	}
}
