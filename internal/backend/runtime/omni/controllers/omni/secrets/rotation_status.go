// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secrets

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/secretrotation"
)

// RotationPaused represents the status indicating that the rotation process is currently paused.
const RotationPaused = "rotation paused"

// SecretRotationStatusController uses omni.ClusterSecrets to create an omni.ClusterSecretRotationStatus and omni.ClusterMachineSecrets for each omni.ClusterMachine.
type SecretRotationStatusController struct {
	*qtransform.QController[*omni.ClusterSecrets, *omni.ClusterSecretsRotationStatus]
}

// SecretRotationStatusControllerName is the name of the SecretRotationStatusController.
var SecretRotationStatusControllerName = "SecretRotationStatusController"

// NewSecretRotationStatusController instantiates the secret rotation status controller.
func NewSecretRotationStatusController() *SecretRotationStatusController {
	ctrl := &SecretRotationStatusController{}

	ctrl.QController = qtransform.NewQController(
		qtransform.Settings[*omni.ClusterSecrets, *omni.ClusterSecretsRotationStatus]{
			Name: SecretRotationStatusControllerName,
			MapMetadataFunc: func(clusterSecrets *omni.ClusterSecrets) *omni.ClusterSecretsRotationStatus {
				return omni.NewClusterSecretsRotationStatus(clusterSecrets.Metadata().ID())
			},
			UnmapMetadataFunc: func(rotationStatus *omni.ClusterSecretsRotationStatus) *omni.ClusterSecrets {
				return omni.NewClusterSecrets(resources.DefaultNamespace, rotationStatus.Metadata().ID())
			},
			TransformExtraOutputFunc:        ctrl.reconcileRunning,
			FinalizerRemovalExtraOutputFunc: ctrl.reconcileTearingDown,
		},
		qtransform.WithExtraMappedInput[*omni.ClusterMachineStatus](
			mappers.MapByClusterLabel[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterStatus](
			qtransform.MapperSameID[*omni.ClusterSecrets](),
		),
		qtransform.WithExtraOutputs(controller.Output{
			Type: omni.ClusterMachineSecretsRotationType,
			Kind: controller.OutputExclusive,
		}),
	)

	return ctrl
}

//nolint:gocyclo,cyclop,gocognit
func (ctrl *SecretRotationStatusController) reconcileRunning(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	clusterSecrets *omni.ClusterSecrets,
	rotationStatus *omni.ClusterSecretsRotationStatus,
) error {
	clusterStatus, err := safe.ReaderGetByID[*omni.ClusterStatus](ctx, r, clusterSecrets.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	// ClusterSecretsRotationStatus is trailing behind ClusterSecrets, keep it up to date
	if clusterSecrets.Metadata().Version().String() != rotationStatus.TypedSpec().Value.ClusterSecretsVersion {
		rotationStatus.TypedSpec().Value.ClusterSecretsVersion = clusterSecrets.Metadata().Version().String()
		rotationStatus.TypedSpec().Value.Phase = clusterSecrets.TypedSpec().Value.RotationPhase
		rotationStatus.TypedSpec().Value.Component = clusterSecrets.TypedSpec().Value.ComponentInRotation
	}

	cmStatuses, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterSecrets.Metadata().ID())))
	if err != nil {
		return err
	}

	cmStatusesMap := xslices.ToMap(slices.Collect(cmStatuses.All()), func(r *omni.ClusterMachineStatus) (resource.ID, *omni.ClusterMachineStatus) {
		return r.Metadata().ID(), r
	})

	secretRotations, err := safe.ReaderListAll[*omni.ClusterMachineSecretsRotation](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterSecrets.Metadata().ID())))
	if err != nil {
		return err
	}

	secretRotationsMap := xslices.ToMap(slices.Collect(secretRotations.All()), func(r *omni.ClusterMachineSecretsRotation) (resource.ID, *omni.ClusterMachineSecretsRotation) {
		return r.Metadata().ID(), r
	})

	ongoingRotations := secretrotation.Candidates{}
	rotationsToDelete := secretrotation.Candidates{}
	rotationsToUpdate := secretrotation.Candidates{}

	for cmStatus := range cmStatuses.All() {
		err = ctrl.processMachine(ctx, r, cmStatus, clusterSecrets, rotationStatus, secretRotationsMap, &rotationsToUpdate, &rotationsToDelete, &ongoingRotations)
		if err != nil {
			return err
		}
	}

	for _, rotationToDelete := range rotationsToDelete.Candidates {
		if _, err = ctrl.handleDestroy(ctx, r, rotationToDelete.MachineID); err != nil {
			return err
		}
	}

	if clusterSecrets.TypedSpec().Value.RotationPhase == specs.ClusterSecretsRotationStatusSpec_OK {
		rotationStatus.TypedSpec().Value.Status = ""
		rotationStatus.TypedSpec().Value.Step = ""
		rotationStatus.TypedSpec().Value.Error = ""

		// The block below is to reset ClusterMachineSecretsRotation's Status to IDLE and Phase to OK after rotation is completed.
		// Normally the Status field would flip from IN_PROGRESS to IDLE when rotation is ongoing and validation succeeds.
		// However, after rotation is finalized, there is no need to do another validation. Here we reset the Status and Phase fields
		// for any ClusterMachineSecretsRotation that is not already reset.
		for _, candidate := range rotationsToUpdate.Candidates {
			secretRotation, ok := secretRotationsMap[candidate.MachineID]
			if !ok || (secretRotation != nil && secretRotation.TypedSpec().Value.Phase == clusterSecrets.TypedSpec().Value.RotationPhase) {
				continue
			}

			if err = safe.WriterModify[*omni.ClusterMachineSecretsRotation](ctx, r, omni.NewClusterMachineSecretsRotation(candidate.MachineID),
				func(res *omni.ClusterMachineSecretsRotation) error {
					res.TypedSpec().Value.Status = specs.ClusterMachineSecretsRotationSpec_IDLE
					res.TypedSpec().Value.Phase = specs.ClusterSecretsRotationStatusSpec_OK

					return nil
				}); err != nil {
				return err
			}
		}

		return nil
	}

	if _, clusterLocked := clusterStatus.Metadata().Annotations().Get(omni.ClusterLocked); clusterLocked {
		rotationStatus.TypedSpec().Value.Status = RotationPaused
		rotationStatus.TypedSpec().Value.Step = "waiting for the cluster to be unlocked"
		rotationStatus.TypedSpec().Value.Error = ""

		return nil
	}

	if !clusterStatus.TypedSpec().Value.Ready || clusterStatus.TypedSpec().Value.Phase != specs.ClusterStatusSpec_RUNNING {
		rotationStatus.TypedSpec().Value.Status = RotationPaused
		rotationStatus.TypedSpec().Value.Step = "waiting for the cluster to become ready"
		rotationStatus.TypedSpec().Value.Error = ""

		return nil
	}

	rotationsToUpdate.Sort()

	logger.Info("Rotating secret for phase",
		zap.String("cluster", clusterStatus.Metadata().ID()),
		zap.String("component", rotationStatus.TypedSpec().Value.Component.String()),
		zap.String("phase", rotationStatus.TypedSpec().Value.Phase.String()),
		zap.Int("rotations", rotationsToUpdate.Len()),
		zap.Int("ongoing_rotations", ongoingRotations.Len()),
		zap.Int("blocked", len(rotationsToUpdate.Blocked())),
		zap.Int("not_ready", len(rotationsToUpdate.NotReady())),
	)

	// There are ongoing or recently submitted rotations, requeue until they are done
	if ongoingRotations.Len() > 0 {
		logger.Info("waiting for all ongoing rotations to be completed", zap.Strings("machines", xslices.Map(ongoingRotations.Candidates,
			func(res secretrotation.Candidate) string {
				return res.MachineID
			})))

		return controller.NewRequeueInterval(time.Second)
	}

	if rotationsToUpdate.Len() > 0 {
		if err = ctrl.rotate(ctx, r, logger, clusterSecrets, rotationStatus, &rotationsToUpdate, cmStatusesMap, cmStatuses); err != nil {
			return err
		}
	}

	rotationStatus.TypedSpec().Value.Step = ""
	rotationStatus.TypedSpec().Value.Status = fmt.Sprintf("rotation phase %s is completed for secret %q",
		clusterSecrets.TypedSpec().Value.RotationPhase.String(), clusterSecrets.TypedSpec().Value.ComponentInRotation.String())

	switch clusterSecrets.TypedSpec().Value.RotationPhase {
	case specs.ClusterSecretsRotationStatusSpec_PRE_ROTATE:
		rotationStatus.TypedSpec().Value.Phase = specs.ClusterSecretsRotationStatusSpec_ROTATE
	case specs.ClusterSecretsRotationStatusSpec_ROTATE:
		rotationStatus.TypedSpec().Value.Phase = specs.ClusterSecretsRotationStatusSpec_POST_ROTATE
	case specs.ClusterSecretsRotationStatusSpec_POST_ROTATE:
		rotationStatus.TypedSpec().Value.Phase = specs.ClusterSecretsRotationStatusSpec_OK
	case specs.ClusterSecretsRotationStatusSpec_OK: // nothing to do
	}

	return nil
}

