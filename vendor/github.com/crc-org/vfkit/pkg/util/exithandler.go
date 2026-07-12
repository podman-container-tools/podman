package util

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type exitHandlerRegistry struct {
	handlers []func()
	mutex    sync.Mutex
}

var exitRegistry = exitHandlerRegistry{}

// RegisterExitHandler appends a func Exit handler to the list of handlers.
// The handlers will be invoked when vfkit receives a termination or interruption signal
//
// This method is useful when a caller wishes to execute a func before a shutdown.
func RegisterExitHandler(handler func()) {
	exitRegistry.mutex.Lock()
	defer exitRegistry.mutex.Unlock()
	exitRegistry.handlers = append(exitRegistry.handlers, handler)
}

// SetupExitSignalHandling sets up a signal channel to listen for termination or interruption signals.
// When one of these signals is received, all the registered exit handlers will be invoked, just
// before terminating the program.
func SetupExitSignalHandling(shutdownFunc func()) {
	setupExitSignalHandling(shutdownFunc)
}

// setupExitSignalHandling sets up a signal channel to listen for termination or interruption signals.
// When one of these signals is received, all the registered exit handlers will be invoked.
// It is possible to prevent the program from exiting by setting the doExit param to false (used for testing)
func setupExitSignalHandling(shutdownFunc func()) {
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		defer func() {
			signal.Stop(sigChan)
		}()
		for sig := range sigChan {
			log.Printf("captured %v, calling exit handlers and exiting..", sig)
			if shutdownFunc != nil {
				shutdownFunc()
			}
		}
	}()
}

// ExecuteExitHandlers is call all registered exit handlers
// This function should be called when program finish work(i.e. when VM is turned off by guest OS)
func ExecuteExitHandlers() {
	exitRegistry.mutex.Lock()
	defer exitRegistry.mutex.Unlock()
	for _, handler := range exitRegistry.handlers {
		handler()
	}
}
