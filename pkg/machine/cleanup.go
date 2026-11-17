package machine

import (
	"fmt"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
)

type CleanupCallback struct {
	Funcs []func() error
	mu    sync.Mutex
}

func (c *CleanupCallback) CleanIfErr(err *error) {
	// Do not remove created files if the init is successful
	if *err == nil {
		return
	}
	c.clean()
}

func (c *CleanupCallback) CleanOnSignal(quiet bool) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	_, ok := <-ch
	if !ok {
		return
	}

	if !quiet {
		fmt.Println("Received a terminate signal")
	}
	c.clean()
	if !quiet {
		fmt.Println("Machine command rollback completed")
	}
	os.Exit(1)
}

func (c *CleanupCallback) clean() {
	// When a term signal is received the cleanup can be invoked
	// concurrently in 2 goroutines:
	//
	//    - signal flow: a termination signal is received and the
	//                   goroutine where CleanOnSignal() is running is
	//                   unblocked and starts invoking the callbacks
	//    - error flow: an error is returned in the main goroutine, after
	//                  the signal is received, and CleanIfErr() is invoked,
	//                  but c.Funcs has been set to nil and therefore no
	//                  callbacks is exec in this goroutine
	//
	// When this is the case the second goroutine should be blocked until
	// the first goroutine comples the cleanup. c.Funcs is also set to nil
	// so that cleanup doesn't happen twice.
	c.mu.Lock()
	funcs := c.Funcs
	c.Funcs = nil
	// Cleanup functions invoked in reverse registration order
	for _, cleanfunc := range slices.Backward(funcs) {
		if err := cleanfunc(); err != nil {
			logrus.Error(err)
		}
	}
	c.mu.Unlock()
}

func CleanUp() CleanupCallback {
	return CleanupCallback{
		Funcs: []func() error{},
	}
}

func (c *CleanupCallback) Add(anotherfunc func() error) {
	c.mu.Lock()
	c.Funcs = append(c.Funcs, anotherfunc)
	c.mu.Unlock()
}
