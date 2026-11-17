// Copyright 2018 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package p9

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hugelgupf/p9/linux"
	"github.com/u-root/uio/ulog"
)

// Server is a 9p2000.L server.
type Server struct {
	// attacher provides the attach function.
	attacher Attacher

	// pathTree is the full set of paths opened on this server.
	//
	// These may be across different connections, but rename operations
	// must be serialized globally for safely. There is a single pathTree
	// for the entire server, and not per connection.
	pathTree *pathNode

	// renameMu is a global lock protecting rename operations. With this
	// lock, we can be certain that any given rename operation can safely
	// acquire two path nodes in any order, as all other concurrent
	// operations acquire at most a single node.
	renameMu sync.RWMutex

	// log is a logger to log to, if specified.
	log ulog.Logger
}

// ServerOpt is an optional config for a new server.
type ServerOpt func(s *Server)

// WithServerLogger overrides the default logger for the server.
func WithServerLogger(l ulog.Logger) ServerOpt {
	return func(s *Server) {
		s.log = l
	}
}

// NewServer returns a new server.
func NewServer(attacher Attacher, o ...ServerOpt) *Server {
	s := &Server{
		attacher: attacher,
		pathTree: newPathNode(),
		log:      ulog.Null,
	}
	for _, opt := range o {
		opt(s)
	}
	return s
}

// connState is the state for a single connection.
type connState struct {
	// server is the backing server.
	server *Server

	// fids is the set of active fids.
	//
	// This is used to find fids for files.
	fidMu sync.Mutex
	fids  map[fid]*fidRef

	// tags is the set of active tags.
	//
	// The given channel is closed when the
	// tag is finished with processing.
	tagMu sync.Mutex
	tags  map[tag]chan struct{}

	// messageSize is the maximum message size. The server does not
	// do automatic splitting of messages.
	messageSize   uint32
	readBufPool   sync.Pool
	pristineZeros []byte

	// baseVersion is the version of 9P protocol.
	baseVersion baseVersion

	// version is the agreed upon version X of 9P2000.L.Google.X.
	// version 0 implies 9P2000.L.
	version uint32

	// pendingWg counts requests that are still being handled.
	pendingWg sync.WaitGroup

	// recvMu serializes receiving from t.
	recvMu sync.Mutex

	// recvIdle is the number of goroutines in handleRequests() attempting to
	// lock recvMu so that they can receive from t. recvIdle is accessed
	// using atomic memory operations.
	recvIdle int32

	// If recvShutdown is true, at least one goroutine has observed a
	// connection error while receiving from t, and all goroutines in
	// handleRequests() should exit immediately. recvShutdown is protected
	// by recvMu.
	recvShutdown bool

	// sendMu serializes sending to r.
	sendMu sync.Mutex

	// t reads T messages and r write R messages
	t io.ReadCloser
	r io.WriteCloser
}

// xattrOp is the xattr related operations, walk or create.
type xattrOp int

const (
	xattrNone   = 0
	xattrCreate = 1
	xattrWalk   = 2
)

type pendingXattr struct {
	// the pending xattr-related operation
	op xattrOp

	// name is the attribute.
	name string

	// size of the attribute value, represents the
	// length of the attribute value that is going to write to or read from a file.
	size uint64

	// flags associated with a txattrcreate message.
	// generally Linux setxattr(2) flags.
	flags XattrFlags

	// saved up xattr operation value (for reads, listed / gotten buffer --
	// ready for chunking; for writes, this is used to accumulate chunked
	// values until a Tclunk actuates the operation)
	buf []byte
}

// fidRef wraps a node and tracks references.
type fidRef struct {
	// server is the associated server.
	server *Server

	// file is the associated File.
	file File

	// pendingXattr is the xattr-related operations that are going to be done
	// in a tread or twrite request.
	pendingXattr pendingXattr

	// refs is an active refence count.
	//
	// The node above will be closed only when refs reaches zero.
	refs int64

	// opened indicates whether this has been opened already.
	//
	// This is updated in handlers.go.
	//
	// opened is protected by pathNode.opMu or renameMu (for write).
	opened bool

	// mode is the fidRef's mode from the walk. Only the type bits are
	// valid, the permissions may change. This is used to sanity check
	// operations on this element, and prevent walks across
	// non-directories.
	mode FileMode

	// openFlags is the mode used in the open.
	//
	// This is updated in handlers.go.
	//
	// opened is protected by pathNode.opMu or renameMu (for write).
	openFlags OpenFlags

	// pathNode is the current pathNode for this fid.
	pathNode *pathNode

	// parent is the parent fidRef. We hold on to a parent reference to
	// ensure that hooks, such as Renamed, can be executed safely by the
	// server code.
	//
	// Note that parent cannot be changed without holding both the global
	// rename lock and a writable lock on the associated pathNode for this
	// fidRef. Holding either of these locks is sufficient to examine
	// parent safely.
	//
	// The parent will be nil for root fidRefs and pending xattrwalk/create
	// fidRefs, and non-nil otherwise. The method maybeParent can be used
	// to return a cyclical reference, and hasParent should be used to
	// check for root over looking at parent directly.
	parent *fidRef
}

