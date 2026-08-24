// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// SchematicOptions is used during schematic ID generation.
type SchematicOptions struct {
	overlay               *schematic.Overlay
	embeddedMachineConfig string
	kernelArgs            []string
	extensions            []string
	metaValues            []schematic.MetaValue
	skipConnectionParams  bool
}

// SchematicOption is the optional argument to the GetSchematicID method.
type SchematicOption func(*SchematicOptions)

// WithoutConnectionParams generates the schematic without embedding connection params into the kernel args.
// This flag might be useful for providers which use PXE to boot the machines, so the schematics won't need
// the parameters for Omni connection. This can allow to minimize the amount of schematics needed to be generated for the provider.
func WithoutConnectionParams() SchematicOption {
	return func(so *SchematicOptions) {
		so.skipConnectionParams = true
	}
}

// WithExtraExtensions adds more extensions to the schematic.
// The provider can detect the hardware and install some extensions automatically using this method.
func WithExtraExtensions(extensions ...string) SchematicOption {
	return func(so *SchematicOptions) {
		so.extensions = extensions
	}
}

// WithMetaValues adds meta values to the generated schematic.
// If the meta values with the same names are already set they are overwritten.
func WithMetaValues(values ...schematic.MetaValue) SchematicOption {
	return func(so *SchematicOptions) {
		so.metaValues = values
	}
}

// WithExtraKernelArgs adds kernel args to the schematic.
// This method doesn't remove duplicate kernel arguments.
func WithExtraKernelArgs(args ...string) SchematicOption {
	return func(so *SchematicOptions) {
		so.kernelArgs = args
	}
}

// WithOverlay sets the overlay on the schematic.
func WithOverlay(overlay schematic.Overlay) SchematicOption {
	return func(so *SchematicOptions) {
		so.overlay = &overlay
	}
}

// WithEmbeddedMachineConfig adds embedded machine config to the schematic.
func WithEmbeddedMachineConfig(config string) SchematicOption {
	return func(so *SchematicOptions) {
		so.embeddedMachineConfig = config
	}
}

// ConnectionParams represents kernel params and join config for making the machine join Omni.
type ConnectionParams struct {
	JoinConfig string
	KernelArgs []string

	CustomDataEncoded bool
}

// BootAssetSpec names the boot asset a provision step wants, without saying how the image factory
// spells it, and says how the provider intends to fetch it.
type BootAssetSpec struct {
	// AssetSpec names the asset. It is embedded rather than restated so that a field added to it reaches
	// every caller, instead of being dropped by one of the conversions along the way.
	imagefactory.AssetSpec

	// DownloadTokenTTL is how long this provider needs the URL to keep working for a factory that
	// authenticates downloads with a token. Zero takes Omni's default, which assumes the fetch happens
	// right away.
	DownloadTokenTTL time.Duration

	// StandaloneURL asks for a URL that needs no headers, for a fetch this provider does not perform
	// itself: a hypervisor download API handed a bare URL, for instance. Any authentication then
	// travels inside the URL and Headers comes back empty.
	//
	// Leaving it unset does not promise headers: a factory that authenticates downloads with a token puts
	// it in the URL either way. Send Headers whenever it is not empty, instead of deciding from this field.
	StandaloneURL bool
}

// BootAssetResolver ensures the schematic exists and returns the boot asset built from it.
//
// The infra library wires one that goes through Omni's management API, so that newer Omni versions can
// apply image factory changes this library predates.
type BootAssetResolver func(ctx context.Context, talosVersion string, schematic schematic.Schematic, spec BootAssetSpec) (imagefactory.BootAsset, error)

// NewContext creates a new provision context.
func NewContext[T resource.Resource](
	machineRequest *infra.MachineRequest,
	machineRequestStatus *infra.MachineRequestStatus,
	providerState T,
	connectionParams ConnectionParams,
	runtime controller.QRuntime,
	bootAssetResolver BootAssetResolver,
) Context[T] {
	return Context[T]{
		machineRequest:       machineRequest,
		MachineRequestStatus: machineRequestStatus,
		State:                providerState,
		ConnectionParams:     connectionParams,
		runtime:              runtime,
		bootAssetResolver:    bootAssetResolver,
	}
}

// Context keeps all context which might be required for the provision calls.
type Context[T resource.Resource] struct {
	machineRequest       *infra.MachineRequest
	MachineRequestStatus *infra.MachineRequestStatus
	runtime              controller.QRuntime
	bootAssetResolver    BootAssetResolver
	State                T
	ConnectionParams     ConnectionParams
}

// EnsureBootAsset returns the boot asset named by the spec, for the machine request's Talos version and
// the schematic generated from the machine request and the given options.
//
// It ensures the schematic exists on the image factory Omni is configured with and then resolves the
// asset, so a provision step never handles a schematic ID, a factory URL, or a filename convention.
//
// Fetch BootAsset.URL, sending BootAsset.Headers when they are not empty. See imagefactory.BootAsset
// for the full contract: the URL is opaque and can carry secrets, so it does not belong in a log line
// verbatim, it is not a stable input for a persisted name, and it can be short-lived, so use it
// promptly rather than storing it. BootAsset.ExpiresAt says when it stops working, for a provider that
// hands it on rather than fetching it, and BootAssetSpec.DownloadTokenTTL asks for a longer one.
func (context *Context[T]) EnsureBootAsset(ctx context.Context, logger *zap.Logger, spec BootAssetSpec, opts ...SchematicOption) (imagefactory.BootAsset, error) {
	// Built before the resolver is checked, so that a schematic the options cannot produce is reported as
	// such rather than as a missing resolver.
	schematic, err := context.buildSchematic(logger, opts...)
	if err != nil {
		return imagefactory.BootAsset{}, err
	}

	// The resolver goes through Omni, which owns the schematic upload as well as naming and locating the
	// asset. A provider given only a COSI state has no way to reach it and has to supply its own.
	if context.bootAssetResolver == nil {
		return imagefactory.BootAsset{}, errors.New("provision context has no boot asset resolver, the infra provider needs an Omni API client")
	}

	return context.bootAssetResolver(ctx, context.GetTalosVersion(), schematic, spec)
}

