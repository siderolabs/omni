// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package infra contains boilerplate code for the infra provider implementations.
package infra

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/omni"
	"github.com/siderolabs/omni/client/pkg/infra/controllers"
	"github.com/siderolabs/omni/client/pkg/infra/imagefactory"
	"github.com/siderolabs/omni/client/pkg/infra/internal/resources"
	"github.com/siderolabs/omni/client/pkg/infra/provision"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
)

// ProviderConfig defines the schema, human-readable provider name and description.
type ProviderConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Icon        string `yaml:"icon,omitempty"`
	Schema      string `yaml:"schema"`
}

// ParseProviderConfig loads provider config from the yaml data.
func ParseProviderConfig(data []byte) (ProviderConfig, error) {
	var cfg ProviderConfig

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// RD defines contract for the resource definition.
type RD[V any] interface {
	generic.ResourceWithRD
	protobuf.ResourceUnmarshaler
	*V
}

// Provider runner.
type Provider[T generic.ResourceWithRD] struct {
	provisioner provision.Provisioner[T]
	config      ProviderConfig
	id          string
}

// NewProvider creates a new infra provider and registers provider state resource in the COSI state.
func NewProvider[V any, T RD[V]](
	id string,
	provisioner provision.Provisioner[T],
	config ProviderConfig,
) (*Provider[T], error) {
	var zero V

	t := T(&zero)

	if err := protobuf.RegisterResource(
		t.ResourceDefinition().Type,
		t,
	); err != nil && !strings.Contains(err.Error(), "is already registered") {
		return nil, err
	}

	return &Provider[T]{
		provisioner: provisioner,
		id:          id,
		config:      config,
	}, nil
}

// Run the infra provider.
func (provider *Provider[T]) Run(ctx context.Context, logger *zap.Logger, opts ...Option) error {
	var options Options

	for _, o := range opts {
		o(&options)
	}

	var st state.State

	if options.concurrency == 0 {
		options.concurrency = 1
	}

	if options.imageFactory == nil {
		var err error

		options.imageFactory, err = imagefactory.NewClient(imagefactory.ClientOptions{})
		if err != nil {
			return err
		}
	}

	options.clientOptions = append(options.clientOptions, client.WithOmniClientOptions(
		omni.WithProviderID(provider.id),
	))

	switch {
	case options.state != nil:
		st = options.state
	case options.omniEndpoint != "":
		client, err := client.New(options.omniEndpoint, options.clientOptions...)
		if err != nil {
			return err
		}

		state, err := NewState(client)
		if err != nil {
			return err
		}

		defer client.Close() //nolint:errcheck

		st = state.State()
	default:
		return fmt.Errorf("invalid infra provider configuration: either WithOmniEndpoint or WithState option should be used")
	}

	runtime, err := runtime.NewRuntime(st, logger)
	if err != nil {
		return err
	}

	rds, err := getResourceDefinitions(ctx, st)
	if err != nil {
		return err
	}

	if err = runtime.RegisterQController(controllers.NewProvisionController(
		provider.id,
		provider.provisioner,
		options.concurrency,
		options.imageFactory,
		options.encodeRequestIDsIntoTokens,
		rds,
	)); err != nil {
		return err
	}

	providerHealthStatusController, err := controllers.NewProviderHealthStatusController(provider.id, controllers.ProviderHealthStatusOptions{
		HealthCheckFunc: options.healthCheckFunc,
		Interval:        options.healthCheckInterval,
	})
	if err != nil {
		return err
	}

	if err = runtime.RegisterController(providerHealthStatusController); err != nil {
		return err
	}

	providerStatus := infra.NewProviderStatus(provider.id)

	providerStatus.TypedSpec().Value.Schema = provider.config.Schema
	providerStatus.TypedSpec().Value.Name = provider.config.Name
	providerStatus.TypedSpec().Value.Description = provider.config.Description
	providerStatus.TypedSpec().Value.Icon = provider.config.Icon

	err = st.Create(ctx, providerStatus)
	if err != nil {
		if !state.IsConflictError(err) {
			return err
		}

		_, err = safe.StateUpdateWithConflicts(ctx, st, providerStatus.Metadata(), func(res *infra.ProviderStatus) error {
			res.TypedSpec().Value = providerStatus.TypedSpec().Value

			return nil
		})
		if err != nil {
			return err
		}
	}

	return runtime.Run(ctx)
}

func getResourceDefinitions(ctx context.Context, state state.State) (map[string]struct{}, error) {
	resp, err := state.List(ctx, resource.NewMetadata(meta.NamespaceName, meta.ResourceDefinitionType, "", resource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	rds := make(map[string]struct{}, len(resp.Items))

	for _, rd := range resp.Items {
		rds[rd.Metadata().ID()] = struct{}{}
	}

	return rds, nil
}

// ResourceType generates the correct resource name for the resources managed by the infra providers.
func ResourceType(name, providerID string) string {
	return resources.ResourceType(name, providerID)
}

// ResourceNamespace generates the correct namespace name for the infra provider state.
func ResourceNamespace(providerID string) string {
	return resources.ResourceNamespace(providerID)
}