// IncRef increases the references on a fid.
func (f *fidRef) IncRef() {
	atomic.AddInt64(&f.refs, 1)
}

// DecRef should be called when you're finished with a fid.
func (f *fidRef) DecRef() error {
	if atomic.AddInt64(&f.refs, -1) == 0 {
		var (
			errs []error
			err  = f.file.Close()
		)
		if err != nil {
			err = fmt.Errorf("file: %w", err)
			errs = append(errs, err)
		}

		// Drop the parent reference.
		//
		// Since this fidRef is guaranteed to be non-discoverable when
		// the references reach zero, we don't need to worry about
		// clearing the parent.
		if f.parent != nil {
			// If we've been previously deleted, removing this
			// ref is a no-op. That's expected.
			f.parent.pathNode.removeChild(f)
			if pErr := f.parent.DecRef(); pErr != nil {
				pErr = fmt.Errorf("parent: %w", pErr)
				errs = append(errs, pErr)
			}
		}
		return errors.Join(errs...)
	}
	return nil
}

// TryIncRef returns true if a new reference is taken on the fid, and false if
// the fid has been destroyed.
func (f *fidRef) TryIncRef() bool {
	for {
		r := atomic.LoadInt64(&f.refs)
		if r <= 0 {
			return false
		}
		if atomic.CompareAndSwapInt64(&f.refs, r, r+1) {
			return true
		}
	}
}

// isDeleted returns true if this fidRef has been deleted.
//
// Precondition: this must be called via safelyRead, safelyWrite or
// safelyGlobal.
func (f *fidRef) isDeleted() bool {
	return atomic.LoadUint32(&f.pathNode.deleted) != 0
}

// hasParent indicates whether this is a root fid.
func (f *fidRef) hasParent() bool {
	return f.parent == nil
}

// maybeParent returns a cyclic reference for roots, and the parent otherwise.
func (f *fidRef) maybeParent() *fidRef {
	if f.parent != nil {
		return f.parent
	}
	return f // Root has itself.
}

// notifyDelete marks all fidRefs as deleted.
//
// Precondition: this must be called via safelyWrite or safelyGlobal.
func notifyDelete(pn *pathNode) {
	atomic.StoreUint32(&pn.deleted, 1)

	// Call on all subtrees.
	pn.forEachChildNode(func(pn *pathNode) {
		notifyDelete(pn)
	})
}

// markChildDeleted marks all children below the given name as deleted.
//
// Precondition: this must be called via safelyWrite or safelyGlobal.
func (f *fidRef) markChildDeleted(name string) {
	if origPathNode := f.pathNode.removeWithName(name, nil); origPathNode != nil {
		// Mark all children as deleted.
		notifyDelete(origPathNode)
	}
}

// notifyNameChange calls the relevant Renamed method on all nodes in the path,
// recursively. Note that this applies only for subtrees, as these
// notifications do not apply to the actual file whose name has changed.
//
// Precondition: this must be called via safelyGlobal.
func notifyNameChange(pn *pathNode) {
	// Call on all local references.
	pn.forEachChildRef(func(ref *fidRef, name string) {
		ref.file.Renamed(ref.parent.file, name)
	})

	// Call on all subtrees.
	pn.forEachChildNode(func(pn *pathNode) {
		notifyNameChange(pn)
	})
}

// renameChildTo renames the given child to the target.
//
// Precondition: this must be called via safelyGlobal.
func (f *fidRef) renameChildTo(oldName string, target *fidRef, newName string) {
	target.markChildDeleted(newName)
	origPathNode := f.pathNode.removeWithName(oldName, func(ref *fidRef) {
		// N.B. DecRef can take f.pathNode's parent's childMu. This is
		// allowed because renameMu is held for write via safelyGlobal.
		ref.parent.DecRef() // Drop original reference.
		ref.parent = target // Change parent.
		ref.parent.IncRef() // Acquire new one.
		if f.pathNode == target.pathNode {
			target.pathNode.addChildLocked(ref, newName)
		} else {
			target.pathNode.addChild(ref, newName)
		}
		ref.file.Renamed(target.file, newName)
	})

	if origPathNode != nil {
		// Replace the previous (now deleted) path node.
		target.pathNode.addPathNodeFor(newName, origPathNode)
		// Call Renamed on all children.
		notifyNameChange(origPathNode)
	}
}