// GetRequestID returns machine request id.
func (context *Context[T]) GetRequestID() string {
	return context.machineRequest.Metadata().ID()
}

// GetTalosVersion returns Talos version from the machine request.
func (context *Context[T]) GetTalosVersion() string {
	return context.machineRequest.TypedSpec().Value.TalosVersion
}

// GetMachineRequestSetID returns the machine request set ID.
func (context *Context[T]) GetMachineRequestSetID() (string, bool) {
	return context.machineRequest.Metadata().Labels().Get(omni.LabelMachineRequestSet)
}

// SetMachineUUID in the machine request status.
func (context *Context[T]) SetMachineUUID(value string) {
	context.MachineRequestStatus.TypedSpec().Value.Id = value
}

// SetMachineInfraID in the machine request status.
func (context *Context[T]) SetMachineInfraID(value string) {
	context.MachineRequestStatus.Metadata().Labels().Set(omni.LabelMachineInfraID, value)
}

// UnmarshalProviderData reads provider data string from the machine request into the dest.
func (context *Context[T]) UnmarshalProviderData(dest any) error {
	if context.machineRequest.TypedSpec().Value.ProviderData == "" {
		return nil
	}

	return yaml.Unmarshal([]byte(context.machineRequest.TypedSpec().Value.ProviderData), dest)
}

// CreateConfigPatch for the provisioned machine.
func (context *Context[T]) CreateConfigPatch(ctx context.Context, name string, data []byte) error {
	r := infra.NewConfigPatchRequest(name)

	providerID, ok := context.machineRequest.Metadata().Labels().Get(omni.LabelInfraProviderID)
	if !ok {
		return errors.New("infra provider id is not set on the machine request")
	}

	return safe.WriterModify(ctx, context.runtime, r, func(r *infra.ConfigPatchRequest) error {
		r.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)
		r.Metadata().Labels().Set(omni.LabelMachineRequest, context.GetRequestID())

		return r.TypedSpec().Value.SetUncompressedData(data)
	})
}

// buildSchematic assembles the schematic out of the machine request and the given options.
func (context *Context[T]) buildSchematic(logger *zap.Logger, opts ...SchematicOption) (schematic.Schematic, error) {
	var schematicOptions SchematicOptions

	for _, o := range opts {
		o(&schematicOptions)
	}

	res := schematic.Schematic{
		Customization: schematic.Customization{
			ExtraKernelArgs: context.machineRequest.TypedSpec().Value.KernelArgs,
			Meta: xslices.Map(context.machineRequest.TypedSpec().Value.MetaValues, func(v *specs.MetaValue) schematic.MetaValue {
				return schematic.MetaValue{
					Key:   uint8(v.Key),
					Value: v.Value,
				}
			}),
			SystemExtensions: schematic.SystemExtensions{
				OfficialExtensions: context.machineRequest.TypedSpec().Value.Extensions,
			},
			EmbeddedMachineConfiguration: schematicOptions.embeddedMachineConfig,
		},
	}

	for _, extension := range schematicOptions.extensions {
		if slices.Index(res.Customization.SystemExtensions.OfficialExtensions, extension) != -1 {
			continue
		}

		res.Customization.SystemExtensions.OfficialExtensions = append(res.Customization.SystemExtensions.OfficialExtensions, extension)
	}

	slices.Sort(res.Customization.SystemExtensions.OfficialExtensions)

	for _, metaValue := range schematicOptions.metaValues {
		index := slices.IndexFunc(res.Customization.Meta, func(v schematic.MetaValue) bool {
			return v.Key == metaValue.Key
		})

		if index == -1 {
			res.Customization.Meta = append(res.Customization.Meta, metaValue)

			continue
		}

		res.Customization.Meta[index] = metaValue
	}

	switch {
	case schematicOptions.overlay != nil:
		res.Overlay = *schematicOptions.overlay
	case context.machineRequest.TypedSpec().Value.Overlay != nil:
		res.Overlay = schematic.Overlay{
			Image: context.machineRequest.TypedSpec().Value.Overlay.Image,
			Name:  context.machineRequest.TypedSpec().Value.Overlay.Name,
		}
	}

	if !schematicOptions.skipConnectionParams {
		if context.ConnectionParams.CustomDataEncoded {
			return schematic.Schematic{}, errors.New(`the provider is configured to embed connection parameters into the schematic, but it also includes a machine request ID, which is not allowed
in the connection parameters in the schematic. If the machine request ID must be part of the connection parameters,
provide them to the machine through another mechanism using the infrastructure provider`)
		}

		res.Customization.ExtraKernelArgs = append(
			res.Customization.ExtraKernelArgs,
			context.ConnectionParams.KernelArgs...,
		)
	}

	slices.Sort(res.Customization.ExtraKernelArgs)

	res.Customization.ExtraKernelArgs = append(res.Customization.ExtraKernelArgs, schematicOptions.kernelArgs...)

	// An embedded machine config is a Talos machine config, so it can carry cluster secrets and join
	// tokens. Log the schematic without it.
	logged := res
	if logged.Customization.EmbeddedMachineConfiguration != "" {
		logged.Customization.EmbeddedMachineConfiguration = "<redacted>"
	}

	logger.Info("creating schematic", zap.Reflect("schematic", logged))

	return res, nil
}
