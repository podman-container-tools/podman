package mpb

import (
	"cmp"
	"io"
	"sync"
	"time"
)

// ContainerOption is a func option to alter default behavior of a bar
// container. Container term refers to a Progress struct which can
// hold one or more Bars.
type ContainerOption func(*pState)

// WithWaitGroup provides means to have a single joint point. If
// *sync.WaitGroup is provided, you can safely call just p.Wait()
// without calling Wait() on provided *sync.WaitGroup. Makes sense
// when there are more than one bar to render.
//
// If a goroutine exits early (for example on error) before the bar
// reaches its total, call (*Bar).Abort on that bar so p.Wait()
// does not wait indefinitely for the bar to reach its total. The
// provided *sync.WaitGroup must still be balanced: call wg.Add(1)
// before starting each goroutine and defer wg.Done() inside it, even
// on the error path.
func WithWaitGroup(wg *sync.WaitGroup) ContainerOption {
	return func(s *pState) {
		s.uwg = wg
	}
}

// WithWidth sets container width. If not set it defaults to terminal
// width. A bar added to the container will inherit its width, unless
// overridden by `func BarWidth(int) BarOption`.
func WithWidth(width int) ContainerOption {
	return func(s *pState) {
		s.reqWidth = width
	}
}

// WithQueueLen sets buffer size of heap manager channel. Ideally it must be
// kept at MAX value, where MAX is number of bars to be rendered at the same
// time. Default queue len is 64.
func WithQueueLen(len int) ContainerOption {
	return func(s *pState) {
		s.hmQueueLen = len
	}
}

// WithRefreshRate overrides default 150ms refresh rate.
func WithRefreshRate(d time.Duration) ContainerOption {
	return func(s *pState) {
		s.refreshRate = d
	}
}

// WithManualRefresh disables internal auto refresh time.Ticker.
// Refresh will occur on value receive from provided ch, yet last bar
// will still trigger final refresh cycle on its completion or abortion,
// similar to last person switches tv off analogy here.
func WithManualRefresh(ch <-chan any) ContainerOption {
	return func(s *pState) {
		s.manualRC = ch
	}
}

// WithRenderDelay delays rendering. By default rendering starts as
// soon as bar is added, with this option it's possible to delay
// rendering process by keeping provided chan unclosed. In other words
// rendering will start as soon as provided chan is closed.
func WithRenderDelay(ch <-chan any) ContainerOption {
	return func(s *pState) {
		s.delayRC = ch
	}
}

// WithShutdownNotifier closes provided channel on shutdown event,
// i.e. after `(*Progress) Wait()` or `(*Progress) Shutdown()` call.
func WithShutdownNotifier(ch chan any) ContainerOption {
	return func(s *pState) {
		s.shutdownNotifier = ch
	}
}

// WithOutput overrides default os.Stdout output. If underlying io.Writer
// is not a terminal then auto refresh is disabled unless WithAutoRefresh
// option is set.
func WithOutput(w io.Writer) ContainerOption {
	return func(s *pState) {
		s.output = cmp.Or(w, io.Discard)
	}
}

// WithDebugOutput sets debug output. It's only used to write render error if any.
func WithDebugOutput(w io.Writer) ContainerOption {
	return func(s *pState) {
		s.debugOut = cmp.Or(w, io.Discard)
	}
}

// WithConsoleWriter overrides default implementation of ConsoleWriter interface.
// This option makes following options ineffective:
//   - WithOutput
//   - ForceTTY
func WithConsoleWriter(cw ConsoleWriter) ContainerOption {
	return func(s *pState) {
		s.cwriter = cw
	}
}

// WithAutoRefresh force auto refresh regardless of what output is set to.
// Applicable only if not WithManualRefresh set.
func WithAutoRefresh() ContainerOption {
	return func(s *pState) {
		s.autoRefresh = true
	}
}

// ForceAutoRefresh is an alias of WithAutoRefresh.
func ForceAutoRefresh() ContainerOption {
	return WithAutoRefresh()
}

// ForceTTY force treating output as tty.
// This one implicitly enables WithAutoRefresh unless WithManualRefresh specified.
// Can be handy if you need to wrap os.Stdout or os.Stderr for example like:
// mpb.WithOutput(io.MultiWriter(os.Stdout, &someTestBuf)).
func ForceTTY() ContainerOption {
	return func(s *pState) {
		s.forceTTY = true
	}
}

// PopCompletedMode pop completed bars out of progress container.
// In this mode completed bars get moved to the top and stop
// participating in rendering cycle.
func PopCompletedMode() ContainerOption {
	return func(s *pState) {
		s.popCompleted = true
	}
}

// ContainerOptional will return provided option only when cond is true.
func ContainerOptional(option ContainerOption, cond bool) ContainerOption {
	if cond {
		return option
	}
	return nil
}

// ContainerOptOn will return provided option only when predicate evaluates to true.
func ContainerOptOn(option ContainerOption, predicate func() bool) ContainerOption {
	if predicate() {
		return option
	}
	return nil
}

// ContainerFuncOptional will call option and return its value only when cond is true.
func ContainerFuncOptional(option func() ContainerOption, cond bool) ContainerOption {
	if cond {
		return option()
	}
	return nil
}

// ContainerFuncOptOn will call option and return its value only when predicate evaluates to true.
func ContainerFuncOptOn(option func() ContainerOption, predicate func() bool) ContainerOption {
	if predicate() {
		return option()
	}
	return nil
}

// withDepleteHeap for test purposes only
func withDepleteHeap(ch chan<- *Bar) ContainerOption {
	return func(s *pState) {
		s.depleteHeap = ch
	}
}
