// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controllers

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/cosi/helpers"
	infrares "github.com/siderolabs/omni/client/pkg/infra/internal/resources"
	"github.com/siderolabs/omni/client/pkg/infra/provision"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/siderolink"
)

const currentStepAnnotation = "infra." + omni.SystemLabelPrefix + "step"

// ProvisionController is the generic controller that operates the Provisioner.
type ProvisionController[T generic.ResourceWithRD] struct {
	generic.NamedController
	provisioner                provision.Provisioner[T]
	imageFactory               provision.FactoryClient
	providerID                 string
	concurrency                uint
	encodeRequestIDsIntoTokens bool
	useV2Tokens                bool
}

// NewProvisionController creates new ProvisionController.
func NewProvisionController[T generic.ResourceWithRD](providerID string, provisioner provision.Provisioner[T], concurrency uint,
	imageFactory provision.FactoryClient, encodeRequestIDsIntoTokens bool,
	resourceDefinitions map[string]struct{},
) *ProvisionController[T] {
	_, providerJoinConfigRegistered := resourceDefinitions[strings.ToLower(siderolinkres.ProviderJoinConfigType)]

	return &ProvisionController[T]{
		NamedController: generic.NamedController{
			ControllerName: providerID + ".ProvisionController",
		},
		providerID:                 providerID,
		provisioner:                provisioner,
		concurrency:                concurrency,
		imageFactory:               imageFactory,
		encodeRequestIDsIntoTokens: encodeRequestIDsIntoTokens,
		useV2Tokens:                providerJoinConfigRegistered,
	}
}

