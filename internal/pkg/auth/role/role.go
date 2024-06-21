// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package role contains the role definitions and checks.
package role

import (
	"fmt"
)

// Role represents a user role.
type Role string

const (
	// None is a role that has no capability.
	//
	// tsgen:RoleNone
	None Role = "None"

	// CloudProvider is a role to be used solely by cloud providers.
	//
	// tsgen:RoleCloudProvider
	CloudProvider Role = "CloudProvider"

	// Reader is a role that has read-only capability.
	//
	// tsgen:RoleReader
	Reader Role = "Reader"

	// Operator is a role that has read/write capability.
	//
	// tsgen:RoleOperator
	Operator Role = "Operator"

	// Admin is a role that has read/write and user/service account management capability.
	//
	// tsgen:RoleAdmin
	Admin Role = "Admin"
)

var roles = []Role{None, CloudProvider, Reader, Operator, Admin}

var indexes = func() map[Role]int {
	result := make(map[Role]int, len(roles))

	for i, role := range roles {
		result[role] = i
	}

	return result
}()

// Parse parses the role string.
func Parse(role string) (Role, error) {
	parsed, ok := indexes[Role(role)]
	if !ok {
		return "", fmt.Errorf("unknown role to parse: %q", role)
	}

	return roles[parsed], nil
}

// Check verifies if the actor role satisfies the required role.
func (r Role) Check(role Role) error {
	thisIndex, ok := indexes[r]
	if !ok {
		return fmt.Errorf("unknown role to run check on: %q", r)
	}

	otherIndex, ok := indexes[role]
	if !ok {
		return fmt.Errorf("unknown other role: %q", role)
	}

	if thisIndex < otherIndex {
		return fmt.Errorf("access denied: insufficient role: %q", r)
	}

	return nil
}

// Previous returns the previous role - i.e., the role with fewer capabilities.
func (r Role) Previous() (Role, error) {
	index, ok := indexes[r]
	if !ok {
		return "", fmt.Errorf("unknown current role in 'previous' check: %q", r)
	}

	if index == 0 {
		return "", fmt.Errorf("no 'previous' role for %q", r)
	}

	return roles[index-1], nil
}

// Min returns the least capable role from the given roles.
func Min(first Role, role ...Role) (Role, error) {
	result := first

	resultIndex, ok := indexes[first]
	if !ok {
		return "", fmt.Errorf("unknown first role in min check: %q", first)
	}

	for i, currentRole := range role {
		currentIndex, currentIndexOk := indexes[currentRole]
		if !currentIndexOk {
			return "", fmt.Errorf("unknown role in min check at index %d: %q", i, currentRole)
		}

		if currentIndex < resultIndex {
			result = currentRole
			resultIndex = currentIndex
		}
	}

	return result, nil
}

// Max returns the most capable role from the given roles.
func Max(first Role, role ...Role) (Role, error) {
	result := first

	resultIndex, ok := indexes[first]
	if !ok {
		return "", fmt.Errorf("unknown first role in max check: %q", first)
	}

	for i, currentRole := range role {
		currentIndex, currentIndexOk := indexes[currentRole]
		if !currentIndexOk {
			return "", fmt.Errorf("unknown role in max check at index %d: %q", i, currentRole)
		}

		if currentIndex > resultIndex {
			result = currentRole
			resultIndex = currentIndex
		}
	}

	return result, nil
}
