package mpb

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"io"
	"iter"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v8/decor"
)

// Bar represents a progress bar.
type Bar struct {
	ctx            context.Context
	cancel         context.CancelFunc
	index          int // used by heap
	priority       int // used by heap
	shutdown       int
	frameCh        chan *renderFrame
	operateState   chan func(*bState)
	container      *Progress
	bs             *bState
	bsOk           chan struct{}
	ewmaDecorators []decor.EwmaDecorator
}

type decorSyncTable [2][]*decor.Sync
type rowProducer func(decor.Statistics) (io.Reader, error)

// bState is actual bar's state.
type bState struct {
	waitFor      *Bar // key for (*pState).queueBars
	id           int
	priority     int
	reqWidth     int
	total0       int64
	total1       int64
	current      int64
	refill       int64
	rowProducers iter.Seq[rowProducer]
	filler       BarFiller
	buffers      [3]*bytes.Buffer
	decorGroups  [2][]decor.Decorator
	trimSpace    bool
	rmOnComplete bool
	aborted      bool
	noPop        bool
}

type renderFrame struct {
	rows         []io.Reader
	err          error
	rmOnComplete bool
	noPop        bool
}

// ProxyReader wraps io.Reader with metrics required for progress tracking.
// Panics if `r` is nil. If `r` is io.ReadCloser then calling Close on the
// returned value will close the underlying reader. If *Bar instance is already
// completed or aborted then (nil, ErrDone[*Bar]) is returned.
func (b *Bar) ProxyReader(r io.Reader) (io.ReadCloser, error) {
	if r == nil {
		panic(errors.New("expected non nil io.Reader"))
	}
	select {
	case <-b.ctx.Done():
		return nil, ErrDone[*Bar]{nil}
	default:
		return newProxyReader(b, r), nil
	}
}

// ProxyReadSeeker wraps io.ReadSeeker with metrics required for progress
// tracking. It is the ReadSeeker counterpart of ProxyReader, intended for
// use cases such as S3 multipart uploads where the AWS SDK requires an
// io.ReadSeeker. Seek calls reset the bar's current value to the new absolute
// offset so the bar stays in sync after retries or rewinds. Panics if `rs` is
// nil. If `rs` is io.ReadCloser then calling Close on the returned value will
// close the underlying reader. If *Bar instance is already completed or aborted
// then (nil, ErrDone[*Bar]) is returned.
func (b *Bar) ProxyReadSeeker(rs io.ReadSeeker) (io.ReadSeekCloser, error) {
	if rs == nil {
		panic(errors.New("expected non nil io.ReadSeeker"))
	}
	select {
	case <-b.ctx.Done():
		return nil, ErrDone[*Bar]{nil}
	default:
		return newProxyReadSeeker(b, rs), nil
	}
}

// ProxyWriter wraps io.Writer with metrics required for progress tracking.
// Panics if `w` is nil. If `w` is io.WriteCloser then calling Close on the
// returned value will close the underlying writer. If *Bar instance is already
// completed or aborted then (nil, ErrDone[*Bar]) is returned.
func (b *Bar) ProxyWriter(w io.Writer) (io.WriteCloser, error) {
	if w == nil {
		panic(errors.New("expected non nil io.Writer"))
	}
	select {
	case <-b.ctx.Done():
		return nil, ErrDone[*Bar]{nil}
	default:
		return newProxyWriter(b, w), nil
	}
}

// ID returns id of the bar.
func (b *Bar) ID() int {
	result := make(chan int, 1)
	select {
	case b.operateState <- func(s *bState) { result <- s.id }:
		return <-result
	case <-b.ctx.Done():
		b.Wait()
		return b.bs.id
	}
}

// Current returns bar's current value, in other words sum of all increments.
func (b *Bar) Current() int64 {
	result := make(chan int64, 1)
	select {
	case b.operateState <- func(s *bState) { result <- s.current }:
		return <-result
	case <-b.ctx.Done():
		b.Wait()
		return b.bs.current
	}
}

