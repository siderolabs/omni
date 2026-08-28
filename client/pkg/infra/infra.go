// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package infra contains boilerplate code for the infra provider implementations.
package infra

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/client/omni"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/infra/controllers"
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

	// Installation media are resolved through Omni, so a provider given only a state has no way to reach one:
	// such a setup has to supply its own resolver.
	installationMediaResolver := options.installationMediaResolver
	if installationMediaResolver == nil && c != nil {
		installationMediaResolver = func(ctx context.Context, talosVersion string, schematic schematic.Schematic, spec provision.MediaSpec) (imagefactory.InstallationMedia, error) {
			return EnsureInstallationMedia(ctx, c, talosVersion, schematic, spec)
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
		options.encodeRequestIDsIntoTokens,
		rds,
		installationMediaResolver,
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
	providerStatus.TypedSpec().Value.Version = options.version

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

// EnsureSchematic uploads the schematic through Omni and returns the ID the image factory gave it.
//
// The schematic goes to whichever image factory Omni is configured with for the given Talos version, so
// a caller needs no factory endpoint or credentials of its own. An empty version means Omni's default
// Talos version.
func EnsureSchematic(ctx context.Context, c *client.Client, talosVersion string, schematic schematic.Schematic) (string, error) {
	raw, err := schematic.Marshal()
	if err != nil {
		return "", fmt.Errorf("failed to marshal the schematic: %w", err)
	}

	resp, err := c.Management().CreateSchematicFromRaw(ctx, raw, talosVersion)
	if err != nil {
		return "", schematicError(err)
	}

	return resp.SchematicId, nil
}

func schematicError(err error) error {
	if status.Code(err) == codes.Unimplemented {
		return fmt.Errorf("this infra provider requires Omni 1.8 or newer, which the server it is connected to does not appear to be: %w", err)
	}

	return fmt.Errorf("failed to create the schematic through Omni: %w", err)
}

// EnsureInstallationMedia ensures the schematic exists on the image factory Omni is configured with, and
// returns the installation medium the given spec identifies, built from it.
//
// Providers that run a provision step should call Context.EnsureInstallationMedia instead, which builds the
// schematic out of the machine request for them. This is for the ones that never build a provision
// context, such as a provider serving its own iPXE endpoint.
//
// Fetch InstallationMedia.URL, sending InstallationMedia.Headers when they are not empty. See imagefactory.InstallationMedia
// for the full contract.
func EnsureInstallationMedia(
	ctx context.Context,
	c *client.Client,
	talosVersion string,
	schematic schematic.Schematic,
	spec provision.MediaSpec,
) (imagefactory.InstallationMedia, error) {
	schematicID, err := EnsureSchematic(ctx, c, talosVersion, schematic)
	if err != nil {
		return imagefactory.InstallationMedia{}, err
	}

	return resolveInstallationMedia(ctx, c, talosVersion, spec, schematicID)
}

// installationMediaKinds maps the kinds this library names onto the wire enum.
var installationMediaKinds = map[imagefactory.InstallationMediaKind]management.InstallationMediaURLRequest_InstallationMediaKind{
	imagefactory.InstallationMediaKindPXE:  management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_PXE,
	imagefactory.InstallationMediaKindISO:  management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_ISO,
	imagefactory.InstallationMediaKindDisk: management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_DISK,
}

// resolveInstallationMedia asks Omni to locate the medium, where the server decides how it is
// authenticated. That is what lets factory auth schemes evolve without updating this library: the
// caller applies whatever the server returns.
//
// A server that predates the installation media API answers Unimplemented, and it is built here instead
// from the image factory configuration in Omni's state, which every version exposes.
func resolveInstallationMedia(
	ctx context.Context,
	c *client.Client,
	talosVersion string,
	spec provision.MediaSpec,
	schematicID string,
) (imagefactory.InstallationMedia, error) {
	request, err := installationMediaRequest(talosVersion, schematicID, spec)
	if err != nil {
		return imagefactory.InstallationMedia{}, err
	}

	resp, err := c.Management().GetInstallationMediaURL(ctx, request)
	if err != nil {
		if status.Code(err) != codes.Unimplemented {
			return imagefactory.InstallationMedia{}, fmt.Errorf("failed to get the installation media URL from Omni: %w", err)
		}

		return imagefactory.ResolveInstallationMedia(ctx, c.Omni().State(), talosVersion, spec.MediaSpec, schematicID, spec.StandaloneURL)
	}

	media := installationMediaFromResponse(resp)
	media.SchematicID = schematicID

	return media, nil
}

// installationMediaRequest converts a provision step's spec into the management API request.
func installationMediaRequest(talosVersion, schematicID string, spec provision.MediaSpec) (*management.InstallationMediaURLRequest, error) {
	kind, ok := installationMediaKinds[spec.Kind]
	if !ok {
		return nil, fmt.Errorf("unknown installation media kind %q", spec.Kind)
	}

	request := &management.InstallationMediaURLRequest{
		TalosVersion:          talosVersion,
		SchematicId:           schematicID,
		StandaloneUrl:         spec.StandaloneURL,
		InstallationMediaKind: kind,
		Platform:              spec.Platform,
		Architecture:          spec.Architecture,
		Format:                spec.Format,
		SecureBoot:            spec.SecureBoot,
	}

	// A zero duration on the wire would look like a request for no lifetime at all, so it stays unset and
	// Omni applies its own default.
	if spec.DownloadTokenTTL != 0 {
		request.DownloadTokenTtl = durationpb.New(spec.DownloadTokenTTL)
	}

	return request, nil
}

// installationMediaFromResponse converts the management API response into the client-side type.
func installationMediaFromResponse(resp *management.InstallationMediaURLResponse) imagefactory.InstallationMedia {
	var headers http.Header

	if len(resp.Headers) > 0 {
		headers = make(http.Header, len(resp.Headers))

		for name, value := range resp.Headers {
			headers.Set(name, value)
		}
	}

	var expiresAt time.Time

	// An Omni that predates the field, or a factory whose downloads do not expire, sends nothing, and
	// AsTime would turn that into the Unix epoch rather than the zero time the field documents.
	if resp.ExpiresAt != nil {
		expiresAt = resp.ExpiresAt.AsTime()
	}

	return imagefactory.InstallationMedia{
		URL:              resp.Url,
		Headers:          headers,
		StorageKey:       resp.StorageKey,
		ImageFactoryHost: resp.ImageFactoryHost,
		ExpiresAt:        expiresAt,
	}
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
