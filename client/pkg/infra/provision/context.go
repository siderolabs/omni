// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"context"
	"errors"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// FactoryClient ensures that the given schematic exists in the image factory.
type FactoryClient interface {
	EnsureSchematic(context.Context, schematic.Schematic) (string, error)
}

// SchematicOptions is used during schematic ID generation.
type SchematicOptions struct {
	overlay              *schematic.Overlay
	kernelArgs           []string
	extensions           []string
	metaValues           []schematic.MetaValue
	skipConnectionParams bool
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

// ConnectionParams represents kernel params and join config for making the machine join Omni.
type ConnectionParams struct {
	JoinConfig string
	KernelArgs []string

	CustomDataEncoded bool
}

// NewContext creates a new provision context.
func NewContext[T resource.Resource](
	machineRequest *infra.MachineRequest,
	machineRequestStatus *infra.MachineRequestStatus,
	state T,
	connectionParams ConnectionParams,
	imageFactory FactoryClient,
	runtime controller.QRuntime,
) Context[T] {
	return Context[T]{
		machineRequest:       machineRequest,
		MachineRequestStatus: machineRequestStatus,
		State:                state,
		ConnectionParams:     connectionParams,
		imageFactory:         imageFactory,
		runtime:              runtime,
	}
}

// Context keeps all context which might be required for the provision calls.
type Context[T resource.Resource] struct {
	machineRequest       *infra.MachineRequest
	imageFactory         FactoryClient
	MachineRequestStatus *infra.MachineRequestStatus
	runtime              controller.QRuntime
	State                T
	ConnectionParams     ConnectionParams
}

// GetRequestID returns machine request id.
func (context *Context[T]) GetRequestID() string {
	return context.machineRequest.Metadata().ID()
}

// GetTalosVersion returns Talos version from the machine request.
func (context *Context[T]) GetTalosVersion() string {
	return context.machineRequest.TypedSpec().Value.TalosVersion
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

// GenerateSchematicID generate the final schematic out of the machine request.
// This method also calls the image factory and uploads the schematic there.
func (context *Context[T]) GenerateSchematicID(ctx context.Context, logger *zap.Logger, opts ...SchematicOption) (string, error) {
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
			return "", errors.New(`the provider is configured to embed connection parameters into the schematic, but it also includes a machine request ID, which is not allowed
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

	logger.Info("creating schematic", zap.Reflect("schematic", res))

	return context.imageFactory.EnsureSchematic(ctx, res)
}
