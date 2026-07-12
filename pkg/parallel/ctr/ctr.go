//go:build !remote && (linux || freebsd)

package ctr

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/parallel"
)

// ContainerOp performs the given function on the given set of
// containers, using a number of parallel threads.
// If no error is returned, each container specified in ctrs will have an entry
// in the resulting map; containers with no error will be set to nil.
func ContainerOp(ctx context.Context, ctrs []*libpod.Container, applyFunc func(*libpod.Container) error) (map[*libpod.Container]error, error) {
	// We could use a sync.Map but given Go's lack of generic I'd rather
	// just use a lock on a normal map...
	// The expectation is that most of the time is spent in applyFunc
	// anyways.

	errMap := make(map[*libpod.Container]<-chan error)

	for _, ctr := range ctrs {
		c := ctr
		logrus.Debugf("Starting parallel job on container %s", c.ID())
		errChan := parallel.Enqueue(ctx, func() error {
			return applyFunc(c)
		})
		errMap[c] = errChan
	}

	finalErr := make(map[*libpod.Container]error)
	for ctr, errChan := range errMap {
		err := <-errChan
		finalErr[ctr] = err
	}

	return finalErr, nil
}
