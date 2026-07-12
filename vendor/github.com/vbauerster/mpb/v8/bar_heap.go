package mpb

import "container/heap"

var _ heap.Interface = (*barHeap)(nil)

type barHeap []*Bar

func (s barHeap) Len() int { return len(s) }

// it's a reversed Less same as sort.Reverse(sort.Interface) would do
// becasuse we need greater priority item to pop first
func (s barHeap) Less(i, j int) bool {
	return s[j].priority < s[i].priority
}

func (s barHeap) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
	s[i].index = i
	s[j].index = j
}

func (h *barHeap) Push(x any) {
	b := x.(*Bar)
	b.index = h.Len()
	*h = append(*h, b)
}

func (h *barHeap) Pop() any {
	s := *h
	i := s.Len() - 1
	b := s[i]
	b.index = -1 // for safety
	s[i] = nil   // nil to avoid memory leak
	*h = s[:i]
	return b
}
