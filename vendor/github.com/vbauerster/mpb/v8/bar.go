package mpb

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v8/decor"
)

// Bar represents a progress bar.
type Bar struct {
	index        int // used by heap
	priority     int // used by heap
	frameCh      chan *renderFrame
	operateState chan func(*bState)
	container    *Progress
	bs           *bState
	bsOk         chan struct{}
	ctx          context.Context
	cancel       func()
}

type decorSyncTable [2][]*decor.Sync
type extenderFunc func(decor.Statistics, ...io.Reader) ([]io.Reader, error)

// bState is actual bar's state.
type bState struct {
	id              int
	priority        int
	reqWidth        int
	shutdown        int
	total           int64
	current         int64
	refill          int64
	buffers         [3]*bytes.Buffer
	decorGroups     [2][]decor.Decorator
	ewmaDecorators  []decor.EwmaDecorator
	filler          BarFiller
	extender        extenderFunc
	renderReq       chan<- time.Time
	waitBar         *Bar // key for (*pState).queueBars
	trimSpace       bool
	aborted         bool
	triggerComplete bool
	rmOnComplete    bool
	noPop           bool
	autoRefresh     bool
}

type renderFrame struct {
	rows         []io.Reader
	shutdown     int
	rmOnComplete bool
	noPop        bool
	err          error
}

// ProxyReader wraps io.Reader with metrics required for progress tracking.
// Panics if `r` is nil. If `r` is io.ReadCloser then calling Close on `pr`
// will close underlying `r`s io.ReadCloser. If underlying *Bar instance is
// already completed or aborted then value of `pr` is nil. If underlying
// *Bar instance was initialized with total <= 0 then it's necessary to call
// `(*Bar).SetTotal(-1, true)` after copy operation completes. Most of the
// time it means that there is need to call `(*Bar).SetTotal(-1, true)` after
// io.Copy(dst, pr) returns.
func (b *Bar) ProxyReader(r io.Reader) (pr io.ReadCloser) {
	if r == nil {
		panic(errors.New("expected non nil io.Reader"))
	}
	result := make(chan bool, 1)
	select {
	case b.operateState <- func(s *bState) { result <- len(s.ewmaDecorators) != 0 }:
		return newProxyReader(r, b, <-result)
	case <-b.ctx.Done():
		return nil
	}
}

// ProxyWriter wraps io.Writer with metrics required for progress tracking.
// Panics if `w` is nil. If `w` is io.WriteCloser then calling Close on `pw`
// will close underlying `w`s io.WriteCloser. If underlying *Bar instance is
// already completed or aborted then value of `pw` is nil. If underlying
// *Bar instance was initialized with total <= 0 then it's necessary to call
// `(*Bar).SetTotal(-1, true)` after copy operation completes. Most of the
// time it means that there is need to call `(*Bar).SetTotal(-1, true)` after
// io.Copy(pw, src) returns.
func (b *Bar) ProxyWriter(w io.Writer) (pw io.WriteCloser) {
	if w == nil {
		panic(errors.New("expected non nil io.Writer"))
	}
	result := make(chan bool, 1)
	select {
	case b.operateState <- func(s *bState) { result <- len(s.ewmaDecorators) != 0 }:
		return newProxyWriter(w, b, <-result)
	case <-b.ctx.Done():
		return nil
	}
}

// ID returns id of the bar.
func (b *Bar) ID() int {
	result := make(chan int, 1)
	select {
	case b.operateState <- func(s *bState) { result <- s.id }:
		return <-result
	case <-b.bsOk:
		return b.bs.id
	}
}

// Current returns bar's current value, in other words sum of all increments.
func (b *Bar) Current() int64 {
	result := make(chan int64, 1)
	select {
	case b.operateState <- func(s *bState) { result <- s.current }:
		return <-result
	case <-b.bsOk:
		return b.bs.current
	}
}

// SetRefill sets refill flag with specified amount.
// The underlying BarFiller will change its visual representation, to
// indicate refill event. Refill event may be referred to some retry
// operation for example.
func (b *Bar) SetRefill(amount int64) {
	select {
	case b.operateState <- func(s *bState) { s.refill = min(amount, s.current) }:
	case <-b.ctx.Done():
	}
}

