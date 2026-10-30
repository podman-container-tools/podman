//go:build !remote && (linux || freebsd)

package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"go.podman.io/podman/v6/pkg/api/types"
)

// referenceIDHandler adds X-Reference-Id Header allowing event correlation
// and Apache style request logging
func referenceIDHandler() mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		// Only log Apache access_log-like entries at Info level or below
		out := io.Discard
		if slog.Default().Enabled(context.Background(), slog.LevelInfo) {
			out = log.Writer()
		}

		return handlers.CombinedLoggingHandler(out,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rid := r.Header.Get("X-Reference-Id")
				if rid == "" {
					if c := r.Context().Value(types.ConnKey); c == nil {
						rid = uuid.New().String()
					} else {
						rid = fmt.Sprintf("%p", c)
					}
				}

				r.Header.Set("X-Reference-Id", rid)
				w.Header().Set("X-Reference-Id", rid)
				h.ServeHTTP(w, r)
			}))
	}
}
