// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/task"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task/clustermachine"
)

// ClusterMachineIdentityController manages ClusterMachineIdentity resource lifecycle.
type ClusterMachineIdentityController struct {
	runner *task.Runner[clustermachine.IdentityCollectorChan, clustermachine.IdentityCollectorTaskSpec]
}

// Name implements controller.Controller interface.
func (ctrl *ClusterMachineIdentityController) Name() string {
	return "ClusterMachineIdentityController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ClusterMachineIdentityController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterMachineType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.TalosConfigType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ClusterMachineIdentityController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.ClusterMachineIdentityType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocognit
func (ctrl *ClusterMachineIdentityController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	notifyCh := make(chan *omni.ClusterMachineIdentity)

	ctrl.runner = task.NewEqualRunner[clustermachine.IdentityCollectorTaskSpec]()
	defer ctrl.runner.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case clusterMachineIdentity := <-notifyCh:
			err := safe.WriterModify(ctx, r, clusterMachineIdentity, func(res *omni.ClusterMachineIdentity) error {
				// we should replace the labels
				res.Metadata().Labels().Do(func(temp kvutils.TempKV) {
					for _, key := range res.Metadata().Labels().Keys() {
						temp.Delete(key)
					}

					for key, value := range clusterMachineIdentity.Metadata().Labels().Raw() {
						temp.Set(key, value)
					}
				})

				_, isCp := clusterMachineIdentity.Metadata().Labels().Get(omni.LabelControlPlaneRole)

				spec := clusterMachineIdentity.TypedSpec().Value

				switch {
				case !isCp:
					res.TypedSpec().Value.EtcdMemberId = 0
				case spec.EtcdMemberId != 0:
					res.TypedSpec().Value.EtcdMemberId = spec.EtcdMemberId
				}

				if spec.NodeIdentity != "" {
					res.TypedSpec().Value.NodeIdentity = spec.NodeIdentity
				}

				if spec.Nodename != "" {
					res.TypedSpec().Value.Nodename = spec.Nodename
				}

				if spec.NodeIps != nil {
					res.TypedSpec().Value.NodeIps = spec.NodeIps
				}

				return nil
			})
			if err != nil {
				return err
			}
		case <-r.EventCh():
			if err := ctrl.reconcileCollectors(ctx, r, logger, notifyCh); err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *ClusterMachineIdentityController) reconcileCollectors(ctx context.Context, r controller.Runtime, logger *zap.Logger, notifyCh clustermachine.IdentityCollectorChan) error {
	list, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, r)
	if err != nil {
		return err
	}

	tracker := trackResource(r, resources.DefaultNamespace, omni.ClusterMachineIdentityType)

	expectedCollectors := map[string]clustermachine.IdentityCollectorTaskSpec{}

	for iter := list.Iterator(); iter.Next(); {
		clusterMachine := iter.Value()

		tracker.keep(clusterMachine)

		if clusterMachine.Metadata().Phase() == resource.PhaseTearingDown {
			// skip machines being torn down
			continue
		}

		id := clusterMachine.Metadata().ID()

		clusterName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelCluster)
		if !ok {
			return fmt.Errorf("failed to determine the cluster of the cluster machine %s", id)
		}

		machineSetName, ok := clusterMachine.Metadata().Labels().Get(omni.LabelMachineSet)
		if !ok {
			return fmt.Errorf("failed to determine the machine set of the cluster machine %s", id)
		}

		machine, err := safe.ReaderGet[*omni.Machine](ctx, r,
			omni.NewMachine(resources.DefaultNamespace, id).Metadata(),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !machine.TypedSpec().Value.Connected {
			// skip machines which are not connected
			continue
		}

		talosConfig, err := safe.ReaderGet[*omni.TalosConfig](ctx, r, resource.NewMetadata(
			resources.DefaultNamespace,
			omni.TalosConfigType,
			clusterName,
			resource.VersionUndefined,
		))
		if err != nil {
			return err
		}

		config := omni.NewTalosClientConfig(talosConfig)

		_, isControlPlane := clusterMachine.Metadata().Labels().Get(omni.LabelControlPlaneRole)

		expectedCollectors[id] = clustermachine.NewIdentityCollectorTaskSpec(
			id,
			config,
			talosConfig.Metadata().Version(),
			machine.TypedSpec().Value.ManagementAddress,
			isControlPlane,
			clusterName,
			machineSetName,
		)
	}

	ctrl.runner.Reconcile(ctx, logger, expectedCollectors, notifyCh)

	return tracker.cleanup(ctx)
}
