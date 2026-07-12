//go:build !remote && (linux || freebsd)

package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/shutdown"
	"go.podman.io/podman/v6/pkg/api/grpcpb"
	"go.podman.io/podman/v6/pkg/api/handlers"
	grpchandlers "go.podman.io/podman/v6/pkg/api/handlers/grpc"
	"go.podman.io/podman/v6/pkg/api/server/idle"
	"go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util/tlsutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type APIServer struct {
	http.Server                      // The HTTP work happens here
	grpc               *grpc.Server  // GRPC stuff happens here
	net.Listener                     // mux for routing HTTP API calls to libpod routines
	*libpod.Runtime                  // Where the real work happens
	*schema.Decoder                  // Decoder for Query parameters to structs
	context.CancelFunc               // Stop APIServer
	context.Context                  // Context to carry objects to handlers
	CorsHeaders        string        // Inject Cross-Origin Resource Sharing (CORS) headers
	PProfAddr          string        // Binding network address for pprof profiles
	idleTracker        *idle.Tracker // Track connections to support idle shutdown
	tlsCertFile        string        // TLS serving certificate PEM file
	tlsKeyFile         string        // TLS serving certificate private key PEM file
	tlsClientCAFile    string        // TLS client certifiicate CA bundle PEM file
}

// Number of seconds to wait for next request, if exceeded shutdown server
const (
	DefaultCorsHeaders       = ""
	DefaultServiceDuration   = 300 * time.Second
	UnlimitedServiceDuration = 0 * time.Second
)

// shutdownOnce ensures Shutdown() may safely be called from several go routines
var shutdownOnce sync.Once

// NewServerWithSettings will create and configure a new API server using provided settings
func NewServerWithSettings(runtime *libpod.Runtime, listener net.Listener, opts entities.ServiceOptions) (*APIServer, error) {
	return newServer(runtime, listener, opts)
}