func (ctrl *SecretRotationStatusController) reconcileTearingDown(ctx context.Context, r controller.ReaderWriter, _ *zap.Logger, clusterSecrets *omni.ClusterSecrets) error {
	cmStatuses, err := safe.ReaderListAll[*omni.ClusterMachineStatus](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterSecrets.Metadata().ID())))
	if err != nil {
		return err
	}

	allDestroyed := true

	for cmStatus := range cmStatuses.All() {
		if cmStatus.Metadata().Phase() == resource.PhaseRunning {
			allDestroyed = false

			continue
		}

		var destroyed bool

		destroyed, err = ctrl.handleDestroy(ctx, r, cmStatus.Metadata().ID())
		if err != nil {
			return err
		}

		if !destroyed {
			allDestroyed = false
		}
	}

	if !allDestroyed {
		return xerrors.NewTagged[qtransform.SkipReconcileTag](fmt.Errorf("waiting for all cluster machine statuses to be deleted before destroying machine secrets rotations resources"))
	}

	return nil
}

func (ctrl *SecretRotationStatusController) processMachine(
	ctx context.Context,
	r controller.ReaderWriter,
	cmStatus *omni.ClusterMachineStatus,
	clusterSecrets *omni.ClusterSecrets,
	rotationStatus *omni.ClusterSecretsRotationStatus,
	secretRotationsMap map[resource.ID]*omni.ClusterMachineSecretsRotation,
	rotationsToUpdate *secretrotation.Candidates,
	rotationsToDelete *secretrotation.Candidates,
	ongoingRotations *secretrotation.Candidates,
) error {
	_, isControlPlane := cmStatus.Metadata().Labels().Get(omni.LabelControlPlaneRole)
	_, locked := cmStatus.Metadata().Annotations().Get(omni.MachineLocked)
	hostname, _ := cmStatus.Metadata().Labels().Get(omni.LabelHostname)

	candidate := secretrotation.Candidate{
		MachineID:    cmStatus.Metadata().ID(),
		Hostname:     hostname,
		ControlPlane: isControlPlane,
		Blocked:      locked,
		Ready:        cmStatus.TypedSpec().Value.Ready,
	}

	// ClusterMachineStatus is being deleted, delete the corresponding ClusterMachineSecretsRotation
	if cmStatus.Metadata().Phase() == resource.PhaseTearingDown {
		rotationsToDelete.Add(candidate)

		return nil
	}

	secretRotation, ok := secretRotationsMap[cmStatus.Metadata().ID()]
	// ClusterMachineSecretsRotation does not exist yet, create it
	if !ok {
		rotationsToUpdate.Add(candidate)

		return nil
	}

	// ClusterMachineSecretsRotation hasn't been updated yet to match the rotation phase coming from ClusterSecrets, update it
	if clusterSecrets.TypedSpec().Value.RotationPhase != secretRotation.TypedSpec().Value.Phase {
		rotationsToUpdate.Add(candidate)

		return nil
	}

	// ClusterMachineSecretsRotation is in progress, validate if the rotation phase is completed
	if secretRotation.TypedSpec().Value.Status == specs.ClusterMachineSecretsRotationSpec_IN_PROGRESS {
		valid, err := candidate.Validate(ctx, clusterSecrets, cmStatus)
		if err != nil {
			rotationStatus.TypedSpec().Value.Error = err.Error()
		}

		if valid {
			if err = safe.WriterModify[*omni.ClusterMachineSecretsRotation](ctx, r, omni.NewClusterMachineSecretsRotation(secretRotation.Metadata().ID()),
				func(res *omni.ClusterMachineSecretsRotation) error {
					helpers.CopyLabels(cmStatus, res, omni.LabelCluster, omni.LabelMachineSet, omni.LabelWorkerRole, omni.LabelControlPlaneRole)
					res.TypedSpec().Value.Status = specs.ClusterMachineSecretsRotationSpec_IDLE
					res.TypedSpec().Value.Phase = clusterSecrets.TypedSpec().Value.RotationPhase

					return nil
				}); err != nil {
				return err
			}

			return nil
		}

		ongoingRotations.Add(candidate)
	}

	// If we reached here, then it means the ClusterMachineSecretsRotation is up to date, nothing to do

	return nil
}

