// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/mappers"
)

// ClusterStatusController manages ClusterStatus resource lifecycle.
//
// ClusterStatusController aggregates the cluster state based on the cluster machines states.
type ClusterStatusController = qtransform.QController[*omni.Cluster, *omni.ClusterStatus]

// NewClusterStatusController initializes ClusterStatusController.
//
//nolint:gocognit,gocyclo,cyclop
func NewClusterStatusController(embeddedDiscoveryServiceEnabled bool) *ClusterStatusController {
	return qtransform.NewQController(
		qtransform.Settings[*omni.Cluster, *omni.ClusterStatus]{
			Name: "ClusterStatusController",
			MapMetadataFunc: func(cluster *omni.Cluster) *omni.ClusterStatus {
				return omni.NewClusterStatus(resources.DefaultNamespace, cluster.Metadata().ID())
			},
			UnmapMetadataFunc: func(clusterStatus *omni.ClusterStatus) *omni.Cluster {
				return omni.NewCluster(resources.DefaultNamespace, clusterStatus.Metadata().ID())
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, _ *zap.Logger, cluster *omni.Cluster, clusterStatus *omni.ClusterStatus) error {
				shouldUseEmbeddedDiscoveryService := func() (bool, error) {
					if !embeddedDiscoveryServiceEnabled {
						return false, nil
					}

					version, err := semver.ParseTolerant(cluster.TypedSpec().Value.GetTalosVersion())
					if err != nil {
						return false, err
					}

					if version.LT(semver.MustParse("1.5.0")) {
						return false, nil
					}

					return cluster.TypedSpec().Value.GetFeatures().GetUseEmbeddedDiscoveryService(), nil
				}

				list, err := safe.ReaderListAll[*omni.MachineSetStatus](
					ctx, r,
					state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster.Metadata().ID())),
				)
				if err != nil {
					return err
				}

				lbStatus, err := safe.ReaderGetByID[*omni.LoadBalancerStatus](ctx, r, cluster.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				clusterSecrets, err := safe.ReaderGetByID[*omni.ClusterSecrets](ctx, r, cluster.Metadata().ID())
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				cpStatusReady := false

				clusterIsAvailable := false
				cpMachineSetHealthy := false
				allMachineSetsReady := true
				clusterHasConnectedControlPlanes := false

				machines := specs.Machines{}

				phases := map[specs.MachineSetPhase]int{}

				for mss := range list.All() {
					machineSetStatus := mss.TypedSpec().Value

					machines.Total += machineSetStatus.GetMachines().GetTotal()
					machines.Healthy += machineSetStatus.GetMachines().GetHealthy()
					machines.Requested += machineSetStatus.GetMachines().GetRequested()
					machines.Connected += machineSetStatus.GetMachines().GetConnected()

					_, isControlPlane := mss.Metadata().Labels().Get(omni.LabelControlPlaneRole)
					if isControlPlane {
						var cpStatus *omni.ControlPlaneStatus

						cpStatus, err = safe.ReaderGet[*omni.ControlPlaneStatus](
							ctx, r,
							resource.NewMetadata(resources.DefaultNamespace, omni.ControlPlaneStatusType, mss.Metadata().ID(), resource.VersionUndefined),
						)
						if err != nil && !state.IsNotFoundError(err) {
							return err
						}

						if cpStatus != nil && len(cpStatus.TypedSpec().Value.Conditions) > 0 {
							cpStatusReady = true

							for _, condition := range cpStatus.TypedSpec().Value.Conditions {
								cpStatusReady = cpStatusReady && condition.Status == specs.ControlPlaneStatusSpec_Condition_Ready
							}
						}

						if machineSetStatus.GetMachines().GetTotal() > 0 {
							clusterIsAvailable = true
						}

						if machineSetStatus.Phase == specs.MachineSetPhase_Running && machineSetStatus.Ready {
							cpMachineSetHealthy = true
						}

						if machineSetStatus.GetMachines().GetConnected() > 0 {
							clusterHasConnectedControlPlanes = true
						}
					}

					phases[machineSetStatus.Phase]++

					allMachineSetsReady = allMachineSetsReady && machineSetStatus.Ready
				}

				phase := specs.ClusterStatusSpec_UNKNOWN

				switch {
				case cluster.Metadata().Phase() == resource.PhaseTearingDown:
					phase = specs.ClusterStatusSpec_DESTROYING
				case len(phases) == 1 && phases[specs.MachineSetPhase_Destroying] > 0:
					// all destroying
					phase = specs.ClusterStatusSpec_DESTROYING
				case phases[specs.MachineSetPhase_ScalingUp] > 0:
					// at least one scaling up
					phase = specs.ClusterStatusSpec_SCALING_UP
				case phases[specs.MachineSetPhase_ScalingDown] > 0 || phases[specs.MachineSetPhase_Destroying] > 0:
					// at least one scaling down
					phase = specs.ClusterStatusSpec_SCALING_DOWN
				case phases[specs.MachineSetPhase_Running] > 0 || phases[specs.MachineSetPhase_Reconfiguring] > 0:
					// some running/reconfiguration
					phase = specs.ClusterStatusSpec_RUNNING
				}

				useEmbeddedDiscoveryService, err := shouldUseEmbeddedDiscoveryService()
				if err != nil {
					return fmt.Errorf("failed to determine if embedded discovery service should be used: %w", err)
				}

				clusterStatus.TypedSpec().Value = &specs.ClusterStatusSpec{
					Available:                   clusterIsAvailable,
					Ready:                       allMachineSetsReady && phase == specs.ClusterStatusSpec_RUNNING,
					KubernetesAPIReady:          lbStatus != nil && lbStatus.TypedSpec().Value.Healthy,
					ControlplaneReady:           cpStatusReady && cpMachineSetHealthy,
					Phase:                       phase,
					Machines:                    &machines,
					HasConnectedControlPlanes:   clusterHasConnectedControlPlanes,
					UseEmbeddedDiscoveryService: useEmbeddedDiscoveryService,
				}

				helpers.CopyUserLabels(clusterStatus, cluster.Metadata().Labels().Raw())

				if clusterSecrets != nil && clusterSecrets.TypedSpec().Value.Imported {
					clusterStatus.Metadata().Labels().Set(omni.LabelClusterTainted, "")
				} else {
					clusterStatus.Metadata().Labels().Delete(omni.LabelClusterTainted)
				}

				return nil
			},
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.LoadBalancerStatus, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.MachineSetStatus, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			mappers.MapByClusterLabel[*omni.ControlPlaneStatus, *omni.Cluster](),
		),
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.ClusterSecrets, *omni.Cluster](),
		),
		qtransform.WithIgnoreTeardownUntil(), // keep ClusterStatus alive until every other controller is done with Cluster
	)
}
