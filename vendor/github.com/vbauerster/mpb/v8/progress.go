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
	"slices"
	"sync"
	"time"

	"github.com/vbauerster/cupwriter"
	"github.com/vbauerster/mpb/v8/decor"
)

const defaultRefreshRate = 150 * time.Millisecond
const defaultHmQueueLength = 64
const defaultWidth = 80

// ErrDone represents use after `(*Progress).Wait()` error.
var ErrDone = fmt.Errorf("%T instance can't be reused after %[1]T.Wait()", (*Progress)(nil))

// Progress represents a container that renders one or more progress bars.
type Progress struct {
	// Render error if any, to be inspected after (*Progress).Wait call only.
	Error error

	ctx          context.Context
	cancel       context.CancelCauseFunc
	pwg          *sync.WaitGroup
	bwg          *sync.WaitGroup
	operateState chan func(*pState)
	interceptIO  chan func(io.Writer)
	renderReq    chan time.Time
	done         chan struct{}
	noRenderMode bool
}

// pState holds bars in its priorityQueue, it gets passed to (*Progress).serve monitor goroutine.
type pState struct {
	hm          heapManager
	idCount     int
	popPriority int

	// following are provided/overrode by user
	uwg              *sync.WaitGroup
	hmQueueLen       int
	reqWidth         int
	refreshRate      time.Duration
	delayRC          <-chan any
	manualRC         <-chan any
	shutdownNotifier chan any
	depleteHeap      chan<- *Bar
	queueBars        map[*Bar]*Bar
	output           io.Writer
	debugOut         io.Writer
	cwriter          ConsoleWriter
	popCompleted     bool
	autoRefresh      bool
	forceTTY         bool
	hasUnrendered    bool
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
	ctx, cancel := context.WithCancelCause(ctx)

	s := &pState{
		popPriority: math.MinInt32,
		queueBars:   make(map[*Bar]*Bar),
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

	if s.cwriter == nil {
		s.cwriter = cupwriter.New(s.output, s.forceTTY)
	}

	p := &Progress{
		ctx:          ctx,
		cancel:       cancel,
		pwg:          new(sync.WaitGroup),
		bwg:          new(sync.WaitGroup),
		operateState: make(chan func(*pState)),
		interceptIO:  make(chan func(io.Writer)),
		done:         make(chan struct{}),
	}

	var refreshStrategy func(*Progress, *pState)
	switch {
	case s.manualRC != nil:
		p.renderReq = make(chan time.Time)
		refreshStrategy = (*Progress).manualRefreshListener
	case s.autoRefresh || s.cwriter.IsTerminal():
		p.renderReq = make(chan time.Time)
		refreshStrategy = (*Progress).autoRefreshListener
	default:
		p.noRenderMode = true
		refreshStrategy = (*Progress).nopRefreshListener
	}

	p.pwg.Add(3)
	s.hm = make(heapManager, cmp.Or(s.hmQueueLen, defaultHmQueueLength))
	go s.hm.run(p.pwg, s.shutdownNotifier, s.depleteHeap)
	go p.serve(s)
	go refreshStrategy(p, s)
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

// New creates a bar from provided BarFillerBuilder interface.
// Default implementations are:
//
//	BarStyle()
//	SpinnerStyle()
//	NopStyle()
func (p *Progress) New(total int64, builder BarFillerBuilder, options ...BarOption) *Bar {
	if builder == nil {
		builder = NopStyle()
	}
	bar, err := p.Add(total, builder.Build(), options...)
	if err != nil {
		panic(err)
	}
	return bar
}

// Add creates a bar which renders itself by provided BarFiller.
// If `total <= 0` triggering complete event by increment methods is disabled.
// If called after `(*Progress).Wait()` then `(nil, ErrDone)` is returned.
func (p *Progress) Add(total int64, filler BarFiller, options ...BarOption) (*Bar, error) {
	if filler == nil {
		filler = NopStyle().Build()
	} else if f, ok := filler.(BarFillerFunc); ok && f == nil {
		filler = NopStyle().Build()
	}
	ch := make(chan *Bar, 1)
	select {
	case p.operateState <- func(s *pState) {
		bs := s.makeBarState(total, filler, options...)
		bar := p.makeBar(bs)
		if bs.isQueue() {
			s.queueBars[bs.waitFor] = bar
		} else if !p.noRenderMode {
			s.hm.push(bar, true, nil)
		}
		p.bwg.Go(func() {
			bar.serve(bs)
			for _, group := range bs.decorGroups {
				p.bwg.Go(func() {
					decoratorOnShutdown(group)
				})
			}
		})
		ch <- bar
	}:
		return <-ch, nil
	case <-p.done:
		return nil, ErrDone
	}
}

func (p *Progress) makeBar(bs *bState) *Bar {
	ctx, cancel := context.WithCancelCause(p.ctx)
	bar := &Bar{
		ctx:          ctx,
		cancel:       cancel,
		priority:     bs.priority,
		frameCh:      make(chan *renderFrame, 1),
		operateState: make(chan func(*bState)),
		bsOk:         make(chan struct{}),
		container:    p,
	}
	if p.noRenderMode {
		return bar
	}
	for _, group := range bs.decorGroups {
		for _, d := range group {
			if d, ok := unwrap(d).(decor.EwmaDecorator); ok {
				bar.ewmaDecorators = append(bar.ewmaDecorators, d)
			}
		}
	}
	return bar
}

// blocks until iteration is done
func (p *Progress) iterateBars(yield func(*Bar) bool) error {
	seqCh := make(chan iter.Seq[*Bar], 1)
	select {
	case p.operateState <- func(s *pState) { s.hm.iter(seqCh) }:
		for b := range <-seqCh {
			if !yield(b) {
				break
			}
		}
		return nil
	case <-p.done:
		return ErrDone
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

// Wait waits for all bars to complete and then shutdowns the container.
// There is no way to reuse `*Progress` instance after this method has been called.
func (p *Progress) Wait() {
	p.bwg.Wait()
	p.Shutdown()
}

// Shutdown cancels any running bar immediately and then shutdowns `*Progress`
// instance. Normally this method shouldn't be called unless you know what you
// are doing. Proper way to shutdown is to call `(*Progress).Wait()` instead.
func (p *Progress) Shutdown() {
	p.cancel(nil)
	p.pwg.Wait()
}

func (p *Progress) serve(s *pState) {
	defer func() {
		if s.uwg != nil {
			s.uwg.Wait() // wait for user wg
		}
		p.bwg.Wait()
		close(s.hm)
		close(s.shutdownNotifier)
		p.pwg.Done()
	}()

	var cw ConsoleWriter
	if s.delayRC != nil {
		cw, s.cwriter = s.cwriter, cupwriter.New(io.Discard, false)
	}

	for {
		select {
		case <-s.delayRC:
			s.cwriter = cw
			s.delayRC = nil
		case op := <-p.operateState:
			op(s)
		case fn := <-p.interceptIO:
			fn(s.cwriter)
		case <-p.renderReq:
			s.hasUnrendered = false
			err := s.render()
			if err != nil {
				p.cancel(err)
				// refreshStrategy goroutine is sending to p.renderReq unbuffered chan
				// without any select therefore p.renderReq must be depleted here
				// otherwise refreshStrategy goroutine may block and leak.
				for {
					select {
					case <-p.renderReq:
					case <-p.done:
						_, _ = fmt.Fprintln(s.debugOut, err.Error())
						p.Error = err
						return
					}
				}
			}
		case <-p.done:
			if !p.noRenderMode && s.hasUnrendered {
				err := s.render()
				if err != nil {
					_, _ = fmt.Fprintln(s.debugOut, err.Error())
					p.Error = err
				}
			}
			return
		}
	}
}

func (p *Progress) autoRefreshListener(s *pState) {
	defer p.pwg.Done()
	ticker := time.NewTicker(cmp.Or(s.refreshRate, defaultRefreshRate))
	defer ticker.Stop()
	for {
		select {
		case t := <-ticker.C:
			p.renderReq <- t
		case <-p.ctx.Done():
			close(p.done)
			return
		}
	}
}

func (p *Progress) manualRefreshListener(s *pState) {
	defer p.pwg.Done()
	for {
		select {
		case x := <-s.manualRC:
			if t, ok := x.(time.Time); ok {
				p.renderReq <- t
			} else {
				p.renderReq <- time.Now()
			}
		case <-p.ctx.Done():
			close(p.done)
			return
		}
	}
}

func (p *Progress) nopRefreshListener(_ *pState) {
	defer p.pwg.Done()
	<-p.ctx.Done()
	close(p.done)
}

func (s *pState) render() (err error) {
	s.hm.sync()

	var width, height int
	if s.cwriter.IsTerminal() {
		width, height, err = s.cwriter.GetTermSize()
		if err != nil {
			return err
		}
	} else {
		width = cmp.Or(s.reqWidth, defaultWidth)
		height = width*3/2 + 1
	}

	offload := make(chan heapRequest)
	defer close(offload)
	var total, popCount int
	var rows [][]io.Reader

	for b := range s.hm.render(width, offload) {
		frame := <-b.frameCh
		if frame.err != nil {
			b.cancel(frame.err)
			return frame.err // b.frameCh is buffered it's ok to return here
		}
		var discarded int
		for _, row := range slices.Backward(frame.rows) {
			if total < height {
				total++
			} else {
				_, _ = io.Copy(io.Discard, row)
				discarded++
			}
		}
		rows = append(rows, frame.rows)

		switch b.shutdown {
		case 1:
			b.cancel(nil)
			if q, ok := s.queueBars[b]; ok {
				delete(s.queueBars, b)
				q.priority = b.priority
				s.hm.push(q, true, offload)
				continue
			}
			if s.popCompleted && !frame.noPop {
				b.priority = s.popPriority
				s.popPriority++
				frame.rmOnComplete = false
			}
			if frame.rmOnComplete {
				s.hasUnrendered = true
				continue
			}
		case 2:
			if s.popCompleted && !frame.noPop {
				popCount += len(frame.rows) - discarded
				continue
			}
		}

		s.hm.push(b, false, offload)
	}

	for _, row := range slices.Backward(rows) {
		for _, r := range row {
			n, err := s.cwriter.ReadFrom(r)
			if err != nil {
				return err
			}
			if n == 0 {
				total--
			}
		}
	}

	return s.cwriter.Flush(total - popCount)
}

func (s *pState) makeBarState(total int64, filler BarFiller, options ...BarOption) *bState {
	bs := &bState{
		id:       s.idCount,
		priority: s.idCount,
		reqWidth: s.reqWidth,
		total0:   cmp.Or(total, -1),
		filler:   filler,
	}

	bs.extender = func(base rowProducer) iter.Seq[rowProducer] {
		return slices.Values([]rowProducer{base})
	}

	for _, opt := range options {
		if opt != nil {
			opt(bs)
		}
	}

	bs.buffers[0] = bytes.NewBuffer(make([]byte, 0, 256)) // filler
	bs.buffers[1] = bytes.NewBuffer(make([]byte, 0, 128)) // prepend
	bs.buffers[2] = bytes.NewBuffer(make([]byte, 0, 128)) // append

	s.idCount++
	return bs
}
