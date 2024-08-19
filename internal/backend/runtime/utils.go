// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package runtime

import (
	"slices"
)

// SliceSet is a set of items.
type SliceSet[T any] struct {
	cmp   func(a, b T) int
	slice []T
}

// NewSliceSet creates new SliceSet.
func NewSliceSet[T any](cmp func(a, b T) int) SliceSet[T] {
	return SliceSet[T]{
		cmp: cmp,
	}
}

// Add adds item to the set.
func (s *SliceSet[T]) Add(item T) {
	if s.slice == nil {
		s.slice = []T{item}

		return
	}

	i, found := slices.BinarySearchFunc(s.slice, item, s.cmp)
	if found {
		return
	}

	s.slice = slices.Insert(s.slice, i, item)
}

// Len returns number of items in the set.
func (s *SliceSet[T]) Len() int {
	return len(s.slice)
}

// ForEach iterates over all items in the set.
func (s *SliceSet[T]) ForEach(fn func(item T)) {
	for _, item := range s.slice {
		fn(item)
	}
}

// Contains returns true if item is in the set.
func (s *SliceSet[T]) Contains(item T) bool {
	if len(s.slice) == 0 {
		return false
	}

	_, found := slices.BinarySearchFunc(s.slice, item, s.cmp)

	return found
}

// Reset removes all items from the set.
func (s *SliceSet[T]) Reset() {
	s.slice = nil
}

// Min returns minimum value in the set.
func (s *SliceSet[T]) Min() (T, bool) {
	if len(s.slice) == 0 {
		var zero T

		return zero, false
	}

	return s.slice[0], true
}

// Max returns maximum value in the set.
func (s *SliceSet[T]) Max() (T, bool) {
	if len(s.slice) == 0 {
		var zero T

		return zero, false
	}

	return s.slice[len(s.slice)-1], true
}

// StreamOffsetLimiter limits number of items returned by stream.
type StreamOffsetLimiter[T any] struct {
	cmp       func(a, b T) int
	min       *T
	max       *T
	offsetSet SliceSet[T]
	limitSet  SliceSet[T]

	offset int
	limit  int
}

// MakeStreamOffsetLimiter creates new StreamOffsetLimiter. To create a correct range it expects
// that all values come sorted for elements it didn't see before.
func MakeStreamOffsetLimiter[T any](offset int, limit int, cmp func(a, b T) int) *StreamOffsetLimiter[T] {
	return &StreamOffsetLimiter[T]{
		cmp:       cmp,
		offsetSet: NewSliceSet(cmp),
		limitSet:  NewSliceSet(cmp),
		offset:    offset,
		limit:     limit,
	}
}

// Check checks that value is in the range of limit and offset.
func (l *StreamOffsetLimiter[T]) Check(val T) bool {
	// limit and offset are not set, so we return everything.
	if l.limit == 0 && l.offset == 0 {
		return true
	}

	// We have a range, so we can check if value is in the range.
	if l.min != nil {
		if l.max != nil {
			return l.cmp(val, *l.min) >= 0 && l.cmp(val, *l.max) <= 0
		}

		// We have empty limit, so we return everything above offset.
		return l.cmp(val, *l.min) > 0
	}

	// We haven't skipped offset yet, so we need to skip it.
	if l.offsetSet.Len() < l.offset {
		l.offsetSet.Add(val)

		return false
	}

	// We have skipped offset, check if item is in it.
	if l.offsetSet.Contains(val) {
		return false
	}

	// We have empty limit, so we return everything above offset.
	if l.limit == 0 {
		if l.min == nil {
			minLimit, _ := l.offsetSet.Max()
			l.min = &minLimit
			l.offsetSet.Reset()
		}

		return l.cmp(val, *l.min) > 0
	}

	// We haven't reached limit yet, so we need to add it to the limit set.
	if l.limitSet.Len() < l.limit {
		l.limitSet.Add(val)

		return true
	}

	// Creating range using min and max from limit set.
	minLimit, _ := l.limitSet.Min()
	maxLimit, _ := l.limitSet.Max()

	l.min = &minLimit
	l.max = &maxLimit

	// We no longer need limit set and offset set, since we have the range now.
	l.limitSet.Reset()
	l.offsetSet.Reset()

	return l.cmp(val, *l.min) >= 0 && l.cmp(val, *l.max) <= 0
}
