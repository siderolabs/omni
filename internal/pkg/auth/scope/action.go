// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package scope

// Action represents an action in the scope.
type Action string

// Action constants.
const (
	ActionRead    Action = "read"
	ActionCreate  Action = "create"
	ActionModify  Action = "modify"
	ActionDestroy Action = "destroy"
	ActionAny     Action = "*"
)
