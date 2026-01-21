// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package debug provides HTTP handlers in debug mode.
package debug

import (
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/state"
)

// Handler of imager requests.
type Handler struct {
	runtime *runtime.Runtime
	state   state.State
}

// NewHandler creates a new debug handler.
//
// Debug handler provides real implementation only when sidero.debug is enabled.
func NewHandler(runtime *runtime.Runtime, state state.State) *Handler {
	return &Handler{
		runtime: runtime,
		state:   state,
	}
}
