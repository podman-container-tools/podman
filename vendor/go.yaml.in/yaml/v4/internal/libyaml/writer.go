// Copyright 2006-2010 Kirill Simonov
// Copyright 2011-2019 Canonical Ltd
// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0 AND MIT

// Output writer with buffering.
// Provides write buffering for the emitter stage.

package libyaml

import "fmt"

// Flush the output buffer.
func (emitter *Emitter) flush() error {
	if emitter.write_handler == nil {
		panic("write handler not set")
	}

	// Check if the buffer is empty.
	if emitter.buffer_pos == 0 {
		return nil
	}

	if err := emitter.write_handler(emitter, emitter.buffer[:emitter.buffer_pos]); err != nil {
		return WriterError{
			Err: fmt.Errorf("write error: %w", err),
		}
	}
	emitter.buffer_pos = 0
	return nil
}
