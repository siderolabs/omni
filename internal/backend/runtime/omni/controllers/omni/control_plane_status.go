// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/pkg/check"
)

// ControlPlaneStatusController creates ControlPlaneStatus resource for controlplane MachineSets.
type ControlPlaneStatusController = qtransform.QController[*omni.MachineSet, *omni.ControlPlaneStatus]

const (
	// ControlPlaneCheckTimeout is the timeout for the controlplane checks.
	ControlPlaneCheckTimeout = 5 * time.Minute
	requeueTimeout           = 30 * time.Minute
)

// NewControlPlaneStatusController initializes ControlPlaneStatusController.
func NewControlPlaneStatusController() *ControlPlaneStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineSet, *omni.ControlPlaneStatus]{
			Name: "ControlPlaneStatusController",
			MapMetadataFunc: func(machineSet *omni.MachineSet) *omni.ControlPlaneStatus {
				return omni.NewControlPlaneStatus(machineSet.Metadata().ID())
			},
			UnmapMetadataFunc: func(cpStatus *omni.ControlPlaneStatus) *omni.MachineSet {
				return omni.NewMachineSet(cpStatus.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, machineSet *omni.MachineSet, cpStatus *omni.ControlPlaneStatus) error {
				if _, isControlplane := machineSet.Metadata().Labels().Get(omni.LabelControlPlaneRole); !isControlplane {
					return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("not a controlplane machineset")
				}

				clusterName, ok := machineSet.Metadata().Labels().Get(omni.LabelCluster)
				if !ok {
					return fmt.Errorf("failed to get cluster name from the machine set %s", machineSet.Metadata().ID())
				}

				cpStatus.Metadata().Labels().Set(omni.LabelCluster, clusterName)

				spec := cpStatus.TypedSpec().Value

				// set a timeout to run the checks
				ctx, cancel := context.WithTimeout(ctx, ControlPlaneCheckTimeout)
				defer cancel()

				handlers := []struct {
					check     func(context.Context, controller.Reader, string) error
					condition specs.ConditionType
				}{
					{
						condition: specs.ConditionType_WireguardConnection,
						check:     check.Connection,
					},
					{
						condition: specs.ConditionType_Etcd,
						check:     check.Etcd,
					},
				}

				var (
					interruptReason string
					interrupted     bool
				)

				var checkErr error

				for _, handler := range handlers {
					if interrupted {
						spec.SetCondition(
							handler.condition,
							specs.ControlPlaneStatusSpec_Condition_Unknown,
							specs.ControlPlaneStatusSpec_Condition_Error,
							interruptReason,
						)

						continue
					}

					err := handler.check(ctx, r, clusterName)
					if err != nil {
						var checkFail *check.Error

						if errors.As(err, &checkFail) {
							spec.SetCondition(handler.condition, checkFail.Status, checkFail.Severity, checkFail.Error())

							interrupted = checkFail.Interrupt
							interruptReason = fmt.Sprintf("The check wasn't run because the condition check %q has failed", handler.condition.String())
							checkErr = errors.Join(checkErr, err)

							continue
						}

						return err
					}

					spec.SetCondition(handler.condition, specs.ControlPlaneStatusSpec_Condition_Ready, specs.ControlPlaneStatusSpec_Condition_Info, "")
				}

				if checkErr != nil {
					return checkErr
				}

				return controller.NewRequeueInterval(requeueTimeout)
			},
		},
		qtransform.WithExtraMappedInput[*omni.TalosConfig](
			mappers.MapClusterResourceToLabeledResources[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachineStatus](
			mappers.MapByMachineSetLabelOnlyControlplane[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.ClusterMachine](
			mappers.MapByMachineSetLabelOnlyControlplane[*omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput[*omni.EtcdAuditResult](
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, etcdAuditResult controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
				clusterName := etcdAuditResult.ID()

				items, err := safe.ReaderListAll[*omni.MachineSet](ctx, r,
					state.WithLabelQuery(
						resource.LabelEqual(omni.LabelCluster, clusterName),
						resource.LabelExists(omni.LabelControlPlaneRole),
					),
				)
				if err != nil {
					return nil, err
				}

				return slices.Collect(items.Pointers()), nil
			},
		),
		qtransform.WithConcurrency(4),
	)
}
