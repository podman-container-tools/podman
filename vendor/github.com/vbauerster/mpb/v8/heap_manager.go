package mpb

import (
	"container/heap"
	"iter"
	"slices"
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
	width int
	seqCh chan<- iter.Seq[*Bar]
}

type fixData struct {
	bar      *Bar
	priority int
	lazy     bool
}

func (m heapManager) run(pwg *sync.WaitGroup, shutdown <-chan any, handOverBarHeap chan<- []*Bar) {
	var bHeap barHeap
	var sync bool
	var prevLen int
	var pMatrix map[int][]*decor.Sync
	var aMatrix map[int][]*decor.Sync

	defer func() {
		if handOverBarHeap != nil {
			ordered := make([]*Bar, 0, bHeap.Len())
			for bHeap.Len() != 0 {
				ordered = append(ordered, heap.Pop(&bHeap).(*Bar))
			}
			handOverBarHeap <- ordered
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
			data := req.data.(renderData)
			for _, b := range bHeap {
				go b.render(data.width)
			}
			ordered := make([]*Bar, 0, bHeap.Len())
			for bHeap.Len() != 0 {
				ordered = append(ordered, heap.Pop(&bHeap).(*Bar))
			}
			data.seqCh <- slices.Values(ordered)
		case h_iter:
			seqCh := req.data.(chan<- iter.Seq[*Bar])
			done := make(chan struct{})
			seqCh <- func(yield func(*Bar) bool) {
				defer close(done)
				for _, b := range bHeap {
					if !yield(b) {
						break
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

func (m heapManager) push(b *Bar, sync bool) {
	data := pushData{b, sync}
	m <- heapRequest{cmd: h_push, data: data}
}

func (m heapManager) render(width int) iter.Seq[*Bar] {
	seqCh := make(chan iter.Seq[*Bar], 1)
	m <- heapRequest{cmd: h_render, data: renderData{
		width: width,
		seqCh: seqCh,
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
	for _, s := range column {
		select {
		case w := <-s.Tx:
			if w > maxWidth {
				maxWidth = w
			}
		case <-done:
			return
		}
	}
	for _, s := range column {
		s.Rx <- maxWidth
	}
}
