// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra

import (
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/client"
)

// State creates new cloud provider state.
type State struct {
	Client *client.Client
}

// NewState creates new cloud provider state.
func NewState(endpoint string, opts ...client.Option) (*State, error) {
	client, err := client.New(endpoint, opts...)
	if err != nil {
		return nil, err
	}

	return &State{Client: client}, nil
}

// Close closes the connection to Omni.
func (s *State) Close() error {
	return s.Client.Close()
}

// State returns COSI state.
func (s *State) State() state.State {
	return s.Client.Omni().State()
}
