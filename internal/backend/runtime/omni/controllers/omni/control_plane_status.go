// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/pkg/check"
)

// ControlPlaneStatusController creates ControlPlaneStatus resource for controlplane MachineSets.
type ControlPlaneStatusController = qtransform.QController[*omni.MachineSet, *omni.ControlPlaneStatus]

// ControlPlaneCheckTimeout is the timeout for the controlplane checks.
const ControlPlaneCheckTimeout = 5 * time.Minute

// NewControlPlaneStatusController initializes ControlPlaneStatusController.
func NewControlPlaneStatusController() *ControlPlaneStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.MachineSet, *omni.ControlPlaneStatus]{
			Name: "ControlPlaneStatusController",
			MapMetadataFunc: func(machineSet *omni.MachineSet) *omni.ControlPlaneStatus {
				return omni.NewControlPlaneStatus(resources.DefaultNamespace, machineSet.Metadata().ID())
			},
			UnmapMetadataFunc: func(cpStatus *omni.ControlPlaneStatus) *omni.MachineSet {
				return omni.NewMachineSet(resources.DefaultNamespace, cpStatus.Metadata().ID())
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

							continue
						}

						return err
					}

					spec.SetCondition(handler.condition, specs.ControlPlaneStatusSpec_Condition_Ready, specs.ControlPlaneStatusSpec_Condition_Info, "")
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperNone[*omni.TalosConfig](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByMachineSetLabelOnlyControlplane[*omni.ClusterMachineStatus, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByMachineSetLabelOnlyControlplane[*omni.ClusterMachine, *omni.MachineSet](),
		),
		qtransform.WithExtraMappedInput(
			func(ctx context.Context, _ *zap.Logger, r controller.QRuntime, etcdAuditResult *omni.EtcdAuditResult) ([]resource.Pointer, error) {
				clusterName := etcdAuditResult.Metadata().ID()

				items, err := safe.ReaderListAll[*omni.MachineSet](ctx, r,
					state.WithLabelQuery(
						resource.LabelEqual(omni.LabelCluster, clusterName),
						resource.LabelExists(omni.LabelControlPlaneRole),
					),
				)
				if err != nil {
					return nil, err
				}

				return safe.Map(items, func(machineSet *omni.MachineSet) (resource.Pointer, error) {
					return machineSet.Metadata(), nil
				})
			},
		),
		qtransform.WithConcurrency(4),
	)
}
