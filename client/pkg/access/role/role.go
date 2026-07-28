// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package role contains the role definitions and checks.
package role

import (
	"fmt"
	"slices"
)

// Role represents a user role.
type Role string

const (
	// None is a role that has no capability.
	//
	// tsgen:RoleNone
	None Role = "None"

	// InfraProvider is a role to be used solely by infra providers.
	//
	// tsgen:RoleInfraProvider
	InfraProvider Role = "InfraProvider"

	// Reader is a role that has read-only capability.
	//
	// tsgen:RoleReader
	Reader Role = "Reader"

	// Auditor is a role that has read-only capability, plus the capability to read the audit log.
	//
	// Reading the audit log is restricted to this role and Admin, so it is checked by exact role match
	// rather than by the ordering below.
	//
	// tsgen:RoleAuditor
	Auditor Role = "Auditor"

	// Operator is a role that has read/write capability.
	//
	// tsgen:RoleOperator
	Operator Role = "Operator"

	// Admin is a role that has read/write and user/service account management capability.
	//
	// tsgen:RoleAdmin
	Admin Role = "Admin"
)

var roles = []Role{None, InfraProvider, Reader, Auditor, Operator, Admin}

// AuditLogRoles are the roles allowed to read the audit log.
//
// Operator outranks Auditor in the ordering above yet is deliberately absent, so audit log access is
// checked by exact role rather than by rank. This is the single source of that policy.
var AuditLogRoles = []Role{Auditor, Admin}

// CanReadAuditLog reports whether the role is allowed to read the audit log.
func CanReadAuditLog(r Role) bool {
	return slices.Contains(AuditLogRoles, r)
}

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

// Compare the roles.
func (r Role) Compare(another Role) int {
	r1Index, ok := indexes[r]
	if !ok {
		panic(fmt.Sprintf("unknown role %s", r))
	}

	r2Index, ok := indexes[another]
	if !ok {
		panic(fmt.Sprintf("unknown role %s", another))
	}

	switch {
	case r1Index < r2Index:
		return -1
	case r1Index > r2Index:
		return 1
	}

	return 0
}

// Min returns the least capable role from the given roles.
//
// Audit log access is not a point on the ordering: Auditor sits below Operator, yet only Auditor and Admin
// may read it. Capping by rank alone would therefore grant audit log access to a combination where no input
// had it, for example an Operator holding a key registered as Auditor. The result keeps audit log access
// only when every input has it, and falls back to Reader otherwise.
func Min(first Role, role ...Role) (Role, error) {
	result := first

	resultIndex, ok := indexes[first]
	if !ok {
		return "", fmt.Errorf("unknown first role in min check: %q", first)
	}

	canReadAuditLog := CanReadAuditLog(first)

	for i, currentRole := range role {
		currentIndex, currentIndexOk := indexes[currentRole]
		if !currentIndexOk {
			return "", fmt.Errorf("unknown role in min check at index %d: %q", i, currentRole)
		}

		canReadAuditLog = canReadAuditLog && CanReadAuditLog(currentRole)

		if currentIndex < resultIndex {
			result = currentRole
			resultIndex = currentIndex
		}
	}

	if result == Auditor && !canReadAuditLog {
		return Reader, nil
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