// SetRefillCurrent sets refill to the current amount.
func (b *Bar) SetRefillCurrent() {
	b.SetRefill(math.MaxInt64)
}

// TraverseDecorators traverses available decorators and calls `cb`
// on each unwrapped one.
func (b *Bar) TraverseDecorators(cb func(decor.Decorator)) (ok bool) {
	select {
	case b.operateState <- func(s *bState) {
		for _, group := range s.decorGroups {
			for _, d := range group {
				cb(unwrap(d))
			}
		}
	}:
		return true
	case <-b.ctx.Done():
		return false
	}
}

// EnableTriggerComplete enables triggering complete event. It's effective
// only for bars which were constructed with `total <= 0`. If `current >= total`
// at the moment of call, complete event is triggered right away.
func (b *Bar) EnableTriggerComplete() {
	select {
	case b.operateState <- func(s *bState) {
		if s.triggerComplete {
			return
		}
		s.triggerComplete = true
		if s.current >= s.total {
			s.current = s.total
			b.done(s.renderReq, s.autoRefresh)
		}
	}:
	case <-b.ctx.Done():
	}
}

// SetTotal sets total to an arbitrary value. It's effective only for bar
// which was constructed with `total <= 0`. Setting total to negative value
// is equivalent to `(*Bar).SetTotal((*Bar).Current(), bool)` but faster.
// If `complete` is true complete event is triggered right away.
// Calling `(*Bar).EnableTriggerComplete` makes this one no operational.
func (b *Bar) SetTotal(total int64, complete bool) {
	select {
	case b.operateState <- func(s *bState) {
		if s.triggerComplete {
			return
		}
		if total < 0 {
			s.total = s.current
		} else {
			s.total = total
		}
		if complete {
			s.current = s.total
			s.triggerComplete = true
			b.done(s.renderReq, s.autoRefresh)
		}
	}:
	case <-b.ctx.Done():
	}
}

// SetCurrent sets progress' current to an arbitrary value.
func (b *Bar) SetCurrent(current int64) {
	if current < 0 {
		return
	}
	select {
	case b.operateState <- func(s *bState) {
		s.current = current
		if s.triggerComplete && s.current >= s.total {
			s.current = s.total
			b.done(s.renderReq, s.autoRefresh)
		}
	}:
	case <-b.ctx.Done():
	}
}

// Increment is a shorthand for b.IncrInt64(1).
func (b *Bar) Increment() {
	b.IncrInt64(1)
}

// IncrBy is a shorthand for b.IncrInt64(int64(n)).
func (b *Bar) IncrBy(n int) {
	b.IncrInt64(int64(n))
}

// IncrInt64 increments progress by amount of n.
func (b *Bar) IncrInt64(n int64) {
	select {
	case b.operateState <- func(s *bState) {
		s.current += n
		if s.triggerComplete && s.current >= s.total {
			s.current = s.total
			b.done(s.renderReq, s.autoRefresh)
		}
	}:
	case <-b.ctx.Done():
	}
}

// EwmaIncrement is a shorthand for b.EwmaIncrInt64(1, iterDur).
func (b *Bar) EwmaIncrement(iterDur time.Duration) {
	b.EwmaIncrInt64(1, iterDur)
}

// EwmaIncrBy is a shorthand for b.EwmaIncrInt64(int64(n), iterDur).
func (b *Bar) EwmaIncrBy(n int, iterDur time.Duration) {
	b.EwmaIncrInt64(int64(n), iterDur)
}

// EwmaIncrInt64 increments progress by amount of n and updates EWMA based
// decorators by dur of a single iteration.
func (b *Bar) EwmaIncrInt64(n int64, iterDur time.Duration) {
	select {
	case b.operateState <- func(s *bState) {
		for _, d := range s.ewmaDecorators {
			d.EwmaUpdate(n, iterDur)
		}
		s.current += n
		if s.triggerComplete && s.current >= s.total {
			s.current = s.total
			b.done(s.renderReq, s.autoRefresh)
		}
	}:
	case <-b.ctx.Done():
	}
}