func newServer(runtime *libpod.Runtime, listener net.Listener, opts entities.ServiceOptions) (*APIServer, error) {
	logrus.Infof("API service listening on %q. URI: %q", listener.Addr(), runtime.RemoteURI())
	if opts.CorsHeaders == "" {
		logrus.Debug("CORS Headers were not set")
	} else {
		logrus.Debugf("CORS Headers were set to %q", opts.CorsHeaders)
	}

	router := mux.NewRouter().UseEncodedPath()
	tracker := idle.NewTracker(opts.Timeout)

	serverProtocols := &http.Protocols{}
	serverProtocols.SetHTTP1(true)
	serverProtocols.SetHTTP2(true)

	server := APIServer{
		Server: http.Server{
			ConnContext: func(ctx context.Context, c net.Conn) context.Context {
				return context.WithValue(ctx, types.ConnKey, c)
			},
			ConnState:   tracker.ConnState,
			ErrorLog:    log.New(logrus.StandardLogger().Out, "", 0),
			Handler:     router,
			IdleTimeout: opts.Timeout * 2,
			Protocols:   serverProtocols,
		},
		grpc:            grpc.NewServer(),
		CorsHeaders:     opts.CorsHeaders,
		Listener:        listener,
		PProfAddr:       opts.PProfAddr,
		idleTracker:     tracker,
		tlsCertFile:     opts.TLSCertFile,
		tlsKeyFile:      opts.TLSKeyFile,
		tlsClientCAFile: opts.TLSClientCAFile,
	}

	router.NewRoute().HeadersRegexp("Content-Type", "application/grpc(\\+.*)?").Handler(server.grpc)
	reflection.Register(server.grpc)

	server.BaseContext = func(_ net.Listener) context.Context {
		ctx := context.WithValue(context.Background(), types.DecoderKey, handlers.NewAPIDecoder())
		ctx = context.WithValue(ctx, types.CompatDecoderKey, handlers.NewCompatAPIDecoder())
		ctx = context.WithValue(ctx, types.RuntimeKey, runtime)
		ctx = context.WithValue(ctx, types.IdleTrackerKey, tracker)
		return ctx
	}

	if opts.TLSClientCAFile != "" {
		logrus.Debugf("will validate client certs against %s", opts.TLSClientCAFile)
		pool, err := tlsutil.ReadCertBundle(opts.TLSClientCAFile)
		if err != nil {
			return nil, err
		}
		server.TLSConfig = &tls.Config{
			ClientCAs:  pool,
			ClientAuth: tls.RequireAndVerifyClientCert,
		}
	}

	// Capture panics and print stack traces for diagnostics,
	// additionally process X-Reference-Id Header to support event correlation
	router.Use(panicHandler(), referenceIDHandler())
	router.NotFoundHandler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// We can track user errors...
			logrus.Infof("Failed Request: (%d:%s) for %s:'%s'", http.StatusNotFound, http.StatusText(http.StatusNotFound), r.Method, r.URL.String())
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		},
	)

	router.MethodNotAllowedHandler = http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// We can track user errors...
			logrus.Infof("Failed Request: (%d:%s) for %s:'%s'", http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed), r.Method, r.URL.String())
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		},
	)

	for _, fn := range []func(*mux.Router) error{
		server.registerAuthHandlers,
		server.registerArtifactHandlers,
		server.registerArchiveHandlers,
		server.registerContainersHandlers,
		server.registerDistributionHandlers,
		server.registerEventsHandlers,
		server.registerExecHandlers,
		server.registerGenerateHandlers,
		server.registerHealthCheckHandlers,
		server.registerImagesHandlers,
		server.registerInfoHandlers,
		server.registerManifestHandlers,
		server.registerMonitorHandlers,
		server.registerNetworkHandlers,
		server.registerPingHandlers,
		server.registerKubeHandlers,
		server.registerPluginsHandlers,
		server.registerPodsHandlers,
		server.registerQuadletHandlers,
		server.registerSecretHandlers,
		server.registerSwaggerHandlers,
		server.registerSwarmHandlers,
		server.registerSystemHandlers,
		server.registerVersionHandlers,
		server.registerVolumeHandlers,
	} {
		if err := fn(router); err != nil {
			return nil, err
		}
	}

	grpcpb.RegisterNoopServer(server.grpc, grpchandlers.NewNoopServer(runtime)) // TODO: make this table-driven instead of a one-off?

	if logrus.IsLevelEnabled(logrus.TraceLevel) {
		// If in trace mode log request and response bodies
		router.Use(loggingHandler())
		_ = router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
			path, err := route.GetPathTemplate()
			if err != nil {
				path = "<N/A>"
			}
			methods, err := route.GetMethods()
			if err != nil {
				methods = []string{"<N/A>"}
			}
			logrus.Tracef("Methods: %6s Path: %s", strings.Join(methods, ", "), path)
			return nil
		})
	}

	return &server, nil
}

// setupSystemd notifies systemd API service is ready
// If the NOTIFY_SOCKET is set, communicate the PID and readiness, and unset INVOCATION_ID
// so conmon and containers are in the correct cgroup.  Also unset NOTIFY_SOCKET
// to avoid any further usage of the socket.
func (s *APIServer) setupSystemd() {
	if _, found := os.LookupEnv("NOTIFY_SOCKET"); !found {
		return
	}

	payload := fmt.Sprintf("MAINPID=%d\n", os.Getpid())
	payload += daemon.SdNotifyReady
	if sent, err := daemon.SdNotify(true, payload); err != nil {
		logrus.Errorf("API service failed to notify systemd of Conmon PID: %v", err)
	} else if !sent {
		logrus.Warn("API service unable to successfully send SDNotify")
	}

	if err := os.Unsetenv("INVOCATION_ID"); err != nil {
		logrus.Errorf("API service failed unsetting INVOCATION_ID: %v", err)
	}
	if err := os.Unsetenv("NOTIFY_SOCKET"); err != nil {
		logrus.Errorf("API service failed unsetting NOTIFY_SOCKET: %v", err)
	}
}

