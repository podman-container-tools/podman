// Copyright 2020 The go-libvirt Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package event

import (
	"context"
)

// emptyEvent is used as a zero-value. Clients will never receive one of these;
// they are only here to satisfy the compiler. See the comments in process() for
// more information.
type emptyEvent struct{}

func (emptyEvent) GetCallbackID() int32 { return 0 }

// Stream is an unbounded buffered event channel. The implementation
// consists of a pair of unbuffered channels and a goroutine to manage them.
// Client behavior will not cause incoming events to block.
type Stream struct {
	// Program specifies the source of the events - libvirt or QEMU.
	Program uint32

	// CallbackID is returned by the event registration call.
	CallbackID int32

	// manage unbounded channel behavior.
	queue   []Event
	qlen    chan (chan int)
	in, out chan Event

	// terminates processing
	shutdown context.CancelFunc
}

// NewStream configures a new Event Stream. Incoming events are appended to a
// queue, which is then relayed to the listening client. Client behavior will
// not cause incoming events to block. It is the responsibility of the caller
// to terminate the Stream via Shutdown() when no longer in use.
func NewStream(program uint32, cbID int32) *Stream {
	s := &Stream{
		Program:    program,
		CallbackID: cbID,
		in:         make(chan Event),
		out:        make(chan Event),
		qlen:       make(chan (chan int)),
	}

	// Start the processing loop, which will return a routine we can use to
	// shut the queue down later.
	s.shutdown = s.start()

	return s
}

// Len will return the current count of events in the internal queue for a
// stream. It does this by sending a message to the stream's process() loop,
// which will then write the current length to the channel contained in that
// message.
func (s *Stream) Len() int {
	// Send a request to the process() loop to get the current length of the
	// queue
	ch := make(chan int)
	s.qlen <- ch
	return <-ch
}

// Recv returns the next available event from the Stream's queue.
func (s *Stream) Recv() chan Event {
	return s.out
}

// Push appends a new event to the queue.
func (s *Stream) Push(e Event) {
	s.in <- e
}

// Shutdown gracefully terminates Stream processing, releasing all internal
// resources. Events which have not yet been received by the client will be
// dropped. Subsequent calls to Shutdown() are idempotent.
func (s *Stream) Shutdown() {
	if s.shutdown != nil {
		s.shutdown()
	}
}

// start starts the event processing loop, which will continue to run until
// terminated by the returned context.CancelFunc.
func (s *Stream) start() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())

	go s.process(ctx)

	return cancel
}

// process manages an Stream's lifecycle until canceled by the provided context.
// Incoming events are appended to a queue which is then relayed to the
// listening client. New events pushed onto the queue will not block if the
// client is not actively polling for them; the stream will buffer them
// internally.
func (s *Stream) process(ctx context.Context) {
	// Close the output channel so that clients know this stream is finished.
	// We don't close s.in to avoid creating a race with the stream's Push()
	// function.
	defer close(s.out)

	// This function is used to retrieve the next event from the queue, to be
	// sent to the client. If there are no more events to send, it returns a nil
	// channel and a zero-value event.
	nextEvent := func() (chan Event, Event) {
		sendCh := chan Event(nil)
		next := Event(emptyEvent{})
		if len(s.queue) > 0 {
			sendCh = s.out
			next = s.queue[0]
		}
		return sendCh, next
	}

	// The select statement in this loop relies on the fact that a send to a nil
	// channel will block forever. If we have no entries in the queue, the
	// sendCh variable will be nil, so the clause that attempts to send an event
	// to the client will never complete. Clients will never receive an
	// emptyEvent.
	for {
		sendCh, nextEvt := nextEvent()

		select {
		// new event received, append to queue
		case e := <-s.in:
			s.queue = append(s.queue, e)

		case lenCh := <-s.qlen:
			lenCh <- len(s.queue)

		// client received an event, pop from queue
		case sendCh <- nextEvt:
			s.queue = s.queue[1:]

		// shutdown requested
		case <-ctx.Done():
			return
		}
	}
}
