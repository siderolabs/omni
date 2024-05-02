// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package set is a generic set.
package set

import (
	"cmp"
	"slices"

	"github.com/siderolabs/gen/maps"
)

// Set implements a generic set.
type Set[K cmp.Ordered] map[K]struct{}

// Contains checks if the set has value.
func (s Set[K]) Contains(value K) bool {
	_, ok := s[value]

	return ok
}

// Add adds a value to the set.
func (s Set[K]) Add(value K) {
	s[value] = struct{}{}
}

// Difference calculates difference between a set and number of other sets.
func Difference[K cmp.Ordered](a Set[K], other ...Set[K]) Set[K] {
	res := make(Set[K], len(a))

outer:
	for k := range a {
		for _, b := range other {
			if _, ok := b[k]; ok {
				continue outer
			}
		}

		res[k] = struct{}{}
	}

	return res
}

// Intersection calculates intersections of a set with all other sets.
func Intersection[K cmp.Ordered](a Set[K], other ...Set[K]) Set[K] {
	res := make(Set[K], len(a))

outer:
	for k := range a {
		for _, b := range other {
			if _, ok := b[k]; !ok {
				continue outer
			}
		}

		res[k] = struct{}{}
	}

	return res
}

// Union calculates union of a set with all other sets.
func Union[K cmp.Ordered](sets ...Set[K]) Set[K] {
	res := Set[K]{}

	for _, set := range sets {
		for k := range set {
			res[k] = struct{}{}
		}
	}

	return res
}

// Values converts set to a sorted slice.
func Values[K cmp.Ordered](s Set[K]) []K {
	keys := maps.Keys(s)

	slices.Sort(keys)

	return keys
}
