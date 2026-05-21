// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package infra contains boilerplate code for the infra provider implementations.
package infra

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/omni"
	"github.com/siderolabs/omni/client/pkg/infra/controllers"
	"github.com/siderolabs/omni/client/pkg/infra/imagefactory"
	"github.com/siderolabs/omni/client/pkg/infra/internal/resources"
	"github.com/siderolabs/omni/client/pkg/infra/provision"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
)

// proxiedImageFactoryMinVersion is the minimum Omni version that supports proxying image factory requests
// through the management API. Older servers don't expose CreateSchematicFromRaw, so the provider has to
// reach the image factory directly.
var proxiedImageFactoryMinVersion = semver.Version{Major: 1, Minor: 9}

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

	options.clientOptions = append(options.clientOptions, client.WithOmniClientOptions(
		omni.WithProviderID(provider.id),
	))

	var (
		c   *client.Client
		err error
	)

	switch {
	case options.state != nil:
		st = options.state
	case options.omniEndpoint != "":
		c, err = client.New(options.omniEndpoint, options.clientOptions...)
		if err != nil {
			return err
		}

		var state *State

		state, err = NewState(c)
		if err != nil {
			return err
		}

		defer c.Close() //nolint:errcheck

		st = state.State()
	default:
		return fmt.Errorf("invalid infra provider configuration: either WithOmniEndpoint or WithState option should be used")
	}

	if options.imageFactory == nil {
		options.imageFactory, err = newImageFactoryClient(ctx, c, st, logger)
		if err != nil {
			return err
		}
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

// newImageFactoryClient picks the image factory client implementation based on the Omni server version.
// Omni >= 1.9 supports proxying through the management API; older servers (and configurations without
// an Omni API client) require talking to the image factory directly. The endpoint is taken from
// FeaturesConfig.ImageFactoryBaseUrl when set, otherwise NewDirectClient falls back to the default
// public Talos image factory URL.
func newImageFactoryClient(ctx context.Context, c *client.Client, st state.State, logger *zap.Logger) (provision.FactoryClient, error) {
	useProxied := c != nil

	if useProxied {
		supported, err := supportsProxiedImageFactory(ctx, st, logger)
		if err != nil {
			return nil, err
		}

		useProxied = supported
	}

	if useProxied {
		return imagefactory.NewProxiedClient(c)
	}

	features, err := safe.ReaderGetByID[*omnires.FeaturesConfig](ctx, st, omnires.FeaturesConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get features config: %w", err)
	}

	return imagefactory.NewDirectClient(imagefactory.ClientOptions{
		FactoryEndpoint: features.TypedSpec().Value.ImageFactoryBaseUrl,
	})
}

// supportsProxiedImageFactory returns true if the Omni server is new enough to proxy image factory requests
// through the management API. If the version can't be determined it defaults to true so that the provider
// keeps using the new API path.
func supportsProxiedImageFactory(ctx context.Context, st state.State, logger *zap.Logger) (bool, error) {
	sysVersion, err := safe.ReaderGetByID[*system.SysVersion](ctx, st, system.SysVersionID)
	if err != nil {
		return false, fmt.Errorf("failed to get Omni system version: %w", err)
	}

	backendVersion := sysVersion.TypedSpec().Value.BackendVersion

	v, err := semver.ParseTolerant(strings.TrimPrefix(backendVersion, "v"))
	if err != nil {
		logger.Warn("failed to parse Omni backend version, assuming proxied image factory API is supported",
			zap.String("backend_version", backendVersion), zap.Error(err))

		return true, nil
	}

	return v.GTE(proxiedImageFactoryMinVersion), nil
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
