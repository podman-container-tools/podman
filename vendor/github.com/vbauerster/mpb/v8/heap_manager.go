package mpb

import (
	"container/heap"
	"errors"
	"iter"
	"sync"

	"github.com/vbauerster/mpb/v8/decor"
)

type heapManager chan heapRequest

type heapCmd int

const (
	h_sync heapCmd = iota
	h_push
	h_render
	h_iter
	h_fix
)

type heapRequest struct {
	cmd  heapCmd
	data any
}

type pushData struct {
	bar  *Bar
	sync bool
}

type renderData struct {
	width   int
	seqCh   chan<- iter.Seq[*Bar]
	offload <-chan heapRequest
}

type fixData struct {
	bar      *Bar
	priority int
	lazy     bool
}

func (m heapManager) run(pwg *sync.WaitGroup, shutdown <-chan any, depleteHeap chan<- *Bar) {
	var bHeap barHeap
	var sync bool
	var prevLen int
	var pMatrix map[int][]*decor.Sync
	var aMatrix map[int][]*decor.Sync

	defer func() {
		if depleteHeap != nil {
			for bHeap.Len() != 0 {
				depleteHeap <- heap.Pop(&bHeap).(*Bar)
			}
			close(depleteHeap)
		}
		pwg.Done()
	}()

	for req := range m {
		switch req.cmd {
		case h_sync:
			if sync || prevLen != bHeap.Len() {
				pMatrix = make(map[int][]*decor.Sync)
				aMatrix = make(map[int][]*decor.Sync)
				for _, b := range bHeap {
					table := b.wSyncTable()
					for i, s := range table[0] {
						pMatrix[i] = append(pMatrix[i], s)
					}
					for i, s := range table[1] {
						aMatrix[i] = append(aMatrix[i], s)
					}
				}
				sync, prevLen = false, bHeap.Len()
			}
			syncWidth(pMatrix, shutdown)
			syncWidth(aMatrix, shutdown)
		case h_push:
			data := req.data.(pushData)
			heap.Push(&bHeap, data.bar)
			sync = sync || data.sync
		case h_render:
			var pushQ []heapRequest
			data := req.data.(renderData)
			for _, b := range bHeap {
				go b.render(data.width)
			}
			data.seqCh <- func(yield func(*Bar) bool) {
				for bHeap.Len() != 0 {
					if !yield(heap.Pop(&bHeap).(*Bar)) {
						return
					}
				}
			}
			for req := range data.offload {
				pushQ = append(pushQ, req)
			}
			for _, req := range pushQ {
				data := req.data.(pushData)
				heap.Push(&bHeap, data.bar)
				sync = sync || data.sync
			}
		case h_iter:
			seqCh := req.data.(chan<- iter.Seq[*Bar])
			done := make(chan struct{})
			seqCh <- func(yield func(*Bar) bool) {
				defer close(done)
				for _, b := range bHeap {
					if !yield(b) {
						return
					}
				}
			}
			<-done
		case h_fix:
			data := req.data.(fixData)
			if data.bar.index < 0 {
				break
			}
			data.bar.priority = data.priority
			if !data.lazy {
				heap.Fix(&bHeap, data.bar.index)
			}
		}
	}
}

func (m heapManager) sync() {
	m <- heapRequest{cmd: h_sync}
}

func (m heapManager) push(bar *Bar, sync bool, offload chan<- heapRequest) {
	req := heapRequest{cmd: h_push, data: pushData{
		bar:  bar,
		sync: sync,
	}}
	select {
	case m <- req:
	default:
		if offload != nil {
			offload <- req
		} else {
			bar.container.bwg.Go(func() {
				m <- req
			})
		}
	}
}

func (m heapManager) render(width int, offload <-chan heapRequest) iter.Seq[*Bar] {
	if offload == nil {
		panic(errors.New("expected non nil offload chan heapRequest"))
	}
	seqCh := make(chan iter.Seq[*Bar], 1)
	m <- heapRequest{cmd: h_render, data: renderData{
		width:   width,
		seqCh:   seqCh,
		offload: offload,
	}}
	return <-seqCh
}

func (m heapManager) iter(seqCh chan<- iter.Seq[*Bar]) {
	m <- heapRequest{cmd: h_iter, data: seqCh}
}

func (m heapManager) fix(b *Bar, priority int, lazy bool) {
	data := fixData{b, priority, lazy}
	m <- heapRequest{cmd: h_fix, data: data}
}

func syncWidth(matrix map[int][]*decor.Sync, done <-chan any) {
	for _, column := range matrix {
		go maxWidthDistributor(column, done)
	}
}

func maxWidthDistributor(column []*decor.Sync, done <-chan any) {
	var maxWidth int
loop:
	for _, s := range column {
		select {
		case w := <-s.Tx:
			if w > maxWidth {
				maxWidth = w
			}
		case <-done:
			break loop
		}
	}
	for _, s := range column {
		s.Rx <- maxWidth
	}
}
