package mpb

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"iter"
	"math"
	"os"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v8/cwriter"
	"github.com/vbauerster/mpb/v8/decor"
)

const defaultRefreshRate = 150 * time.Millisecond
const defaultHmQueueLength = 64

// ErrDone represents use after `(*Progress).Wait()` error.
var ErrDone = fmt.Errorf("%T instance can't be reused after %[1]T.Wait()", (*Progress)(nil))

// Progress represents a container that renders one or more progress bars.
type Progress struct {
	pwg, bwg     *sync.WaitGroup
	operateState chan func(*pState)
	interceptIO  chan func(io.Writer)
	done         <-chan struct{}
	ctx          context.Context
	cancel       func()
}

type queueBar struct {
	state *bState
	bar   *Bar
}

// pState holds bars in its priorityQueue, it gets passed to (*Progress).serve monitor goroutine.
type pState struct {
	hm          heapManager
	renderReq   chan time.Time
	idCount     int
	popPriority int

	// following are provided/overrode by user
	hmQueueLen       int
	reqWidth         int
	refreshRate      time.Duration
	delayRC          <-chan any
	manualRC         <-chan any
	shutdownNotifier chan any
	handOverBarHeap  chan<- []*Bar
	queueBars        map[*Bar]*queueBar
	output           io.Writer
	debugOut         io.Writer
	uwg              *sync.WaitGroup
	popCompleted     bool
	autoRefresh      bool
	rmOnComplete     bool
}

// New creates new Progress container instance. It's not possible to
// reuse instance after `(*Progress).Wait` method has been called.
func New(options ...ContainerOption) *Progress {
	return NewWithContext(context.Background(), options...)
}

// NewWithContext creates new Progress container instance with provided
// context. It's not possible to reuse instance after `(*Progress).Wait`
// method has been called.
func NewWithContext(ctx context.Context, options ...ContainerOption) *Progress {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)

	s := &pState{
		hmQueueLen:  defaultHmQueueLength,
		renderReq:   make(chan time.Time),
		popPriority: math.MinInt32,
		refreshRate: defaultRefreshRate,
		queueBars:   make(map[*Bar]*queueBar),
		output:      os.Stdout,
		debugOut:    io.Discard,
	}

	for _, opt := range options {
		if opt != nil {
			opt(s)
		}
	}

	if s.shutdownNotifier == nil {
		s.shutdownNotifier = make(chan any)
	}

	s.hm = make(heapManager, s.hmQueueLen)

	p := &Progress{
		pwg:          new(sync.WaitGroup),
		bwg:          new(sync.WaitGroup),
		operateState: make(chan func(*pState)),
		interceptIO:  make(chan func(io.Writer)),
		ctx:          ctx,
		cancel:       cancel,
	}

	cw := cwriter.New(s.output)
	switch {
	case s.manualRC != nil:
		done := make(chan struct{})
		p.done = done
		s.autoRefresh = false
		go s.manualRefreshListener(ctx, done)
	case s.autoRefresh || cw.IsTerminal():
		done := make(chan struct{})
		p.done = done
		s.autoRefresh = true
		go s.autoRefreshListener(ctx, done)
	default:
		p.done = ctx.Done()
		s.autoRefresh = false
	}

	p.pwg.Add(2)
	go s.hm.run(p.pwg, s.shutdownNotifier, s.handOverBarHeap)
	go p.serve(s, cw)
	return p
}

// AddBar creates a bar with default bar filler.
func (p *Progress) AddBar(total int64, options ...BarOption) *Bar {
	return p.New(total, barStyleComposer, options...)
}

// AddSpinner creates a bar with default spinner filler.
func (p *Progress) AddSpinner(total int64, options ...BarOption) *Bar {
	return p.New(total, spinnerStyleComposer, options...)
}

// New creates a bar by calling `Build` method on provided `BarFillerBuilder`.
func (p *Progress) New(total int64, builder BarFillerBuilder, options ...BarOption) *Bar {
	if builder == nil {
		return p.MustAdd(total, nil, options...)
	}
	return p.MustAdd(total, builder.Build(), options...)
}

// MustAdd creates a bar which renders itself by provided BarFiller.
// If `total <= 0` triggering complete event by increment methods is
// disabled. Panics if called after `(*Progress).Wait()`.
func (p *Progress) MustAdd(total int64, filler BarFiller, options ...BarOption) *Bar {
	bar, err := p.Add(total, filler, options...)
	if err != nil {
		panic(err)
	}
	return bar
}

