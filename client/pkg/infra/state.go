// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra

import (
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/client"
)

// State creates new infra provider state.
type State struct {
	Client *client.Client
}

// NewState creates new infra provider state.
func NewState(client *client.Client) (*State, error) {
	return &State{Client: client}, nil
}

// State returns COSI state.
func (s *State) State() state.State {
	return s.Client.Omni().State()
}