// SetRefill sets refill flag with specified amount.
// The underlying BarFiller will change its visual representation, to
// indicate refill event. Refill event may be referred to some retry
// operation for example.
func (b *Bar) SetRefill(amount int64) {
	if amount < 0 {
		return
	}
	select {
	case b.operateState <- func(s *bState) { s.refill = min(amount, s.current) }:
	case <-b.ctx.Done():
	}
}

// SetRefillCurrent sets refill to the current amount.
func (b *Bar) SetRefillCurrent() {
	b.SetRefill(math.MaxInt64)
}

// TraverseDecorators returns a single-use iterator over [int, decor.Decor]
// the underlying bar contains. int represents decorator's group not an order:
// 0=prepend and 1=append. If bar is done i.e. called after (*Bar).Wait method
// then (nil, ErrDone[*Bar]) is returned. Bar's serve/render loop is blocked
// until iteration is done. Attempt to use an iterator more than once will
// lead to a panic.
func (b *Bar) TraverseDecorators() (iter.Seq2[int, decor.Decorator], error) {
	res := make(chan iter.Seq2[int, decor.Decorator], 1)
	select {
	case b.operateState <- func(s *bState) {
		done := make(chan struct{})
		res <- func(yield func(int, decor.Decorator) bool) {
			defer close(done)
			for i, group := range s.decorGroups {
				for _, d := range group {
					if !yield(i, d) {
						return
					}
				}
			}
		}
		<-done
	}:
		return <-res, nil
	case <-b.ctx.Done():
		return nil, ErrDone[*Bar]{nil}
	}
}

// DecoratorAverageAdjust adjusts decorators implementing decor.AverageDecorator
// interface. Call if there is need to set start time after decorators have been
// constructed. Returns ErrDone[*Bar] if called after (*Bar).Wait method.
func (b *Bar) DecoratorAverageAdjust(start time.Time) error {
	it, err := b.TraverseDecorators()
	if err != nil {
		return err
	}
	for _, d := range it {
		if d, ok := unwrap(d).(decor.AverageDecorator); ok {
			d.AverageAdjust(start)
		}
	}
	return nil
}

// EnableTriggerComplete enables triggering complete event for bar
// which was constructed with `total <= 0`. Completion is triggered
// right away on `current == total` state at the moment of call.
func (b *Bar) EnableTriggerComplete() {
	select {
	case b.operateState <- func(s *bState) {
		s.total0 = max(cmp.Or(s.total1, s.total0), 0)
		if s.completed() {
			b.done()
		}
	}:
	case <-b.ctx.Done():
	}
}