// Settings implements controller.QController interface.
func (ctrl *ProvisionController[T]) Settings() controller.QSettings {
	var t T

	inputs := []controller.Input{
		{
			Namespace: resources.InfraProviderNamespace,
			Type:      infra.MachineRequestType,
			Kind:      controller.InputQPrimary,
		},
		{
			Namespace: resources.InfraProviderNamespace,
			Type:      infra.MachineRequestStatusType,
			Kind:      controller.InputQMappedDestroyReady,
		},
		{
			Namespace: resources.InfraProviderNamespace,
			Type:      infra.MachineRegistrationType,
			Kind:      controller.InputQMapped,
		},
		{
			Namespace: resources.InfraProviderNamespace,
			Type:      infra.ConfigPatchRequestType,
			Kind:      controller.InputQMappedDestroyReady,
		},
		{
			Namespace: t.ResourceDefinition().DefaultNamespace,
			Type:      t.ResourceDefinition().Type,
			Kind:      controller.InputQMappedDestroyReady,
		},
	}

	if ctrl.useV2Tokens {
		inputs = append(inputs,
			controller.Input{
				Namespace: resources.InfraProviderNamespace,
				Type:      siderolinkres.ProviderJoinConfigType,
				Kind:      controller.InputQMapped,
				ID:        optional.Some(ctrl.providerID),
			},
			controller.Input{
				Namespace: resources.DefaultNamespace,
				Type:      siderolinkres.APIConfigType,
				Kind:      controller.InputQMapped,
				ID:        optional.Some(siderolinkres.ConfigID),
			},
		)
	} else {
		inputs = append(inputs,
			controller.Input{
				Namespace: resources.DefaultNamespace,
				Type:      siderolinkres.ConnectionParamsType,
				Kind:      controller.InputQMapped,
				ID:        optional.Some(siderolinkres.ConfigID),
			},
		)
	}

	return controller.QSettings{
		RunHook: func(ctx context.Context, _ *zap.Logger, q controller.QRuntime) error {
			return ctrl.cleanupDanglingMachines(ctx, q)
		},
		Inputs: inputs,
		Outputs: []controller.Output{
			{
				Kind: controller.OutputExclusive,
				Type: infra.MachineRequestStatusType,
			},
			{
				Kind: controller.OutputShared,
				Type: t.ResourceDefinition().Type,
			},
			{
				Kind: controller.OutputShared,
				Type: infra.ConfigPatchRequestType,
			},
		},
		Concurrency: optional.Some(ctrl.concurrency),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *ProvisionController[T]) MapInput(ctx context.Context, _ *zap.Logger,
	r controller.QRuntime, ptr controller.ReducedResourceMetadata,
) ([]resource.Pointer, error) {
	var t T

	switch ptr.Type() {
	case siderolinkres.ProviderJoinConfigType,
		siderolinkres.APIConfigType:
		return nil, nil
	case infra.MachineRegistrationType:
		machineRequest, ok := ptr.Labels().Get(omni.LabelMachineRequest)
		if !ok {
			return nil, nil
		}

		return []resource.Pointer{
			infra.NewMachineRequest(machineRequest).Metadata(),
		}, nil
	case infra.ConfigPatchRequestType:
		id, ok := ptr.Labels().Get(omni.LabelMachineRequest)
		if !ok {
			return nil, fmt.Errorf("label %q not found", omni.LabelMachineRequest)
		}

		return []resource.Pointer{
			infra.NewMachineRequest(id).Metadata(),
		}, nil
	case infra.MachineRequestType,
		infra.MachineRequestStatusType,
		t.ResourceDefinition().Type:
		return []resource.Pointer{
			infra.NewMachineRequest(ptr.ID()).Metadata(),
		}, nil
	}

	return nil, fmt.Errorf("got unexpected type %s", ptr.Type())
}

// Reconcile implements controller.QController interface.
func (ctrl *ProvisionController[T]) Reconcile(ctx context.Context,
	logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
) error {
	machineRequest, err := safe.ReaderGet[*infra.MachineRequest](ctx, r, infra.NewMachineRequest(ptr.ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if providerID, ok := machineRequest.Metadata().Labels().Get(omni.LabelInfraProviderID); !ok || providerID != ctrl.providerID {
		return nil
	}

	if machineRequest.Metadata().Phase() == resource.PhaseTearingDown {
		return ctrl.reconcileTearingDown(ctx, r, logger, machineRequest)
	}

	if !machineRequest.Metadata().Finalizers().Has(ctrl.Name()) {
		if err = r.AddFinalizer(ctx, machineRequest.Metadata(), ctrl.Name()); err != nil {
			return err
		}
	}

	machineRequestStatus, err := ctrl.initializeStatus(ctx, r, logger, machineRequest)
	if err != nil {
		return err
	}

	return safe.WriterModify(ctx, r, machineRequestStatus, func(res *infra.MachineRequestStatus) error {
		return ctrl.reconcileRunning(ctx, r, logger, machineRequest, res)
	})
}

//nolint:gocognit
func (ctrl *ProvisionController[T]) reconcileRunning(ctx context.Context, r controller.QRuntime, logger *zap.Logger,
	machineRequest *infra.MachineRequest, machineRequestStatus *infra.MachineRequestStatus,
) error {
	connectionParams, err := ctrl.getConnectionArgs(ctx, r, machineRequest)
	if err != nil {
		return err
	}

	var t T

	md := resource.NewMetadata(infrares.ResourceNamespace(ctrl.providerID), t.ResourceDefinition().Type, machineRequest.Metadata().ID(), resource.VersionUndefined)

	var res resource.Resource

	res, err = r.Get(ctx, md)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if res == nil {
		res, err = protobuf.CreateResource(t.ResourceDefinition().Type)
		if err != nil {
			return err
		}

		*res.Metadata() = md

		// initialize empty spec
		if r, ok := res.Spec().(interface {
			UnmarshalJSON(bytes []byte) error
		}); ok {
			if err = r.UnmarshalJSON([]byte("{}")); err != nil {
				return err
			}
		}
	}

	if machineRequestStatus.TypedSpec().Value.Id == "" {
		var machines safe.List[*infra.MachineRegistration]

		machines, err = safe.ReaderListAll[*infra.MachineRegistration](ctx, r, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelMachineRequest, machineRequest.Metadata().ID())),
		)
		if err != nil {
			return err
		}

		if machines.Len() == 1 {
			logger.Info("setting machine request UUID", zap.String("machine", machines.Get(0).Metadata().ID()))

			machineRequestStatus.TypedSpec().Value.Id = machines.Get(0).Metadata().ID()
		}
	}

	// nothing to do as the machine was already provisioned
	if machineRequestStatus.TypedSpec().Value.Stage == specs.MachineRequestStatusSpec_PROVISIONED {
		return nil
	}

	steps := ctrl.provisioner.ProvisionSteps()

	initialStep, _ := res.Metadata().Annotations().Get(currentStepAnnotation)

	for i, step := range steps {
		if initialStep != "" && step.Name() != initialStep {
			continue
		}

		initialStep = ""

		logger.Info("running provision step", zap.String("step", step.Name()))

		var requeueError error

		machineRequestStatus.TypedSpec().Value.Status = fmt.Sprintf("Running Step: %q (%d/%d)", step.Name(), i+1, len(steps))
		machineRequestStatus.TypedSpec().Value.Error = ""
		machineRequestStatus.TypedSpec().Value.Stage = specs.MachineRequestStatusSpec_PROVISIONING

		if err = safe.WriterModify(ctx, r, res.(T), func(st T) error { //nolint:forcetypeassert,errcheck
			err = step.Run(ctx, logger, provision.NewContext(
				machineRequest,
				machineRequestStatus,
				st,
				connectionParams,
				ctrl.imageFactory,
				r,
			))

			st.Metadata().Annotations().Set(currentStepAnnotation, step.Name())

			if err != nil {
				if !xerrors.TypeIs[*controller.RequeueError](err) {
					return err
				}

				requeueError = err
			}

			return nil
		}); err != nil {
			logger.Error("machine provision failed", zap.Error(err), zap.String("step", step.Name()))

			machineRequestStatus.TypedSpec().Value.Error = err.Error()
			machineRequestStatus.TypedSpec().Value.Stage = specs.MachineRequestStatusSpec_FAILED

			return controller.NewRequeueError(err, time.Minute)
		}

		if err = safe.WriterModify(ctx, r, machineRequestStatus, func(res *infra.MachineRequestStatus) error {
			res.TypedSpec().Value = machineRequestStatus.TypedSpec().Value

			return nil
		}); err != nil {
			return err
		}

		if requeueError != nil {
			return requeueError
		}
	}

	machineRequestStatus.TypedSpec().Value.Stage = specs.MachineRequestStatusSpec_PROVISIONED
	machineRequestStatus.TypedSpec().Value.Status = "Provision Complete"

	*machineRequestStatus.Metadata().Labels() = *machineRequest.Metadata().Labels()

	logger.Info("machine provision finished")

	return nil
}

func (ctrl *ProvisionController[T]) removePatches(ctx context.Context, r controller.QRuntime, requestID string) (bool, error) {
	patches, err := safe.ReaderListAll[*infra.ConfigPatchRequest](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelInfraProviderID, ctrl.providerID),
		resource.LabelEqual(omni.LabelMachineRequest, requestID),
	))
	if err != nil {
		return false, err
	}

	return helpers.TeardownAndDestroyAll(ctx, r, patches.Pointers())
}