func (ctrl *SecretRotationStatusController) rotate(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	clusterSecrets *omni.ClusterSecrets,
	rotationStatus *omni.ClusterSecretsRotationStatus,
	rotationsToUpdate *secretrotation.Candidates,
	cmStatusesMap map[resource.ID]*omni.ClusterMachineStatus,
	cmStatuses safe.List[*omni.ClusterMachineStatus],
) error {
	rotationToUpdate := rotationsToUpdate.Candidates[0]

	if !rotationToUpdate.Ready {
		logger.Warn("waiting for machine to become ready", zap.String("machine", rotationToUpdate.MachineID))

		rotationStatus.TypedSpec().Value.Status = RotationPaused
		rotationStatus.TypedSpec().Value.Step = fmt.Sprintf("waiting for a machine to become ready from: %q", rotationsToUpdate.NotReady())

		return controller.NewRequeueInterval(5 * time.Second)
	}

	if rotationToUpdate.Blocked {
		logger.Warn("waiting for machine to be unlocked", zap.String("machine", rotationToUpdate.MachineID))

		rotationStatus.TypedSpec().Value.Status = RotationPaused
		rotationStatus.TypedSpec().Value.Step = fmt.Sprintf("waiting for a machine to be unlocked from: %q", rotationsToUpdate.Blocked())

		return controller.NewRequeueInterval(5 * time.Second)
	}

	cmStatus := cmStatusesMap[rotationToUpdate.MachineID]
	if err := safe.WriterModify[*omni.ClusterMachineSecretsRotation](ctx, r, omni.NewClusterMachineSecretsRotation(rotationToUpdate.MachineID),
		func(res *omni.ClusterMachineSecretsRotation) error {
			helpers.CopyLabels(cmStatus, res, omni.LabelCluster, omni.LabelMachineSet, omni.LabelWorkerRole, omni.LabelControlPlaneRole)
			res.TypedSpec().Value.ClusterSecretsVersion = clusterSecrets.Metadata().Version().String()
			res.TypedSpec().Value.Phase = clusterSecrets.TypedSpec().Value.RotationPhase
			res.TypedSpec().Value.Status = specs.ClusterMachineSecretsRotationSpec_IN_PROGRESS

			return nil
		}); err != nil {
		return err
	}

	if err := r.AddFinalizer(ctx, omni.NewClusterMachineStatus(resources.DefaultNamespace, rotationToUpdate.MachineID).Metadata(), SecretRotationStatusControllerName); err != nil {
		return err
	}

	rotationStatus.TypedSpec().Value.Error = ""
	rotationStatus.TypedSpec().Value.Status = fmt.Sprintf("rotation phase %s %d/%d",
		clusterSecrets.TypedSpec().Value.RotationPhase.String(), cmStatuses.Len()-rotationsToUpdate.Len()+1, cmStatuses.Len())
	rotationStatus.TypedSpec().Value.Step = fmt.Sprintf("rotating secret for machine: %s", rotationToUpdate.Hostname)

	// We won't be informed through a resource update. We need to requeue to check the ongoing rotation on the next wake-up.
	return controller.NewRequeueInterval(time.Second)
}

func (ctrl *SecretRotationStatusController) handleDestroy(ctx context.Context, r controller.ReaderWriter, machineID resource.ID) (bool, error) {
	destroyed, err := helpers.TeardownAndDestroy(ctx, r, omni.NewClusterMachineSecretsRotation(machineID).Metadata())
	if err != nil {
		return destroyed, err
	}

	if !destroyed {
		return destroyed, nil
	}

	if err = r.RemoveFinalizer(ctx, omni.NewClusterMachineStatus(resources.DefaultNamespace, machineID).Metadata(), SecretRotationStatusControllerName); err != nil {
		return destroyed, err
	}

	return destroyed, nil
}