// Add creates a bar which renders itself by provided BarFiller.
// If `total <= 0` triggering complete event by increment methods
// is disabled. If called after `(*Progress).Wait()` then
// `(nil, ErrDone)` is returned.
func (p *Progress) Add(total int64, filler BarFiller, options ...BarOption) (*Bar, error) {
	if filler == nil {
		filler = NopStyle().Build()
	} else if f, ok := filler.(BarFillerFunc); ok && f == nil {
		filler = NopStyle().Build()
	}
	ch := make(chan *Bar, 1)
	select {
	case p.operateState <- func(ps *pState) {
		p.bwg.Add(1)
		bs := ps.makeBarState(total, filler, options...)
		bar := p.makeBar(bs.priority)
		if bs.waitBar != nil {
			ps.queueBars[bs.waitBar] = &queueBar{bs, bar}
		} else {
			go bar.serve(bs)
			ps.hm.push(bar, true)
		}
		ch <- bar
	}:
		return <-ch, nil
	case <-p.done:
		return nil, ErrDone
	}
}

func (p *Progress) makeBar(priority int) *Bar {
	ctx, cancel := context.WithCancel(p.ctx)

	bar := &Bar{
		priority:     priority,
		frameCh:      make(chan *renderFrame, 1),
		operateState: make(chan func(*bState)),
		bsOk:         make(chan struct{}),
		container:    p,
		ctx:          ctx,
		cancel:       cancel,
	}

	return bar
}

// blocks until iteration is done
func (p *Progress) iterateBars(yield func(*Bar) bool) (ok bool) {
	seqCh := make(chan iter.Seq[*Bar], 1)
	select {
	case p.operateState <- func(s *pState) { s.hm.iter(seqCh) }:
		for b := range <-seqCh {
			if !yield(b) {
				break
			}
		}
		return true
	case <-p.done:
		return false
	}
}

// UpdateBarPriority either immediately or lazy.
// With lazy flag order is updated after the next refresh cycle.
// If you don't care about laziness just use `(*Bar).SetPriority(int)`.
func (p *Progress) UpdateBarPriority(b *Bar, priority int, lazy bool) {
	if b == nil {
		return
	}
	select {
	case p.operateState <- func(s *pState) { s.hm.fix(b, priority, lazy) }:
	case <-p.done:
	}
}

// Write is implementation of io.Writer.
// Writing to `*Progress` will print lines above a running bar.
// Writes aren't flushed immediately, but at next refresh cycle.
// If called after `(*Progress).Wait()` then `(0, ErrDone)` is returned.
func (p *Progress) Write(b []byte) (int, error) {
	type result struct {
		n   int
		err error
	}
	ch := make(chan result, 1)
	select {
	case p.interceptIO <- func(w io.Writer) {
		n, err := w.Write(b)
		ch <- result{n, err}
	}:
		res := <-ch
		return res.n, res.err
	case <-p.done:
		return 0, ErrDone
	}
}

// Wait waits for all bars to complete and finally shutdowns container. After
// this method has been called, there is no way to reuse `*Progress` instance.
func (p *Progress) Wait() {
	p.bwg.Wait()
	p.Shutdown()
}

// Shutdown cancels any running bar immediately and then shutdowns `*Progress`
// instance. Normally this method shouldn't be called unless you know what you
// are doing. Proper way to shutdown is to call `(*Progress).Wait()` instead.
func (p *Progress) Shutdown() {
	p.cancel()
	p.pwg.Wait()
}

func (p *Progress) serve(s *pState, cw *cwriter.Writer) {
	defer func() {
		if s.uwg != nil {
			s.uwg.Wait() // wait for user wg
		}
		p.bwg.Wait()
		close(s.hm)
		close(s.shutdownNotifier)
		p.pwg.Done()
	}()

	var dw *cwriter.Writer
	if s.delayRC != nil {
		dw = cwriter.New(io.Discard)
	} else {
		dw = cw
	}

	for {
		select {
		case <-s.delayRC:
			dw = cw
			s.delayRC = nil
		case op := <-p.operateState:
			op(s)
		case fn := <-p.interceptIO:
			fn(cw)
		case <-s.renderReq:
			err := s.render(dw)
			if err != nil {
				p.cancel()
				// (*pState).(autoRefreshListener|manualRefreshListener) may block
				// if not depleting s.renderReq
				for {
					select {
					case <-s.renderReq:
					case <-p.done:
						_, _ = fmt.Fprintln(s.debugOut, err.Error())
						return
					}
				}
			}
		case <-p.done:
			if s.autoRefresh && s.rmOnComplete {
				if err := s.render(cw); err != nil {
					_, _ = fmt.Fprintln(s.debugOut, err.Error())
				}
			}
			return
		}
	}
}

