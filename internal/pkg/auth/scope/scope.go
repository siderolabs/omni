// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package scope contains the scope definitions and checks.
package scope

import (
	"fmt"
	"strings"
)

// Scope represents an authorization scope.
type Scope struct {
	// Object the access is granted to, e.g. `cluster`, or `machine`.
	Object Object
	// Action the access is granted for, e.g. `create`, or `delete`.
	Action Action
	// Perspective is the additional sub-action for specific object.
	Perspective Perspective
}

// New creates a new scope.
func New(object Object, action Action, perspective Perspective) Scope {
	return Scope{
		Object:      object,
		Action:      action,
		Perspective: perspective,
	}
}

// Parse parses a scope from string.
func Parse(scope string) (Scope, error) {
	obj, actionAndPerspective, ok := strings.Cut(scope, ":")
	if !ok {
		return Scope{}, fmt.Errorf("invalid scope %q", scope)
	}

	action, perspective, ok := strings.Cut(actionAndPerspective, ".")
	if !ok {
		return New(Object(obj), Action(action), PerspectiveNone), nil
	}

	return New(Object(obj), Action(action), Perspective(perspective)), nil
}

// Matches verifies if the required scope matches the actor scope.
func (s Scope) Matches(actorScope Scope) bool {
	return s.Object == actorScope.Object &&
		((s.Action == actorScope.Action && s.Perspective == actorScope.Perspective) || actorScope.Action == ActionAny)
}

// String converts Scope to string.
func (s Scope) String() string {
	action := string(s.Action)

	if s.Perspective != PerspectiveNone {
		action = action + "." + string(s.Perspective)
	}

	return string(s.Object) + ":" + action
}

// Set of standard scopes.
var (
	ClusterAny            = New(ObjectCluster, ActionAny, PerspectiveNone)
	UserAny               = New(ObjectUser, ActionAny, PerspectiveNone)
	MachineAny            = New(ObjectMachine, ActionAny, PerspectiveNone)
	ServiceAccountAny     = New(ObjectServiceAccount, ActionAny, PerspectiveNone)
	ClusterRead           = New(ObjectCluster, ActionRead, PerspectiveNone)
	ClusterModify         = New(ObjectCluster, ActionModify, PerspectiveNone)
	UserRead              = New(ObjectUser, ActionRead, PerspectiveNone)
	MachineRead           = New(ObjectMachine, ActionRead, PerspectiveNone)
	ServiceAccountRead    = New(ObjectServiceAccount, ActionRead, PerspectiveNone)
	ServiceAccountCreate  = New(ObjectServiceAccount, ActionCreate, PerspectiveNone)
	ServiceAccountDestroy = New(ObjectServiceAccount, ActionDestroy, PerspectiveNone)
)

// UserDefaultScopes is the set of scopes granted to users by default.
//
// This might change as we introduce more user roles.
var UserDefaultScopes = []Scope{
	ClusterAny,
	MachineAny,
	UserAny,
	ServiceAccountAny,
}

// SuspendedScopes is the set of scopes granted to users when the Omni account is suspended.
//
// This might change as we introduce more user roles.
var SuspendedScopes = []Scope{
	ClusterRead,
	MachineRead,
	UserRead,
	ServiceAccountRead,
}