// Serve starts responding to HTTP requests.
func (s *APIServer) Serve() error {
	s.setupPprof()

	if err := shutdown.Register("service", func(_ os.Signal) error {
		s.grpc.GracefulStop()
		err := s.Shutdown(true)
		if err == nil {
			// For `systemctl stop podman.service` support, exit code should be 0
			// but only if we did indeed gracefully shutdown
			shutdown.SetExitCode(0)
		}
		return err
	}); err != nil {
		return err
	}
	// Start the shutdown signal handler.
	if err := shutdown.Start(); err != nil {
		return err
	}

	go func() {
		<-s.idleTracker.Done()
		logrus.Debugf("API service(s) shutting down, idle for %ds", int(s.idleTracker.Duration.Seconds()))
		_ = s.Shutdown(false)
	}()

	// Before we start serving, ensure umask is properly set for container creation.
	_ = syscall.Umask(0o022)

	errChan := make(chan error, 1)
	s.setupSystemd()
	go func() {
		var err error
		if s.tlsClientCAFile != "" || (s.tlsCertFile != "" && s.tlsKeyFile != "") {
			if s.tlsCertFile != "" && s.tlsKeyFile != "" {
				logrus.Debugf("serving TLS with cert %s and key %s", s.tlsCertFile, s.tlsKeyFile)
			}
			err = s.Server.ServeTLS(s.Listener, s.tlsCertFile, s.tlsKeyFile)
		} else {
			s.Server.Protocols.SetUnencryptedHTTP2(true)
			err = s.Server.Serve(s.Listener)
		}
		if err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("failed to start API service: %w", err)
			return
		}
		errChan <- nil
	}()

	return <-errChan
}

// setupPprof enables pprof default endpoints
// Note: These endpoints and the podman flag --cpu-profile are mutually exclusive
//
// Examples:
// #1 go tool pprof -http localhost:8889 localhost:8888/debug/pprof/heap?seconds=120
// Note: web page will only render after a sample has been recorded
// #2 curl http://localhost:8888/debug/pprof/heap > heap.pprof && go tool pprof heap.pprof
func (s *APIServer) setupPprof() {
	if s.PProfAddr == "" {
		return
	}

	logrus.Infof("pprof service listening on %q", s.PProfAddr)
	go func() {
		old := runtime.SetMutexProfileFraction(1)
		defer runtime.SetMutexProfileFraction(old)

		runtime.SetBlockProfileRate(1)
		defer runtime.SetBlockProfileRate(0)

		router := mux.NewRouter()
		router.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)

		err := http.ListenAndServe(s.PProfAddr, router)
		if err != nil && err != http.ErrServerClosed {
			logrus.Warnf("pprof service failed: %v", err)
		}
	}()
}

// Shutdown is a clean shutdown waiting on existing clients
func (s *APIServer) Shutdown(halt bool) error {
	switch {
	case halt:
		logrus.Debug("API service forced shutdown, ignoring timeout Duration")
	case s.idleTracker.Duration == UnlimitedServiceDuration:
		logrus.Debug("API service shutdown request ignored as timeout Duration is UnlimitedService")
		return nil
	}

	shutdownOnce.Do(func() {
		logrus.Debugf("API service shutdown, %d/%d connection(s)",
			s.idleTracker.ActiveConnections(), s.idleTracker.TotalConnections())

		// Gracefully shutdown server(s), duration of wait same as idle window
		deadline := 1 * time.Second
		if s.idleTracker.Duration > 0 {
			deadline = s.idleTracker.Duration
		}
		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		go func() {
			defer cancel()

			err := s.Server.Shutdown(ctx)
			if err != nil && err != context.Canceled && err != http.ErrServerClosed {
				logrus.Error("Failed to cleanly shutdown API service: " + err.Error())
			}
		}()
		<-ctx.Done()
	})
	return nil
}