func (s *pState) autoRefreshListener(ctx context.Context, done chan struct{}) {
	ticker := time.NewTicker(s.refreshRate)
	defer ticker.Stop()
	for {
		select {
		case t := <-ticker.C:
			s.renderReq <- t
		case <-ctx.Done():
			close(done)
			return
		}
	}
}

func (s *pState) manualRefreshListener(ctx context.Context, done chan struct{}) {
	for {
		select {
		case x := <-s.manualRC:
			if t, ok := x.(time.Time); ok {
				s.renderReq <- t
			} else {
				s.renderReq <- time.Now()
			}
		case <-ctx.Done():
			close(done)
			return
		}
	}
}

func (s *pState) render(cw *cwriter.Writer) (err error) {
	s.hm.sync()

	var width, height int
	if cw.IsTerminal() {
		width, height, err = cw.GetTermSize()
		if err != nil {
			return err
		}
	} else {
		width = cmp.Or(s.reqWidth, 80)
		height = width
	}

	return s.flush(cw, height, s.hm.render(width))
}

func (s *pState) flush(cw *cwriter.Writer, height int, seq iter.Seq[*Bar]) error {
	var total, popCount int
	var rows [][]io.Reader

	s.rmOnComplete = false

	for b := range seq {
		frame := <-b.frameCh
		if frame.err != nil {
			b.cancel()
			s.hm.push(b, false)
			return frame.err // b.frameCh is buffered it's ok to return here
		}
		var discarded int
		for i := len(frame.rows) - 1; i >= 0; i-- {
			if total < height {
				total++
			} else {
				_, _ = io.Copy(io.Discard, frame.rows[i]) // Found IsInBounds
				discarded++
			}
		}
		rows = append(rows, frame.rows)

		switch frame.shutdown {
		case 1:
			if qb, ok := s.queueBars[b]; ok {
				delete(s.queueBars, b)
				qb.bar.priority = b.priority
				go qb.bar.serve(qb.state)
				s.hm.push(qb.bar, true)
			} else {
				switch {
				case s.popCompleted && !frame.noPop:
					b.priority = s.popPriority
					s.popPriority++
					fallthrough
				case !frame.rmOnComplete:
					s.hm.push(b, false)
				}
				s.rmOnComplete = s.rmOnComplete || frame.rmOnComplete
			}
			b.cancel()
		case 2:
			if s.popCompleted && !frame.noPop {
				popCount += len(frame.rows) - discarded
				continue
			}
			fallthrough
		default:
			s.hm.push(b, false)
		}
	}

	for i := len(rows) - 1; i >= 0; i-- {
		for _, r := range rows[i] {
			_, err := cw.ReadFrom(r)
			if err != nil {
				return err
			}
		}
	}

	return cw.Flush(total - popCount)
}

func (s *pState) makeBarState(total int64, filler BarFiller, options ...BarOption) *bState {
	bs := &bState{
		id:              s.idCount,
		priority:        s.idCount,
		reqWidth:        s.reqWidth,
		total:           total,
		filler:          filler,
		renderReq:       s.renderReq,
		triggerComplete: total > 0,
		autoRefresh:     s.autoRefresh,
	}

	bs.extender = func(_ decor.Statistics, rows ...io.Reader) ([]io.Reader, error) {
		return rows, nil
	}

	for _, opt := range options {
		if opt != nil {
			opt(bs)
		}
	}

	for _, group := range bs.decorGroups {
		for _, d := range group {
			if d, ok := unwrap(d).(decor.EwmaDecorator); ok {
				bs.ewmaDecorators = append(bs.ewmaDecorators, d)
			}
		}
	}

	bs.buffers[0] = bytes.NewBuffer(make([]byte, 0, 256)) // filler
	bs.buffers[1] = bytes.NewBuffer(make([]byte, 0, 128)) // prepend
	bs.buffers[2] = bytes.NewBuffer(make([]byte, 0, 128)) // append

	s.idCount++
	return bs
}
