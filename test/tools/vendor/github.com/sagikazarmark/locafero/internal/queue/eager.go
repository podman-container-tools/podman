package queue

import "sync"

// NewEager creates a new eager queue.
func NewEager[T any]() Queue[T] {
	return &Eager[T]{}
}

// Eager is a queue that processes items eagerly.
type Eager[T any] struct {
	results []T
	error   error

	mu sync.Mutex
}

// Add implements the [Queue] interface.
func (p *Eager[T]) Add(fn func() (T, error)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Return early if there's an error
	if p.error != nil {
		return
	}

	result, err := fn()
	if err != nil {
		p.error = err

		return
	}

	p.results = append(p.results, result)
}

// Wait implements the [Queue] interface.
func (p *Eager[T]) Wait() ([]T, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.error != nil {
		return nil, p.error
	}

	results := p.results

	// Reset results for reuse
	p.results = nil

	return results, nil
}