// safelyRead executes the given operation with the local path node locked.
// This implies that paths will not change during the operation.
func (f *fidRef) safelyRead(fn func() error) (err error) {
	f.server.renameMu.RLock()
	defer f.server.renameMu.RUnlock()
	f.pathNode.opMu.RLock()
	defer f.pathNode.opMu.RUnlock()
	return fn()
}

// safelyWrite executes the given operation with the local path node locked in
// a writable fashion. This implies some paths may change.
func (f *fidRef) safelyWrite(fn func() error) (err error) {
	f.server.renameMu.RLock()
	defer f.server.renameMu.RUnlock()
	f.pathNode.opMu.Lock()
	defer f.pathNode.opMu.Unlock()
	return fn()
}

// safelyGlobal executes the given operation with the global path lock held.
func (f *fidRef) safelyGlobal(fn func() error) (err error) {
	f.server.renameMu.Lock()
	defer f.server.renameMu.Unlock()
	return fn()
}

// Lookupfid finds the given fid.
//
// You should call fid.DecRef when you are finished using the fid.
func (cs *connState) LookupFID(fid fid) (*fidRef, bool) {
	cs.fidMu.Lock()
	defer cs.fidMu.Unlock()
	fidRef, ok := cs.fids[fid]
	if ok {
		fidRef.IncRef()
		return fidRef, true
	}
	return nil, false
}

// Insertfid installs the given fid.
//
// This fid starts with a reference count of one. If a fid exists in
// the slot already it is closed, per the specification.
func (cs *connState) InsertFID(fid fid, newRef *fidRef) {
	cs.fidMu.Lock()
	defer cs.fidMu.Unlock()
	origRef, ok := cs.fids[fid]
	if ok {
		defer origRef.DecRef()
	}
	newRef.IncRef()
	cs.fids[fid] = newRef
}

// Deletefid removes the given fid.
//
// This simply removes it from the map and drops a reference.
func (cs *connState) DeleteFID(fid fid) error {
	cs.fidMu.Lock()
	defer cs.fidMu.Unlock()
	fidRef, ok := cs.fids[fid]
	if !ok {
		return linux.EBADF
	}
	delete(cs.fids, fid)
	return fidRef.DecRef()
}

// StartTag starts handling the tag.
//
// False is returned if this tag is already active.
func (cs *connState) StartTag(t tag) bool {
	cs.tagMu.Lock()
	defer cs.tagMu.Unlock()
	_, ok := cs.tags[t]
	if ok {
		return false
	}
	cs.tags[t] = make(chan struct{})
	return true
}

// ClearTag finishes handling a tag.
func (cs *connState) ClearTag(t tag) {
	cs.tagMu.Lock()
	defer cs.tagMu.Unlock()
	ch, ok := cs.tags[t]
	if !ok {
		// Should never happen.
		panic("unused tag cleared")
	}
	delete(cs.tags, t)

	// Notify.
	close(ch)
}

// Waittag waits for a tag to finish.
func (cs *connState) WaitTag(t tag) {
	cs.tagMu.Lock()
	ch, ok := cs.tags[t]
	cs.tagMu.Unlock()
	if !ok {
		return
	}

	// Wait for close.
	<-ch
}

