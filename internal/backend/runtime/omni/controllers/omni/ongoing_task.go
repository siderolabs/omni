// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// OngoingTaskController manages omni.OngoingTask.
type OngoingTaskController struct{}

// Name implements controller.Controller interface.
func (ctrl *OngoingTaskController) Name() string {
	return "OngoingTaskController"
}

// Inputs implements controller.Controller interface.
func (ctrl *OngoingTaskController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Type:      omni.ClusterDestroyStatusType,
			Namespace: resources.DefaultNamespace,
			Kind:      controller.InputWeak,
		},
		{
			Type:      omni.TalosUpgradeStatusType,
			Namespace: resources.DefaultNamespace,
			Kind:      controller.InputWeak,
		},
		{
			Type:      omni.KubernetesUpgradeStatusType,
			Namespace: resources.DefaultNamespace,
			Kind:      controller.InputWeak,
		},
		{
			Type:      omni.MachineUpgradeStatusType,
			Namespace: resources.DefaultNamespace,
			Kind:      controller.InputWeak,
		},
		{
			Type:      omni.ClusterSecretsRotationStatusType,
			Namespace: resources.DefaultNamespace,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *OngoingTaskController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.OngoingTaskType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocognit,gocyclo,cyclop
func (ctrl *OngoingTaskController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		tracker := trackResource(r, resources.EphemeralNamespace, omni.OngoingTaskType)

		destroyStatuses, err := safe.ReaderListAll[*omni.ClusterDestroyStatus](ctx, r)
		if err != nil {
			return err
		}

		talosUpgradeStatuses, err := safe.ReaderListAll[*omni.TalosUpgradeStatus](ctx, r)
		if err != nil {
			return err
		}

		kubernetesUpgradeStatuses, err := safe.ReaderListAll[*omni.KubernetesUpgradeStatus](ctx, r)
		if err != nil {
			return err
		}

		machineUpgradeStatuses, err := safe.ReaderListAll[*omni.MachineUpgradeStatus](ctx, r)
		if err != nil {
			return err
		}

		rotationStatuses, err := safe.ReaderListAll[*omni.ClusterSecretsRotationStatus](ctx, r)
		if err != nil {
			return err
		}

		for machineUpgradeStatus := range machineUpgradeStatuses.All() {
			spec := machineUpgradeStatus.TypedSpec().Value

			if !spec.IsMaintenance {
				continue
			}

			if spec.Phase != specs.MachineUpgradeStatusSpec_Upgrading {
				continue
			}

			id := fmt.Sprintf("machine-%s-maintenance-upgrade", machineUpgradeStatus.Metadata().ID())
			task := omni.NewOngoingTask(id)

			tracker.keep(task)

			if err = safe.WriterModify(ctx, r, task, func(res *omni.OngoingTask) error {
				res.TypedSpec().Value.ResourceId = machineUpgradeStatus.Metadata().ID()
				res.TypedSpec().Value.Title = "Updating Machine " + machineUpgradeStatus.Metadata().ID()
				res.TypedSpec().Value.Details = &specs.OngoingTaskSpec_MachineUpgrade{
					MachineUpgrade: spec,
				}

				return nil
			}); err != nil {
				return err
			}
		}

		if err = destroyStatuses.ForEachErr(func(s *omni.ClusterDestroyStatus) error {
			id := fmt.Sprintf("%s-destroy", s.Metadata().ID())
			task := omni.NewOngoingTask(id)

			tracker.keep(task)

			return safe.WriterModify(ctx, r, task, func(res *omni.OngoingTask) error {
				res.TypedSpec().Value.ResourceId = s.Metadata().ID()
				res.TypedSpec().Value.Title = "Destroying Cluster " + s.Metadata().ID()
				res.TypedSpec().Value.Details = &specs.OngoingTaskSpec_Destroy{
					Destroy: s.TypedSpec().Value,
				}

				return nil
			})
		}); err != nil {
			return err
		}

		if err = talosUpgradeStatuses.ForEachErr(func(s *omni.TalosUpgradeStatus) error {
			if s.TypedSpec().Value.Status == "" {
				return nil
			}

			id := fmt.Sprintf("%s-talos-update", s.Metadata().ID())
			task := omni.NewOngoingTask(id)

			tracker.keep(task)

			return safe.WriterModify(ctx, r, task, func(res *omni.OngoingTask) error {
				res.TypedSpec().Value.ResourceId = s.Metadata().ID()
				res.TypedSpec().Value.Title = "Updating Cluster " + s.Metadata().ID()
				res.TypedSpec().Value.Details = &specs.OngoingTaskSpec_TalosUpgrade{
					TalosUpgrade: s.TypedSpec().Value,
				}

				return nil
			})
		}); err != nil {
			return err
		}

		if err = kubernetesUpgradeStatuses.ForEachErr(func(s *omni.KubernetesUpgradeStatus) error {
			if s.TypedSpec().Value.Step == "" {
				return nil
			}

			id := fmt.Sprintf("%s-kubernetes-update", s.Metadata().ID())
			task := omni.NewOngoingTask(id)

			tracker.keep(task)

			return safe.WriterModify(ctx, r, task, func(res *omni.OngoingTask) error {
				res.TypedSpec().Value.ResourceId = s.Metadata().ID()
				res.TypedSpec().Value.Title = "Updating Cluster " + s.Metadata().ID()
				res.TypedSpec().Value.Details = &specs.OngoingTaskSpec_KubernetesUpgrade{
					KubernetesUpgrade: s.TypedSpec().Value,
				}

				return nil
			})
		}); err != nil {
			return err
		}

		if err = rotationStatuses.ForEachErr(func(s *omni.ClusterSecretsRotationStatus) error {
			if s.TypedSpec().Value.Status == "" {
				return nil
			}

			id := fmt.Sprintf("%s-secret-rotation", s.Metadata().ID())
			task := omni.NewOngoingTask(id)

			tracker.keep(task)

			return safe.WriterModify(ctx, r, task, func(res *omni.OngoingTask) error {
				res.TypedSpec().Value.ResourceId = s.Metadata().ID()
				res.TypedSpec().Value.Title = "Rotating Secret for Cluster " + s.Metadata().ID()
				res.TypedSpec().Value.Details = &specs.OngoingTaskSpec_SecretsRotation{
					SecretsRotation: s.TypedSpec().Value,
				}

				return nil
			})
		}); err != nil {
			return err
		}

		if err = tracker.cleanup(ctx); err != nil {
			return err
		}
	}
}