// SetTotal sets total to an arbitrary value. If `total` is negative value
// it's equivalent to `(*Bar).SetTotal((*Bar).Current(), bool)` but faster.
// Completion is triggered right away on `forceComplete == true` even in
// `total == 0` case.
func (b *Bar) SetTotal(total int64, forceComplete bool) {
	select {
	case b.operateState <- func(s *bState) {
		if total < 0 {
			s.total1 = s.current
		} else {
			s.total1 = total
		}
		if forceComplete {
			s.total0, s.current = s.total1, s.total1
			if s.completed() {
				b.done()
			}
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
		if s.completed() {
			b.done()
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
		if s.completed() {
			b.done()
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
		s.current += n
		if s.completed() {
			b.done()
		}
	}:
		for _, d := range b.ewmaDecorators {
			d.EwmaUpdate(n, iterDur)
		}
	case <-b.ctx.Done():
	}
}

// EwmaSetCurrent sets progress' current to an arbitrary value and updates
// EWMA based decorators by dur of a single iteration.
func (b *Bar) EwmaSetCurrent(current int64, iterDur time.Duration) {
	if current < 0 {
		return
	}
	ch := make(chan int64, 1)
	select {
	case b.operateState <- func(s *bState) {
		n := current - s.current
		s.current += n
		if s.completed() {
			b.done()
		}
		ch <- n
	}:
		n := <-ch
		for _, d := range b.ewmaDecorators {
			d.EwmaUpdate(n, iterDur)
		}
	case <-b.ctx.Done():
	}
}

// SetPriority changes bar's order among multiple bars. Zero is highest
// priority, i.e. bar will be on top. If you don't need to set priority
// dynamically, better use BarPriority option.
func (b *Bar) SetPriority(priority int) {
	b.container.UpdateBarPriority(b, priority, false)
}

// Abort interrupts bar's running goroutine. Abort won't be engaged
// if bar is already in complete state. If drop is true bar will be
// removed as well.
func (b *Bar) Abort(drop bool) {
	select {
	case b.operateState <- func(s *bState) {
		if s.aborted || s.completed() {
			return
		}
		s.aborted = true
		s.rmOnComplete = drop
		b.done()
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
	case <-b.ctx.Done():
		b.Wait()
		return b.bs.aborted
	}
}

// Completed reports whether the bar is in completed state.
func (b *Bar) Completed() bool {
	result := make(chan bool, 1)
	select {
	case b.operateState <- func(s *bState) { result <- s.completed() }:
		return <-result
	case <-b.ctx.Done():
		b.Wait()
		return b.bs.completed()
	}
}

// AbortedOrCompleted reports whether a bar is in aborted or completed state.
// Faster and atomic version of `(*Bar).Aborted() || (*Bar).Completed()`.
func (b *Bar) AbortedOrCompleted() bool {
	result := make(chan bool, 1)
	select {
	case b.operateState <- func(s *bState) { result <- s.aborted || s.completed() }:
		return <-result
	case <-b.ctx.Done():
		b.Wait()
		return b.bs.aborted || b.bs.completed()
	}
}

// Wait blocks until bar is completed or aborted.
func (b *Bar) Wait() {
	<-b.bsOk
}

func (b *Bar) serve(bs *bState) {
	defer func() {
		b.bs = bs
		close(b.bsOk)
	}()
	if bs.waitFor != nil {
		<-bs.waitFor.ctx.Done()
		bs.waitFor = nil
	}
	for {
		select {
		case op := <-b.operateState:
			op(bs)
		case <-b.ctx.Done():
			if bs.aborted {
				return
			}
			bs.aborted = !bs.completed()
			return
		}
	}
}

func (b *Bar) render(tw int) {
	fn := func(s *bState) {
		frame := new(renderFrame)
		stat := s.newStatistics(tw)
		for p := range s.rowProducers {
			r, err := p(stat)
			if err != nil && frame.err == nil {
				frame.err = err
				// need to iterate all rowProducer to avoid deadlocks
				// because bar's rowProducer can be either first or last
				continue
			}
			frame.rows = append(frame.rows, r)
		}
		if s.aborted || s.completed() {
			frame.rmOnComplete = s.rmOnComplete
			frame.noPop = s.noPop
			// post increment makes sure OnComplete decorators are rendered
			b.shutdown++
		}
		b.frameCh <- frame
	}
	select {
	case b.operateState <- fn:
	case <-b.ctx.Done():
		b.Wait()
		fn(b.bs)
	}
}

func (b *Bar) wSyncTable() decorSyncTable {
	result := make(chan decorSyncTable, 1)
	select {
	case b.operateState <- func(s *bState) { result <- s.wSyncTable() }:
		return <-result
	case <-b.ctx.Done():
		b.Wait()
		return b.bs.wSyncTable()
	}
}

func (b *Bar) done() {
	if b.container.noRenderMode {
		b.cancel()
	} else {
		// Technically this call isn't required, but if refresh rate is set to
		// one hour for example and bar completes within a few minutes p.Wait()
		// will wait for one hour. This call helps to avoid unnecessary waiting.
		go b.tryEarlyRefresh()
	}
}

func (b *Bar) tryEarlyRefresh() {
	otherRunning := make(chan struct{})
	yield := func(bar *Bar) bool {
		if b == bar || bar.AbortedOrCompleted() {
			return true // continue traverse
		}
		close(otherRunning)
		return false // stop traverse
	}
	if err := b.container.iterateBars(yield); err == nil {
		select {
		case <-otherRunning:
		default:
			// b is the last bar leaving so it should switch tv off
			for {
				select {
				case b.container.renderReq <- time.Now():
				case <-b.ctx.Done():
					return
				}
			}
		}
	}
}

// draw is actual bar's rowProducer.
// It needs copy of decor.Statistics because it modifies stat.AvailableWidth.
// Each decorator gets its own copy of decor.Statistics with updated AvailableWidth.
func (s *bState) draw(stat decor.Statistics) (row io.Reader, err error) {
	defer func() {
		if err != nil {
			for _, buf := range s.buffers {
				buf.Reset()
			}
		}
	}()
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
		err = decorFiller(buf, s.decorGroups[i])
		if err != nil {
			return
		}
	}

	if s.trimSpace || stat.AvailableWidth < 2 {
		err = s.filler.Fill(s.buffers[0], stat)
		if err != nil {
			return
		}
		return io.MultiReader(
			s.buffers[1],
			s.buffers[0],
			s.buffers[2],
			strings.NewReader("\n"),
		), nil
	}

	stat.AvailableWidth -= 2
	err = s.filler.Fill(s.buffers[0], stat)
	if err != nil {
		return
	}
	return io.MultiReader(
		s.buffers[1],
		strings.NewReader(" "),
		s.buffers[0],
		strings.NewReader(" "),
		s.buffers[2],
		strings.NewReader("\n"),
	), nil
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
		table[i] = slices.Clip(row[start:])
		start = len(row)
	}
	return table
}

func (s *bState) isQueue() bool {
	if s.waitFor == nil {
		return false
	}
	select {
	case <-s.waitFor.ctx.Done():
		s.waitFor = nil
		return false
	default:
		return true
	}
}

func (s *bState) completed() bool {
	return s.total0 >= 0 && s.current >= s.total0
}

func (s *bState) newStatistics(tw int) decor.Statistics {
	return decor.Statistics{
		AvailableWidth: tw,
		RequestedWidth: s.reqWidth,
		ID:             s.id,
		Total:          max(cmp.Or(s.total1, s.total0), 0),
		Current:        s.current,
		Refill:         s.refill,
		Completed:      s.completed(),
		Aborted:        s.aborted,
	}
}

func decoratorOnShutdown(group []decor.Decorator) {
	for _, d := range group {
		if d, ok := unwrap(d).(decor.ShutdownListener); ok {
			d.OnShutdown()
		}
	}
}

func unwrap(d decor.Decorator) decor.Decorator {
	if d, ok := d.(decor.Wrapper); ok {
		return unwrap(d.Unwrap())
	}
	return d
}

// makeRowProducers converts fillers to iter.Seq[rowProducer].
// Each BarFiller suppose to write one line only but it is not enforced.
// If BarFiller writes more than one line then whole output is going to be
// corrupted.
func makeRowProducers(base rowProducer, top bool, fillers ...BarFiller) iter.Seq[rowProducer] {
	producers := []rowProducer{base}
	for _, filler := range fillers {
		if filler == nil {
			continue
		}
		if f, ok := filler.(BarFillerFunc); ok && f == nil {
			continue
		}
		buf := new(bytes.Buffer)
		producers = append(producers, func(stat decor.Statistics) (io.Reader, error) {
			err := filler.Fill(buf, stat)
			if err != nil {
				buf.Reset()
				return nil, err
			}
			return buf, nil
		})
	}
	if top {
		slices.Reverse(producers)
	}
	producers = slices.Clip(producers)
	// this one is going to be iterated on each (*Bar).render
	return func(yield func(rowProducer) bool) {
		for _, p := range producers {
			if !yield(p) {
				return
			}
		}
	}
}
