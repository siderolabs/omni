// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/infra/provision"
)

// HealthCheckFunc defines a function that checks the health of the infra provider.
type HealthCheckFunc func(context.Context) error

// Options defines additional infra provider options.
type Options struct {
	state                      state.State
	imageFactory               provision.FactoryClient
	healthCheckFunc            HealthCheckFunc
	omniEndpoint               string
	clientOptions              []client.Option
	concurrency                uint
	healthCheckInterval        time.Duration
	encodeRequestIDsIntoTokens bool
}

// Option define an additional infra provider option.
type Option func(*Options)

// WithClientOptions defines custom options for the Omni API client.
func WithClientOptions(options ...client.Option) Option {
	return func(o *Options) {
		o.clientOptions = options
	}
}

// WithImageFactoryClient sets up the image factory client explicitly.
func WithImageFactoryClient(imageFactory provision.FactoryClient) Option {
	return func(o *Options) {
		o.imageFactory = imageFactory
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

// WithHealthCheckFunc sets the health check function for the infra provider.
//
// The health check function should return a descriptive error if the provider is unhealthy.
func WithHealthCheckFunc(healthCheckFunc HealthCheckFunc) Option {
	return func(o *Options) {
		o.healthCheckFunc = healthCheckFunc
	}
}

// WithHealthCheckInterval sets the health check interval for the infra provider.
func WithHealthCheckInterval(interval time.Duration) Option {
	return func(o *Options) {
		o.healthCheckInterval = interval
	}
}

// WithEncodeRequestIDsIntoTokens enables encoding the request IDs into tokens.
// This eliminates the need for setting the node UUID on the MachineRequestStatus.
// Omni will be able to map the machine request to the link immediately as the machine joins.
//
// NOTE: Only use this configuration when the join config is supplied though a metadata server, nocloud image or similar flows.
// Trying to encode that into a machine schematic will cause the provision to fail.
func WithEncodeRequestIDsIntoTokens() Option {
	return func(o *Options) {
		o.encodeRequestIDsIntoTokens = true
	}
}