// EwmaSetCurrent sets progress' current to an arbitrary value and updates
// EWMA based decorators by dur of a single iteration.
func (b *Bar) EwmaSetCurrent(current int64, iterDur time.Duration) {
	if current < 0 {
		return
	}
	select {
	case b.operateState <- func(s *bState) {
		n := current - s.current
		for _, d := range s.ewmaDecorators {
			d.EwmaUpdate(n, iterDur)
		}
		s.current = current
		if s.triggerComplete && s.current >= s.total {
			s.current = s.total
			b.done(s.renderReq, s.autoRefresh)
		}
	}:
	case <-b.ctx.Done():
	}
}

// DecoratorAverageAdjust adjusts decorators implementing decor.AverageDecorator interface.
// Call if there is need to set start time after decorators have been constructed.
func (b *Bar) DecoratorAverageAdjust(start time.Time) {
	b.TraverseDecorators(func(d decor.Decorator) {
		if d, ok := d.(decor.AverageDecorator); ok {
			d.AverageAdjust(start)
		}
	})
}

// SetPriority changes bar's order among multiple bars. Zero is highest
// priority, i.e. bar will be on top. If you don't need to set priority
// dynamically, better use BarPriority option.
func (b *Bar) SetPriority(priority int) {
	b.container.UpdateBarPriority(b, priority, false)
}

// Abort interrupts bar's running goroutine. Abort won't be engaged
// if bar is already in complete state. If drop is true bar will be
// removed as well. To make sure that bar has been removed call
// `(*Bar).Wait()` method.
func (b *Bar) Abort(drop bool) {
	select {
	case b.operateState <- func(s *bState) {
		if s.aborted || s.completed() {
			return
		}
		s.aborted = true
		s.rmOnComplete = drop
		s.triggerComplete = true
		b.done(s.renderReq, s.autoRefresh)
	}:
	case <-b.ctx.Done():
	}
}

// Aborted reports whether the bar is in aborted state.
func (b *Bar) Aborted() bool {
	result := make(chan bool, 1)
	select {
	case b.operateState <- func(s *bState) { result <- s.aborted }:
		return <-result
	case <-b.bsOk:
		return b.bs.aborted
	}
}

// Completed reports whether the bar is in completed state.
func (b *Bar) Completed() bool {
	result := make(chan bool, 1)
	select {
	case b.operateState <- func(s *bState) { result <- s.completed() }:
		return <-result
	case <-b.bsOk:
		return b.bs.completed()
	}
}

// AbortedOrCompleted reports whether a bar is in aborted or completed state.
// Faster and atomic version of `(*Bar).Aborted() || (*Bar).Completed()`.
func (b *Bar) AbortedOrCompleted() bool {
	result := make(chan bool, 1)
	select {
	case b.operateState <- func(s *bState) {
		result <- s.aborted || s.completed()
	}:
		return <-result
	case <-b.bsOk:
		return b.bs.aborted || b.bs.completed()
	}
}

// Wait blocks until bar is completed or aborted.
func (b *Bar) Wait() {
	<-b.bsOk
}

func (b *Bar) serve(bs *bState) {
	decoratorsOnShutdown := func(group []decor.Decorator) {
		for _, d := range group {
			if d, ok := unwrap(d).(decor.ShutdownListener); ok {
				d.OnShutdown()
			}
		}
	}
	defer func() {
		decoratorsOnShutdown(bs.decorGroups[0])
		decoratorsOnShutdown(bs.decorGroups[1])
		b.bs = bs
		close(b.bsOk)
		b.container.bwg.Done()
	}()
	for {
		select {
		case op := <-b.operateState:
			op(bs)
		case <-b.ctx.Done():
			// bar can be aborted by canceling parent ctx without calling b.Abort
			bs.aborted = bs.aborted || !bs.completed()
			return
		}
	}
}

func (b *Bar) render(tw int) {
	fn := func(s *bState) {
		frame := new(renderFrame)
		stat := s.newStatistics(tw)
		r, err := s.draw(stat)
		if err != nil {
			for _, buf := range s.buffers {
				buf.Reset()
			}
			frame.err = err
			b.frameCh <- frame
			return
		}
		frame.rows, frame.err = s.extender(stat, r)
		if s.aborted || s.completed() {
			frame.shutdown = s.shutdown
			frame.rmOnComplete = s.rmOnComplete
			frame.noPop = s.noPop
			// post increment makes sure OnComplete decorators are rendered
			s.shutdown++
		}
		b.frameCh <- frame
	}
	select {
	case b.operateState <- fn:
	case <-b.bsOk:
		fn(b.bs)
	}
}

