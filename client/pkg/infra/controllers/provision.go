// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controllers

import (
	"context"
	"fmt"
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
	infrares "github.com/siderolabs/omni/client/pkg/infra/internal/resources"
	"github.com/siderolabs/omni/client/pkg/infra/provision"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

const currentStepAnnotation = "infra." + omni.SystemLabelPrefix + "step"

// ProvisionController is the generic controller that operates the Provisioner.
type ProvisionController[T generic.ResourceWithRD] struct {
	generic.NamedController
	provisioner  provision.Provisioner[T]
	imageFactory provision.FactoryClient
	providerID   string
	concurrency  uint
}

// NewProvisionController creates new ProvisionController.
func NewProvisionController[T generic.ResourceWithRD](providerID string, provisioner provision.Provisioner[T], concurrency uint,
	imageFactory provision.FactoryClient,
) *ProvisionController[T] {
	return &ProvisionController[T]{
		NamedController: generic.NamedController{
			ControllerName: providerID + ".ProvisionController",
		},
		providerID:   providerID,
		provisioner:  provisioner,
		concurrency:  concurrency,
		imageFactory: imageFactory,
	}
}

// Settings implements controller.QController interface.
func (ctrl *ProvisionController[T]) Settings() controller.QSettings {
	var t T

	return controller.QSettings{
		Inputs: []controller.Input{
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
				Namespace: resources.DefaultNamespace,
				Type:      siderolink.ConnectionParamsType,
				Kind:      controller.InputQMapped,
				ID:        optional.Some(siderolink.ConfigID),
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
		},
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
	r controller.QRuntime, ptr resource.Pointer,
) ([]resource.Pointer, error) {
	var t T

	switch ptr.Type() {
	case siderolink.ConnectionParamsType:
		return nil, nil
	case infra.ConfigPatchRequestType:
		configPatchRequest, err := safe.ReaderGetByID[*infra.ConfigPatchRequest](ctx, r, ptr.ID())
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, nil
			}

			return nil, err
		}

		id, ok := configPatchRequest.Metadata().Labels().Get(omni.LabelMachineRequest)
		if !ok {
			return nil, err
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
	destroyReady := true

	patches, err := safe.ReaderListAll[*infra.ConfigPatchRequest](ctx, r, state.WithLabelQuery(
		resource.LabelEqual(omni.LabelInfraProviderID, ctrl.providerID),
		resource.LabelEqual(omni.LabelMachineRequest, requestID),
	))
	if err != nil {
		return false, err
	}

	for request := range patches.All() {
		ready, err := r.Teardown(ctx, request.Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return false, err
		}

		if !ready {
			destroyReady = false

			continue
		}

		if err = r.Destroy(ctx, request.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return false, err
		}
	}

	return destroyReady, nil
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
	connectionParams, err := safe.ReaderGetByID[*siderolink.ConnectionParams](ctx, r, siderolink.ConfigID)
	if err != nil {
		return provision.ConnectionParams{}, err
	}

	kernelArgs, err := siderolink.GetConnectionArgsForProvider(connectionParams, ctrl.providerID, request.TypedSpec().Value.GrpcTunnel)
	if err != nil {
		return provision.ConnectionParams{}, err
	}

	joinConfig, err := siderolink.GetJoinConfigForProvider(connectionParams, ctrl.providerID, request.TypedSpec().Value.GrpcTunnel)
	if err != nil {
		return provision.ConnectionParams{}, err
	}

	return provision.ConnectionParams{
		KernelArgs: kernelArgs,
		JoinConfig: joinConfig,
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

	resources := []resource.Metadata{
		resource.NewMetadata(t.ResourceDefinition().DefaultNamespace, t.ResourceDefinition().Type, machineRequest.Metadata().ID(), resource.VersionUndefined),
		*infra.NewMachineRequestStatus(machineRequest.Metadata().ID()).Metadata(),
	}

	for _, md := range resources {
		var ready bool

		ready, err = r.Teardown(ctx, md)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !ready {
			return nil
		}

		err = r.Destroy(ctx, md)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}
	}

	if err = ctrl.provisioner.Deprovision(ctx, logger, t, machineRequest); err != nil {
		return err
	}

	logger.Info("machine deprovisioned", zap.String("request_id", machineRequest.Metadata().ID()))

	return r.RemoveFinalizer(ctx, machineRequest.Metadata(), ctrl.Name())
}
