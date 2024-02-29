// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package scope

import (
	"fmt"
	"slices"

	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
)

// Scopes represents a set of scopes.
type Scopes struct {
	scopeList []Scope
}

// NewScopes creates a new Scopes instance.
func NewScopes(scopes ...Scope) Scopes {
	return Scopes{
		scopeList: slices.Clone(scopes),
	}
}

// ParseScopes parses scopes from a list of strings.
func ParseScopes(scopes []string) (Scopes, error) {
	scopeList := make([]Scope, 0, len(scopes))

	for _, stringScope := range scopes {
		sc, err := Parse(stringScope)
		if err != nil {
			return Scopes{}, err
		}

		scopeList = append(scopeList, sc)
	}

	return NewScopes(scopeList...), nil
}

// Check checks if the given scopes are contained in the scope set.
func (s Scopes) Check(requiredScopes ...Scope) error {
	for _, scope := range requiredScopes {
		matched := false

		for _, actorScope := range s.scopeList {
			if scope.Matches(actorScope) {
				matched = true

				break
			}
		}

		if !matched {
			return fmt.Errorf("missing required scope: %s", scope)
		}
	}

	return nil
}

// Intersect returns the intersection of the scope with the provided other scope.
func (s Scopes) Intersect(other Scopes) Scopes {
	intersectionSet := map[Scope]struct{}{}

	for _, scope1 := range s.scopeList {
		for _, scope2 := range other.scopeList {
			if scope1.Matches(scope2) {
				intersectionSet[scope1] = struct{}{}
			}

			if scope2.Matches(scope1) {
				intersectionSet[scope2] = struct{}{}
			}
		}
	}

	return NewScopes(maps.Keys(intersectionSet)...)
}

// Strings marshals the scopes to a list of strings.
func (s Scopes) Strings() []string {
	return xslices.Map(s.scopeList, func(scope Scope) string {
		return scope.String()
	})
}

// List returns the list of scopes.
func (s Scopes) List() []Scope {
	return slices.Clone(s.scopeList)
}
