// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package controllers

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	cloudspecs "github.com/siderolabs/omni/client/api/omni/specs/cloud"
	infrares "github.com/siderolabs/omni/client/pkg/infra/internal/resources"
	"github.com/siderolabs/omni/client/pkg/infra/provision"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/cloud"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// ProvisionController is the generic controller that operates the Provisioner.
type ProvisionController[T generic.ResourceWithRD] struct {
	generic.NamedController
	provisioner provision.Provisioner[T]
	providerID  string
	concurrency uint
}

// NewProvisionController creates new ProvisionController.
func NewProvisionController[T generic.ResourceWithRD](providerID string, provisioner provision.Provisioner[T], concurrency uint) *ProvisionController[T] {
	return &ProvisionController[T]{
		NamedController: generic.NamedController{
			ControllerName: providerID + ".ProvisionController",
		},
		providerID:  providerID,
		provisioner: provisioner,
		concurrency: concurrency,
	}
}

// Settings implements controller.QController interface.
func (ctrl *ProvisionController[T]) Settings() controller.QSettings {
	var t T

	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: resources.CloudProviderNamespace,
				Type:      cloud.MachineRequestType,
				Kind:      controller.InputQPrimary,
			},
			{
				Namespace: resources.CloudProviderNamespace,
				Type:      cloud.MachineRequestStatusType,
				Kind:      controller.InputQMappedDestroyReady,
			},
			{
				Namespace: resources.DefaultNamespace,
				Type:      siderolink.ConnectionParamsType,
				Kind:      controller.InputQMapped,
				ID:        optional.Some(siderolink.ConfigID),
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
				Type: cloud.MachineRequestStatusType,
			},
			{
				Kind: controller.OutputShared,
				Type: t.ResourceDefinition().Type,
			},
		},
		Concurrency: optional.Some[uint](ctrl.concurrency),
	}
}

// MapInput implements controller.QController interface.
func (ctrl *ProvisionController[T]) MapInput(_ context.Context, _ *zap.Logger,
	_ controller.QRuntime, ptr resource.Pointer,
) ([]resource.Pointer, error) {
	if ptr.Type() == siderolink.ConnectionParamsType {
		return nil, nil
	}

	return []resource.Pointer{
		cloud.NewMachineRequest(ptr.ID()).Metadata(),
	}, nil
}

// Reconcile implements controller.QController interface.
func (ctrl *ProvisionController[T]) Reconcile(ctx context.Context,
	logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer,
) error {
	machineRequest, err := safe.ReaderGet[*cloud.MachineRequest](ctx, r, cloud.NewMachineRequest(ptr.ID()).Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
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

	return safe.WriterModify(ctx, r, machineRequestStatus, func(res *cloud.MachineRequestStatus) error {
		return ctrl.reconcileRunning(ctx, r, logger, machineRequest, res)
	})
}

func (ctrl *ProvisionController[T]) reconcileRunning(ctx context.Context, r controller.QRuntime, logger *zap.Logger,
	machineRequest *cloud.MachineRequest, machineRequestStatus *cloud.MachineRequestStatus,
) error {
	connectionParams, err := safe.ReaderGetByID[*siderolink.ConnectionParams](ctx, r, siderolink.ConfigID)
	if err != nil {
		return err
	}

	var t T

	res, err := protobuf.CreateResource(t.ResourceDefinition().Type)
	if err != nil {
		return err
	}

	*res.Metadata() = resource.NewMetadata(infrares.ResourceNamespace(ctrl.providerID), t.ResourceDefinition().Type, machineRequest.Metadata().ID(), resource.VersionUndefined)

	if err = safe.WriterModify(ctx, r, res.(T), func(st T) error { //nolint:forcetypeassert
		var provisionResult provision.Result

		provisionResult, err = ctrl.provisioner.Provision(ctx, logger, st, machineRequest, connectionParams)
		if err != nil {
			if xerrors.TypeIs[*controller.RequeueError](err) {
				return err
			}

			machineRequestStatus.TypedSpec().Value.Error = err.Error()
			machineRequestStatus.TypedSpec().Value.Stage = cloudspecs.MachineRequestStatusSpec_FAILED

			return nil
		}

		machineRequestStatus.TypedSpec().Value.Id = provisionResult.UUID
		machineRequestStatus.TypedSpec().Value.Stage = cloudspecs.MachineRequestStatusSpec_PROVISIONED

		*machineRequestStatus.Metadata().Labels() = *machineRequest.Metadata().Labels()

		machineRequestStatus.Metadata().Labels().Set(omni.LabelMachineInfraID, provisionResult.MachineID)

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (ctrl *ProvisionController[T]) initializeStatus(ctx context.Context, r controller.QRuntime, logger *zap.Logger, machineRequest *cloud.MachineRequest) (*cloud.MachineRequestStatus, error) {
	mrs, err := safe.ReaderGetByID[*cloud.MachineRequestStatus](ctx, r, machineRequest.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return nil, err
	}

	if mrs != nil {
		return mrs, nil
	}

	return safe.WriterModifyWithResult(ctx, r, cloud.NewMachineRequestStatus(machineRequest.Metadata().ID()), func(res *cloud.MachineRequestStatus) error {
		if res.TypedSpec().Value.Stage == cloudspecs.MachineRequestStatusSpec_UNKNOWN {
			res.TypedSpec().Value.Stage = cloudspecs.MachineRequestStatusSpec_PROVISIONING
			*res.Metadata().Labels() = *machineRequest.Metadata().Labels()

			logger.Info("machine provision started", zap.String("request_id", machineRequest.Metadata().ID()))
		}

		return nil
	})
}

func (ctrl *ProvisionController[T]) reconcileTearingDown(ctx context.Context, r controller.QRuntime, logger *zap.Logger, machineRequest *cloud.MachineRequest) error {
	t, err := safe.ReaderGetByID[T](ctx, r, machineRequest.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if err = ctrl.provisioner.Deprovision(ctx, logger, t, machineRequest); err != nil {
		return err
	}

	resources := []resource.Metadata{
		resource.NewMetadata(t.ResourceDefinition().DefaultNamespace, t.ResourceDefinition().Type, machineRequest.Metadata().ID(), resource.VersionUndefined),
		*cloud.NewMachineRequestStatus(machineRequest.Metadata().ID()).Metadata(),
	}

	for _, md := range resources {
		ready, err := r.Teardown(ctx, md)
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

	logger.Info("machine deprovisioned", zap.String("request_id", machineRequest.Metadata().ID()))

	return r.RemoveFinalizer(ctx, machineRequest.Metadata(), ctrl.Name())
}
