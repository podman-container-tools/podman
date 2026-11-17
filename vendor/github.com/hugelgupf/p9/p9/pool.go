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
	"sync"
)

// pool is a simple allocator.
//
// It is used for both tags and FIDs.
type pool struct {
	mu sync.Mutex

	// cache is the set of returned values.
	cache []uint64

	// start is the starting value (if needed).
	start uint64

	// limit is the upper limit.
	limit uint64
}

// Get gets a value from the pool.
func (p *pool) Get() (uint64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Anything cached?
	if len(p.cache) > 0 {
		v := p.cache[len(p.cache)-1]
		p.cache = p.cache[:len(p.cache)-1]
		return v, true
	}

	// Over the limit?
	if p.start == p.limit {
		return 0, false
	}

	// Generate a new value.
	v := p.start
	p.start++
	return v, true
}

// Put returns a value to the pool.
func (p *pool) Put(v uint64) {
	p.mu.Lock()
	p.cache = append(p.cache, v)
	p.mu.Unlock()
}