// handleRequest handles a single request.
//
// The recvDone channel is signaled when recv is done (with a error if
// necessary). The sendDone channel is signaled with the result of the send.
func (cs *connState) handleRequest() bool {
	cs.pendingWg.Add(1)
	defer cs.pendingWg.Done()

	// Obtain the right to receive a message from cs.t.
	atomic.AddInt32(&cs.recvIdle, 1)
	cs.recvMu.Lock()
	atomic.AddInt32(&cs.recvIdle, -1)

	if cs.recvShutdown {
		// Another goroutine already detected a connection problem; exit
		// immediately.
		cs.recvMu.Unlock()
		return false
	}

	messageSize := atomic.LoadUint32(&cs.messageSize)
	if messageSize == 0 {
		// Default or not yet negotiated.
		messageSize = maximumLength
	}

	// Receive a message.
	tag, m, err := recv(cs.server.log, cs.t, messageSize, msgDotLRegistry.get)
	if errSocket, ok := err.(ConnError); ok {
		if errSocket.error != io.EOF {
			// Connection problem; stop serving.
			cs.server.log.Printf("p9.recv: %v", errSocket.error)
		}
		cs.recvShutdown = true
		cs.recvMu.Unlock()
		return false
	}

	// Ensure that another goroutine is available to receive from cs.t.
	if atomic.LoadInt32(&cs.recvIdle) == 0 {
		cs.pendingWg.Add(1)
		go func() {
			defer cs.pendingWg.Done()
			cs.handleRequests() // S/R-SAFE: Irrelevant.
		}()
	}
	cs.recvMu.Unlock()

	// Deal with other errors.
	if err != nil && err != io.EOF {
		// If it's not a connection error, but some other protocol error,
		// we can send a response immediately.
		cs.sendMu.Lock()
		err := send(cs.server.log, cs.r, tag, newErr(err))
		cs.sendMu.Unlock()
		if err != nil {
			cs.server.log.Printf("p9.send: %v", err)
		}
		return true
	}

	// Try to start the tag.
	if !cs.StartTag(tag) {
		cs.server.log.Printf("no valid tag [%05d]", tag)
		// Nothing we can do at this point; client is bogus.
		return true
	}

	// Handle the message.
	r := cs.handle(m)

	// Clear the tag before sending. That's because as soon as this
	// hits the wire, the client can legally send another message
	// with the same tag.
	cs.ClearTag(tag)

	// Send back the result.
	cs.sendMu.Lock()
	err = send(cs.server.log, cs.r, tag, r)
	cs.sendMu.Unlock()
	if err != nil {
		cs.server.log.Printf("p9.send: %v", err)
	}

	msgDotLRegistry.put(m)
	m = nil // 'm' should not be touched after this point.
	return true
}

func (cs *connState) handle(m message) (r message) {
	defer func() {
		if r == nil {
			// Don't allow a panic to propagate.
			err := recover()

			// Include a useful log message.
			cs.server.log.Printf("panic in handler - %v: %s", err, debug.Stack())

			// Wrap in an EFAULT error; we don't really have a
			// better way to describe this kind of error. It will
			// usually manifest as a result of the test framework.
			r = newErr(linux.EFAULT)
		}
	}()

	if handler, ok := m.(handler); ok {
		// Call the message handler.
		r = handler.handle(cs)
	} else {
		// Produce an ENOSYS error.
		r = newErr(linux.ENOSYS)
	}
	return
}

func (cs *connState) handleRequests() {
	for {
		if !cs.handleRequest() {
			return
		}
	}
}

func (cs *connState) stop() {
	// Wait for completion of all inflight request goroutines.. If a
	// request is stuck, something has the opportunity to kill us with
	// SIGABRT to get a stack dump of the offending handler.
	cs.pendingWg.Wait()

	// Ensure the connection is closed.
	cs.r.Close()
	cs.t.Close()

	for _, fidRef := range cs.fids {
		// Drop final reference in the fid table. Note this should
		// always close the file, since we've ensured that there are no
		// handlers running via the wait for Pending => 0 below.
		fidRef.DecRef()
	}
}

// Handle handles a single connection.
func (s *Server) Handle(t io.ReadCloser, r io.WriteCloser) error {
	cs := &connState{
		server: s,
		t:      t,
		r:      r,
		fids:   make(map[fid]*fidRef),
		tags:   make(map[tag]chan struct{}),
	}
	defer cs.stop()

	// Serve requests from t in the current goroutine; handleRequests()
	// will create more goroutines as needed.
	cs.handleRequests()
	return nil
}

func isErrClosing(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}

// Serve handles requests from the bound socket.
//
// The passed serverSocket _must_ be created in packet mode.
func (s *Server) Serve(serverSocket net.Listener) error {
	return s.ServeContext(nil, serverSocket)
}

var errAlreadyClosed = errors.New("already closed")

// ServeContext handles requests from the bound socket.
//
// The passed serverSocket _must_ be created in packet mode.
//
// When the context is done, the listener is closed and serve returns once
// every request has been handled.
func (s *Server) ServeContext(ctx context.Context, serverSocket net.Listener) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	var cancelCause context.CancelCauseFunc
	if ctx != nil {
		ctx, cancelCause = context.WithCancelCause(ctx)

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-ctx.Done()

			// Only close the server socket if it wasn't already closed.
			if err := ctx.Err(); errors.Is(err, errAlreadyClosed) {
				return
			}
			serverSocket.Close()
		}()
	}

	for {
		conn, err := serverSocket.Accept()
		if err != nil {
			if cancelCause != nil {
				cancelCause(errAlreadyClosed)
			}
			if isErrClosing(err) {
				return nil
			}
			// Something went wrong.
			return err
		}

		wg.Add(1)
		go func(conn net.Conn) { // S/R-SAFE: Irrelevant.
			s.Handle(conn, conn)
			wg.Done()
		}(conn)
	}
}