func (b *Bar) wSyncTable() decorSyncTable {
	result := make(chan decorSyncTable, 1)
	select {
	case b.operateState <- func(s *bState) { result <- s.wSyncTable() }:
		return <-result
	case <-b.bsOk:
		return b.bs.wSyncTable()
	}
}

func (s *bState) draw(stat decor.Statistics) (io.Reader, error) {
	decorFiller := func(buf *bytes.Buffer, group []decor.Decorator) (err error) {
		for i, d := range group {
			// need to call Decor in any case because of width synchronization
			str, width := d.Decor(stat)
			if i != 0 && err != nil {
				continue
			}
			if w := stat.AvailableWidth - width; w >= 0 {
				_, err = buf.WriteString(str)
				stat.AvailableWidth = w
			} else if stat.AvailableWidth > 0 {
				trunc := runewidth.Truncate(stripansi.Strip(str), stat.AvailableWidth, "…")
				_, err = buf.WriteString(trunc)
				stat.AvailableWidth = 0
			}
		}
		return err
	}

	for i, buf := range s.buffers[1:] {
		err := decorFiller(buf, s.decorGroups[i])
		if err != nil {
			return nil, err
		}
	}

	if s.trimSpace || stat.AvailableWidth < 2 {
		err := s.filler.Fill(s.buffers[0], stat)
		return io.MultiReader(
			s.buffers[1],
			s.buffers[0],
			s.buffers[2],
			strings.NewReader("\n"),
		), err
	}

	stat.AvailableWidth -= 2
	err := s.filler.Fill(s.buffers[0], stat)
	return io.MultiReader(
		s.buffers[1],
		strings.NewReader(" "),
		s.buffers[0],
		strings.NewReader(" "),
		s.buffers[2],
		strings.NewReader("\n"),
	), err
}

func (s *bState) wSyncTable() (table decorSyncTable) {
	var start int
	var row []*decor.Sync

	for i, group := range s.decorGroups {
		for _, d := range group {
			if s, ok := d.Sync(); ok {
				row = append(row, s)
			}
		}
		table[i], start = row[start:], len(row)
	}
	return table
}

func (b *Bar) done(renderReq chan<- time.Time, autoRefresh bool) {
	if autoRefresh {
		// Technically this call isn't required, but if refresh rate is set to
		// one hour for example and bar completes within a few minutes p.Wait()
		// will wait for one hour. This call helps to avoid unnecessary waiting.
		go b.tryEarlyRefresh(renderReq)
	} else {
		b.cancel()
	}
}

func (b *Bar) tryEarlyRefresh(renderReq chan<- time.Time) {
	otherRunning := make(chan struct{})
	ok := b.container.iterateBars(func(bar *Bar) bool {
		if b != bar && bar.isRunning() {
			close(otherRunning)
			return false // stop traverse
		}
		return true // continue traverse
	})
	if ok {
		select {
		case <-otherRunning:
		default:
			for {
				select {
				case renderReq <- time.Now():
				case <-b.ctx.Done():
					return
				}
			}
		}
	}
}

func (b *Bar) isRunning() bool {
	select {
	case <-b.ctx.Done():
		return false
	default:
		return true
	}
}

func (s *bState) completed() bool {
	return s.triggerComplete && s.current == s.total
}

func (s *bState) newStatistics(tw int) decor.Statistics {
	return decor.Statistics{
		AvailableWidth: tw,
		RequestedWidth: s.reqWidth,
		ID:             s.id,
		Total:          s.total,
		Current:        s.current,
		Refill:         s.refill,
		Completed:      s.completed(),
		Aborted:        s.aborted,
	}
}

func unwrap(d decor.Decorator) decor.Decorator {
	if d, ok := d.(decor.Wrapper); ok {
		return unwrap(d.Unwrap())
	}
	return d
}
