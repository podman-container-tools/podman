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
	"fmt"
	"sync"
)

// pathNode is a single node in a path traversal.
//
// These are shared by all fidRefs that point to the same path.
//
// Lock ordering:
//
//	opMu
//	  childMu
//
//	Two different pathNodes may only be locked if Server.renameMu is held for
//	write, in which case they can be acquired in any order.
type pathNode struct {
	// opMu synchronizes high-level, sematic operations, such as the
	// simultaneous creation and deletion of a file.
	opMu sync.RWMutex

	// deleted indicates that the backing file has been deleted. We stop many
	// operations at the API level if they are incompatible with a file that has
	// already been unlinked. deleted is protected by opMu. However, it may be
	// changed without opMu if this node is deleted as part of an entire subtree
	// on unlink. So deleted must only be accessed/mutated using atomics.
	deleted uint32

	// childMu protects the fields below.
	childMu sync.RWMutex

	// childNodes maps child path component names to their pathNode.
	childNodes map[string]*pathNode

	// childRefs maps child path component names to all of the their
	// references.
	childRefs map[string]map[*fidRef]struct{}

	// childRefNames maps child references back to their path component
	// name.
	childRefNames map[*fidRef]string
}

func newPathNode() *pathNode {
	return &pathNode{
		childNodes:    make(map[string]*pathNode),
		childRefs:     make(map[string]map[*fidRef]struct{}),
		childRefNames: make(map[*fidRef]string),
	}
}

// forEachChildRef calls fn for each child reference.
func (p *pathNode) forEachChildRef(fn func(ref *fidRef, name string)) {
	p.childMu.RLock()
	defer p.childMu.RUnlock()

	for name, m := range p.childRefs {
		for ref := range m {
			fn(ref, name)
		}
	}
}

// forEachChildNode calls fn for each child pathNode.
func (p *pathNode) forEachChildNode(fn func(pn *pathNode)) {
	p.childMu.RLock()
	defer p.childMu.RUnlock()

	for _, pn := range p.childNodes {
		fn(pn)
	}
}

// pathNodeFor returns the path node for the given name, or a new one.
func (p *pathNode) pathNodeFor(name string) *pathNode {
	p.childMu.RLock()
	// Fast path, node already exists.
	if pn, ok := p.childNodes[name]; ok {
		p.childMu.RUnlock()
		return pn
	}
	p.childMu.RUnlock()

	// Slow path, create a new pathNode for shared use.
	p.childMu.Lock()

	// Re-check after re-lock.
	if pn, ok := p.childNodes[name]; ok {
		p.childMu.Unlock()
		return pn
	}

	pn := newPathNode()
	p.childNodes[name] = pn
	p.childMu.Unlock()
	return pn
}

// nameFor returns the name for the given fidRef.
//
// Precondition: addChild is called for ref before nameFor.
func (p *pathNode) nameFor(ref *fidRef) string {
	p.childMu.RLock()
	n, ok := p.childRefNames[ref]
	p.childMu.RUnlock()

	if !ok {
		// This should not happen, don't proceed.
		panic(fmt.Sprintf("expected name for %+v, none found", ref))
	}

	return n
}

// addChildLocked adds a child reference to p.
//
// Precondition: As addChild, plus childMu is locked for write.
func (p *pathNode) addChildLocked(ref *fidRef, name string) {
	if n, ok := p.childRefNames[ref]; ok {
		// This should not happen, don't proceed.
		panic(fmt.Sprintf("unexpected fidRef %+v with path %q, wanted %q", ref, n, name))
	}

	p.childRefNames[ref] = name

	m, ok := p.childRefs[name]
	if !ok {
		m = make(map[*fidRef]struct{})
		p.childRefs[name] = m
	}

	m[ref] = struct{}{}
}

// addChild adds a child reference to p.
//
// Precondition: ref may only be added once at a time.
func (p *pathNode) addChild(ref *fidRef, name string) {
	p.childMu.Lock()
	p.addChildLocked(ref, name)
	p.childMu.Unlock()
}

// removeChild removes the given child.
//
// This applies only to an individual fidRef, which is not required to exist.
func (p *pathNode) removeChild(ref *fidRef) {
	p.childMu.Lock()

	// This ref may not exist anymore. This can occur, e.g., in unlink,
	// where a removeWithName removes the ref, and then a DecRef on the ref
	// attempts to remove again.
	if name, ok := p.childRefNames[ref]; ok {
		m, ok := p.childRefs[name]
		if !ok {
			// This should not happen, don't proceed.
			p.childMu.Unlock()
			panic(fmt.Sprintf("name %s missing from childfidRefs", name))
		}

		delete(m, ref)
		if len(m) == 0 {
			delete(p.childRefs, name)
		}
	}

	delete(p.childRefNames, ref)

	p.childMu.Unlock()
}

// addPathNodeFor adds an existing pathNode as the node for name.
//
// Preconditions: newName does not exist.
func (p *pathNode) addPathNodeFor(name string, pn *pathNode) {
	p.childMu.Lock()

	if opn, ok := p.childNodes[name]; ok {
		p.childMu.Unlock()
		panic(fmt.Sprintf("unexpected pathNode %+v with path %q", opn, name))
	}

	p.childNodes[name] = pn
	p.childMu.Unlock()
}

// removeWithName removes all references with the given name.
//
// The provided function is executed after reference removal. The only method
// it may (transitively) call on this pathNode is addChildLocked.
//
// If a child pathNode for name exists, it is removed from this pathNode and
// returned by this function. Any operations on the removed tree must use this
// value.
func (p *pathNode) removeWithName(name string, fn func(ref *fidRef)) *pathNode {
	p.childMu.Lock()
	defer p.childMu.Unlock()

	if m, ok := p.childRefs[name]; ok {
		for ref := range m {
			delete(m, ref)
			delete(p.childRefNames, ref)
			if fn == nil {
				continue
			}

			// Attempt to hold a reference while calling fn() to
			// prevent concurrent destruction of the child, which
			// can lead to data races. If the child has already
			// been destroyed, then we can skip the callback.
			if ref.TryIncRef() {
				fn(ref)
				ref.DecRef()
			}
		}
	}

	// Return the original path node, if it exists.
	origPathNode := p.childNodes[name]
	delete(p.childNodes, name)
	return origPathNode
}
