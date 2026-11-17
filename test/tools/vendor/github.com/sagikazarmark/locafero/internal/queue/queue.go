// Package queue provides a generic queue implementation.
package queue

// Queue represents a generic queue.
type Queue[T any] interface {
	Add(func() (T, error))
	Wait() ([]T, error)
}
