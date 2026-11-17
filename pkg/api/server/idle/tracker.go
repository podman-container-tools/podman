//go:build !remote

package idle

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Tracker holds the state for the server's idle tracking
type Tracker struct {
	// Duration is the API idle window
	Duration time.Duration
	hijacked int                   // count of active connections managed by handlers
	managed  map[net.Conn]struct{} // set of active connections managed by http package
	mux      sync.Mutex            // protect managed map
	timer    *time.Timer
	total    int // total number of connections made to this server instance
}

// NewTracker creates and initializes a new Tracker object
// For best behavior, duration should be 2x http idle connection timeout
func NewTracker(idle time.Duration) *Tracker {
	return &Tracker{
		managed:  make(map[net.Conn]struct{}),
		Duration: idle,
		timer:    time.NewTimer(idle),
	}
}

// ConnState is called on HTTP connection state changes.
//   - Once StateHijacked, StateClose is _NOT_ called on that connection
//   - There are two "idle" timeouts, the http idle connection (not to be confused with the TCP/IP idle socket timeout)
//     and the API idle window.  The caller should set the http idle timeout to 2x the time provided to NewTacker() which
//     is the API idle window.
func (t *Tracker) ConnState(conn net.Conn, state http.ConnState) {
	t.mux.Lock()
	defer t.mux.Unlock()

	logrus.WithFields(logrus.Fields{
		"X-Reference-Id": fmt.Sprintf("%p", conn),
	}).Debugf("IdleTracker:%v %dm+%dh/%dt connection(s)", state, len(t.managed), t.hijacked, t.TotalConnections())

	switch state {
	case http.StateNew:
		t.total++
	case http.StateActive:
		// stop the API timer when the server transitions any connection to an "active" state
		t.managed[conn] = struct{}{}
		t.timer.Stop()
	case http.StateHijacked:
		// hijacked connections should call Close() when finished.
		// Note: If a handler hijack's a connection and then doesn't Close() it,
		//       the API timer will not fire and the server will _NOT_ timeout.
		delete(t.managed, conn)
		t.hijacked++
	case http.StateIdle:
		// When any connection goes into the http idle state, we know:
		// - we have an active connection
		// - the API timer should not be counting down (See case StateNew/StateActive)
		break
	case http.StateClosed:
		oldActive := t.ActiveConnections()

		// Either the server or a hijacking handler has closed the http connection to a client
		if conn == nil {
			t.hijacked-- // guarded by t.mux above
		} else {
			if _, found := t.managed[conn]; found {
				delete(t.managed, conn)
			} else {
				logrus.WithFields(logrus.Fields{
					"X-Reference-Id": fmt.Sprintf("%p", conn),
				}).Warnf("IdleTracker: StateClosed transition by connection marked un-managed")
			}
		}

		// Transitioned from any "active" connection to no connections
		if oldActive > 0 && t.ActiveConnections() == 0 {
			t.timer.Stop()            // See library source for Reset() issues and why they are not fixed
			t.timer.Reset(t.Duration) // Restart the API window timer
		}
	}
}

// Close is used to update Tracker that a StateHijacked connection has been closed by handler (StateClosed)
func (t *Tracker) Close() {
	t.ConnState(nil, http.StateClosed)
}

// ActiveConnections returns the number of current managed or StateHijacked connections
func (t *Tracker) ActiveConnections() int {
	return len(t.managed) + t.hijacked
}

// TotalConnections returns total number of connections made to this instance of the service
func (t *Tracker) TotalConnections() int {
	return t.total
}

// Done is called when idle timer has expired
func (t *Tracker) Done() <-chan time.Time {
	return t.timer.C
}