func (ctrl *ProvisionController[T]) initializeStatus(ctx context.Context, r controller.QRuntime, logger *zap.Logger, machineRequest *infra.MachineRequest) (*infra.MachineRequestStatus, error) {
	mrs, err := safe.ReaderGetByID[*infra.MachineRequestStatus](ctx, r, machineRequest.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if mrs != nil {
		return mrs, nil
	}

	return safe.WriterModifyWithResult(ctx, r, infra.NewMachineRequestStatus(machineRequest.Metadata().ID()), func(res *infra.MachineRequestStatus) error {
		if res.TypedSpec().Value.Stage == specs.MachineRequestStatusSpec_UNKNOWN {
			res.TypedSpec().Value.Stage = specs.MachineRequestStatusSpec_PROVISIONING
			*res.Metadata().Labels() = *machineRequest.Metadata().Labels()

			logger.Info("machine provision started", zap.String("request_id", machineRequest.Metadata().ID()))
		}

		return nil
	})
}

func (ctrl *ProvisionController[T]) getConnectionArgs(ctx context.Context, r controller.QRuntime, request *infra.MachineRequest) (provision.ConnectionParams, error) {
	var options []siderolink.JoinConfigOption

	if ctrl.useV2Tokens {
		providerJoinConfig, err := safe.ReaderGetByID[*siderolinkres.ProviderJoinConfig](ctx, r, ctrl.providerID)
		if err != nil {
			return provision.ConnectionParams{}, err
		}

		siderolinkAPIConfig, err := safe.ReaderGetByID[*siderolinkres.APIConfig](ctx, r, siderolinkres.ConfigID)
		if err != nil {
			return provision.ConnectionParams{}, err
		}

		options = []siderolink.JoinConfigOption{
			siderolink.WithJoinToken(providerJoinConfig.TypedSpec().Value.JoinToken),
			siderolink.WithMachineAPIURL(siderolinkAPIConfig.TypedSpec().Value.MachineApiAdvertisedUrl),
			siderolink.WithGRPCTunnel(request.TypedSpec().Value.GrpcTunnel == specs.GrpcTunnelMode_ENABLED),
			siderolink.WithEventSinkPort(int(siderolinkAPIConfig.TypedSpec().Value.EventsPort)),
			siderolink.WithLogServerPort(int(siderolinkAPIConfig.TypedSpec().Value.LogsPort)),
			siderolink.WithProvider(infra.NewProvider(ctrl.providerID)),
		}
	} else {
		// legacy flow
		connectionParams, err := safe.ReaderGetByID[*siderolinkres.ConnectionParams](ctx, r, siderolinkres.ConfigID)
		if err != nil {
			return provision.ConnectionParams{}, err
		}

		options = []siderolink.JoinConfigOption{
			siderolink.WithJoinToken(connectionParams.TypedSpec().Value.JoinToken),
			siderolink.WithMachineAPIURL(connectionParams.TypedSpec().Value.ApiEndpoint),
			siderolink.WithGRPCTunnel(request.TypedSpec().Value.GrpcTunnel == specs.GrpcTunnelMode_ENABLED),
			siderolink.WithEventSinkPort(int(connectionParams.TypedSpec().Value.EventsPort)),
			siderolink.WithLogServerPort(int(connectionParams.TypedSpec().Value.LogsPort)),
			siderolink.WithProvider(infra.NewProvider(ctrl.providerID)),
			siderolink.WithJoinTokenVersion(jointoken.Version1),
		}
	}

	if ctrl.encodeRequestIDsIntoTokens {
		options = append(options, siderolink.WithMachineRequestID(request.Metadata().ID()))
	}

	opts, err := siderolink.NewJoinOptions(options...)
	if err != nil {
		return provision.ConnectionParams{}, err
	}

	kernelArgs := opts.GetKernelArgs()

	joinConfig, err := opts.RenderJoinConfig()
	if err != nil {
		return provision.ConnectionParams{}, err
	}

	return provision.ConnectionParams{
		KernelArgs:        kernelArgs,
		JoinConfig:        string(joinConfig),
		CustomDataEncoded: ctrl.encodeRequestIDsIntoTokens,
	}, nil
}

func (ctrl *ProvisionController[T]) reconcileTearingDown(ctx context.Context, r controller.QRuntime, logger *zap.Logger, machineRequest *infra.MachineRequest) error {
	t, err := safe.ReaderGetByID[T](ctx, r, machineRequest.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	{
		var ready bool

		if ready, err = ctrl.removePatches(ctx, r, machineRequest.Metadata().ID()); err != nil {
			return err
		}

		if !ready {
			return nil
		}
	}

	resources := []resource.Pointer{
		resource.NewMetadata(t.ResourceDefinition().DefaultNamespace, t.ResourceDefinition().Type, machineRequest.Metadata().ID(), resource.VersionUndefined),
		infra.NewMachineRequestStatus(machineRequest.Metadata().ID()).Metadata(),
	}

	destroyReady, err := helpers.TeardownAndDestroyAll(ctx, r, slices.Values(resources))
	if err != nil {
		return err
	}

	if !destroyReady {
		return nil
	}

	if err = ctrl.provisioner.Deprovision(ctx, logger, t, machineRequest); err != nil {
		return err
	}

	logger.Info("machine deprovisioned", zap.String("request_id", machineRequest.Metadata().ID()))

	return r.RemoveFinalizer(ctx, machineRequest.Metadata(), ctrl.Name())
}

func (ctrl *ProvisionController[T]) cleanupDanglingMachines(ctx context.Context, r controller.QRuntime) error {
	machines, err := safe.ReaderListAll[T](ctx, r)
	if err != nil {
		return err
	}

	for m := range machines.All() {
		var request *infra.MachineRequest

		request, err = safe.ReaderGetByID[*infra.MachineRequest](ctx, r, m.Metadata().ID())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		if request == nil {
			_, err = r.Teardown(ctx, m.Metadata())
			if err != nil && !state.IsNotFoundError(err) {
				return err
			}
		}
	}

	return nil
}
