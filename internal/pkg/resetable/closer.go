// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package resetable provides a Closer helper type for defer statements.
package resetable

// NewCloser returns a new Closer.
func NewCloser(closer func()) Closer {
	return Closer{
		closer: closer,
	}
}

// NewCloserErr returns a new Closer. It is similar to NewCloser but accepts functions that return error.
func NewCloserErr(closer func() error) Closer {
	return NewCloser(func() { closer() }) //nolint:errcheck
}

// Closer is a wrapper around a closer function that can be reset.
// It is useful for conditional closing in defer statements.
type Closer struct {
	closer func()
}

// Close calls the wrapped closer function if it is not nil.
func (s *Closer) Close() {
	if s.closer != nil {
		s.closer()
	}
}

// Reset resets the wrapped closer function.
func (s *Closer) Reset() {
	s.closer = nil
}
