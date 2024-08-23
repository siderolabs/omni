// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra

import (
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/client"
)

// Options defines additional infra provider options.
type Options struct {
	omniEndpoint  string
	state         state.State
	clientOptions []client.Option
	concurrency   uint
}

// Option define an additional infra provider option.
type Option func(*Options)

// WithClientOptions defines custom options for the Omni API client.
func WithClientOptions(options ...client.Option) Option {
	return func(o *Options) {
		o.clientOptions = options
	}
}

// WithConcurrency sets maximum provision concurrency on the controller.
func WithConcurrency(value uint) Option {
	return func(o *Options) {
		o.concurrency = value
	}
}

// WithOmniEndpoint sets Omni API client endpoint to use.
func WithOmniEndpoint(value string) Option {
	return func(o *Options) {
		o.omniEndpoint = value
	}
}

// WithState sets the COSI runtime state explicitly.
// If not set, the infra provider will create Omni API client and get the state from it.
// This option is intended to be used in the advanced use cases, when it's needed to create
// a custom state.
func WithState(state state.State) Option {
	return func(o *Options) {
		o.state = state
	}
}
