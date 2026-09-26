//go:build linux || freebsd

package main

import (
	"bytes"
	"context"
	"log/slog"
	"log/syslog"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	lslog "github.com/sirupsen/logrus/hooks/slog"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

// FIXME test all this. Does the recursion to slog.Default().Handler() work at all?!

func syslogHook() {
	if !registry.PodmanConfig().Syslog {
		return
	}

	handler, err := newSyslogHandler("", "", syslog.LOG_INFO, "")
	if err != nil {
		slog.Debug("Failed to initialize syslog", "err", err)
	} else {
		slog.SetDefault(slog.New(slog.NewMultiHandler(
			slog.Default().Handler(),
			handler,
		)))
	}
}

// syslogDest is a destination for a *slog.TextHandler.
// We ~need this because we don’t want to implement a full slog.Handler ourselves,
// and a *slog.TextHandler uses a single io.Writer destination
// even after WithAttrs / WithGroup; so, we will have multiple syslogHandler
// objects that need to coordinate using a single buffer.

type syslogDest struct {
	syslog *syslog.Writer

	bufLock sync.Mutex
	buf     bytes.Buffer // proteted by bufLock
}

type syslogHandler struct {
	dest *syslogDest
	th   slog.Handler // Writes into buf
}

func newSyslogHandler(network, raddr string, priority syslog.Priority, tag string) (*syslogHandler, error) {
	w, err := syslog.Dial(network, raddr, priority, tag)
	if err != nil {
		return nil, err
	}
	dest := syslogDest{
		syslog: w,
	}

	return &syslogHandler{
		dest: &dest,
		th: slog.NewTextHandler(&dest.buf, &slog.HandlerOptions{
			Level:       lslog.Level(logrus.TraceLevel).Level(),
			ReplaceAttr: syslogReplaceAttr,
		}),
	}, nil
}

func syslogReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.LevelKey {
		return slog.Attr{}
	}
	return a
}

// Enabled implements [slog.Handler].
func (s *syslogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle implements [slog.Handler].
func (s *syslogHandler) Handle(_ context.Context, r slog.Record) error {
	s.dest.bufLock.Lock()
	defer s.dest.bufLock.Unlock()

	rClone := r
	rClone.Time = time.Time{}
	s.dest.buf.Reset()
	if err := s.th.Handle(context.Background(), rClone); err != nil {
		return err
	}
	// FIXME? Strip the msg= field prefix and quoting?

	switch {
	case r.Level >= lslog.Level(logrus.FatalLevel).Level():
		return s.dest.syslog.Crit(s.dest.buf.String())
	case r.Level >= slog.LevelError:
		return s.dest.syslog.Err(s.dest.buf.String())
	case r.Level >= slog.LevelWarn:
		return s.dest.syslog.Warning(s.dest.buf.String())
	case r.Level >= slog.LevelInfo:
		return s.dest.syslog.Info(s.dest.buf.String())
	case r.Level >= slog.LevelDebug, r.Level >= lslog.Level(logrus.TraceLevel).Level():
		return s.dest.syslog.Debug(s.dest.buf.String())
	default:
		return nil
	}
}

// WithAttrs implements [slog.Handler].
func (s *syslogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &syslogHandler{
		dest: s.dest,
		th:   s.th.WithAttrs(attrs),
	}
}

// WithGroup implements [slog.Handler].
func (s *syslogHandler) WithGroup(name string) slog.Handler {
	return &syslogHandler{
		dest: s.dest,
		th:   s.th.WithGroup(name),
	}
}
