// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	talossecrets "github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	talosrole "github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// BeforeClusterCreateFunc is a function that is called before a cluster is created.
type BeforeClusterCreateFunc func(ctx context.Context, t *testing.T, cli *client.Client, machineIDs []resource.ID)

// ClusterOptions are the options for cluster creation.
//
//nolint:govet
type ClusterOptions struct {
	Name string

	// RestoreFromEtcdBackupClusterID is the cluster ID of the cluster to restore from.
	// When specified, the cluster will be created with the etcd from the latest etcd backup of the specified cluster.
	RestoreFromEtcdBackupClusterID string

	ControlPlanes, Workers int
	Features               *specs.ClusterSpec_Features
	EtcdBackup             *specs.EtcdBackupConf

	MachineOptions MachineOptions

	BeforeClusterCreateFunc BeforeClusterCreateFunc

	InfraProvider string
	ProviderData  string

	ScalingTimeout time.Duration

	SkipExtensionCheckOnCreate bool

	AllowSchedulingOnControlPlanes bool
}

// CreateCluster verifies cluster creation.
func CreateCluster(testCtx context.Context, cli *client.Client, options ClusterOptions) TestFunc {
	return func(t *testing.T) {
		if options.ScalingTimeout == 0 {
			options.ScalingTimeout = time.Second * 150
		}

		ctx, cancel := context.WithTimeout(testCtx, options.ScalingTimeout)
		defer cancel()

		st := cli.Omni().State()
		require := require.New(t)

		pickUnallocatedMachines(ctx, t, st, options.ControlPlanes+options.Workers, func(machineIDs []resource.ID) {
			if !options.SkipExtensionCheckOnCreate {
				checkExtensionsWithRetries(ctx, t, cli, []string{HelloWorldServiceExtensionName}, machineIDs)
			}

			if options.BeforeClusterCreateFunc != nil {
				options.BeforeClusterCreateFunc(ctx, t, cli, machineIDs)
			}

			cluster := omni.NewCluster(resources.DefaultNamespace, options.Name)
			cluster.TypedSpec().Value.TalosVersion = options.MachineOptions.TalosVersion
			cluster.TypedSpec().Value.KubernetesVersion = options.MachineOptions.KubernetesVersion
			cluster.TypedSpec().Value.Features = options.Features
			cluster.TypedSpec().Value.BackupConfiguration = options.EtcdBackup

			require.NoError(st.Create(ctx, cluster))

			if options.AllowSchedulingOnControlPlanes {
				ensureAllowSchedulingOnControlPlanesConfigPatch(ctx, t, st, options.Name)
			}

			for i := range options.ControlPlanes {
				t.Logf("Adding machine '%s' to control plane (cluster %q)", machineIDs[i], options.Name)
				bindMachine(ctx, t, st, bindMachineOptions{
					clusterName:                    options.Name,
					role:                           omni.LabelControlPlaneRole,
					machineID:                      machineIDs[i],
					restoreFromEtcdBackupClusterID: options.RestoreFromEtcdBackupClusterID,
				})
			}

			for i := options.ControlPlanes; i < options.ControlPlanes+options.Workers; i++ {
				t.Logf("Adding machine '%s' to workers (cluster %q)", machineIDs[i], options.Name)
				bindMachine(ctx, t, st, bindMachineOptions{
					clusterName: options.Name,
					role:        omni.LabelWorkerRole,
					machineID:   machineIDs[i],
				})
			}

			// assert that machines got allocated (label available is removed)
			rtestutils.AssertResources(ctx, t, st, machineIDs, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
				assert.True(machineStatus.Metadata().Labels().Matches(
					resource.LabelTerm{
						Key:    omni.MachineStatusLabelAvailable,
						Op:     resource.LabelOpExists,
						Invert: true,
					},
				), resourceDetails(machineStatus))
			})
		})
	}
}

// CreateClusterWithMachineClass verifies cluster creation.
func CreateClusterWithMachineClass(testCtx context.Context, st state.State, options ClusterOptions) TestFunc {
	return func(t *testing.T) {
		if options.ScalingTimeout == 0 {
			options.ScalingTimeout = time.Second * 150
		}

		ctx, cancel := context.WithTimeout(testCtx, options.ScalingTimeout)
		defer cancel()

		require := require.New(t)

		cluster := omni.NewCluster(resources.DefaultNamespace, options.Name)
		cluster.TypedSpec().Value.TalosVersion = options.MachineOptions.TalosVersion
		cluster.TypedSpec().Value.KubernetesVersion = options.MachineOptions.KubernetesVersion
		cluster.TypedSpec().Value.Features = options.Features
		cluster.TypedSpec().Value.BackupConfiguration = options.EtcdBackup

		kubespanEnabler := omni.NewConfigPatch(resources.DefaultNamespace, fmt.Sprintf("%s-kubespan-enabler", options.Name))
		kubespanEnabler.Metadata().Labels().Set(omni.LabelCluster, options.Name)

		err := kubespanEnabler.TypedSpec().Value.SetUncompressedData([]byte(`machine:
  network:
    kubespan:
      enabled: true
`))
		require.NoError(err)

		require.NoError(st.Create(ctx, cluster))
		require.NoError(st.Create(ctx, kubespanEnabler))

		if options.AllowSchedulingOnControlPlanes {
			ensureAllowSchedulingOnControlPlanesConfigPatch(ctx, t, st, options.Name)
		}

		machineClass := omni.NewMachineClass(resources.DefaultNamespace, options.Name)

		if options.InfraProvider != "" {
			createOrUpdate(ctx, t, st, machineClass, func(r *omni.MachineClass) error {
				r.TypedSpec().Value.MatchLabels = nil
				r.TypedSpec().Value.AutoProvision = &specs.MachineClassSpec_Provision{
					ProviderId:   options.InfraProvider,
					ProviderData: options.ProviderData,
					Extensions:   []string{HelloWorldServiceExtensionName},
				}

				return nil
			})
		} else {
			createOrUpdate(ctx, t, st, machineClass, func(r *omni.MachineClass) error {
				r.TypedSpec().Value.MatchLabels = []string{omni.MachineStatusLabelConnected}
				r.TypedSpec().Value.AutoProvision = nil

				return nil
			})
		}

		updateMachineClassMachineSets(ctx, t, st, options, machineClass)
	}
}

func ensureAllowSchedulingOnControlPlanesConfigPatch(ctx context.Context, t *testing.T, st state.State, clusterID resource.ID) {
	createOrUpdate(ctx, t, st, omni.NewConfigPatch(resources.DefaultNamespace, fmt.Sprintf("400-%s-control-planes-untaint", clusterID)), func(patch *omni.ConfigPatch) error {
		patch.Metadata().Labels().Set(omni.LabelCluster, clusterID)
		patch.Metadata().Labels().Set(omni.LabelMachineSet, omni.ControlPlanesResourceID(clusterID))

		return patch.TypedSpec().Value.SetUncompressedData([]byte(`cluster:
  allowSchedulingOnControlPlanes: true
`))
	})
}

// ScaleClusterMachineSets scales the cluster with machine sets which are using machine classes.
func ScaleClusterMachineSets(testCtx context.Context, st state.State, options ClusterOptions) TestFunc {
	return func(t *testing.T) {
		if options.ScalingTimeout == 0 {
			options.ScalingTimeout = time.Second * 30
		}

		ctx, cancel := context.WithTimeout(testCtx, options.ScalingTimeout)
		defer cancel()

		updateMachineClassMachineSets(ctx, t, st, options, nil)
	}
}

// ScaleClusterUp scales up the cluster.
func ScaleClusterUp(testCtx context.Context, st state.State, options ClusterOptions) TestFunc {
	return func(t *testing.T) {
		if options.ScalingTimeout == 0 {
			options.ScalingTimeout = time.Second * 30
		}

		ctx, cancel := context.WithTimeout(testCtx, options.ScalingTimeout)
		defer cancel()

		pickUnallocatedMachines(ctx, t, st, options.ControlPlanes+options.Workers, func(machineIDs []resource.ID) {
			for i := range options.ControlPlanes {
				t.Logf("Adding machine '%s' to control plane (cluster %q)", machineIDs[i], options.Name)

				bindMachine(ctx, t, st, bindMachineOptions{
					clusterName: options.Name,
					role:        omni.LabelControlPlaneRole,
					machineID:   machineIDs[i],
				})
			}

			for i := options.ControlPlanes; i < options.ControlPlanes+options.Workers; i++ {
				t.Logf("Adding machine '%s' to workers (cluster %q)", machineIDs[i], options.Name)

				bindMachine(ctx, t, st, bindMachineOptions{
					clusterName: options.Name,
					role:        omni.LabelWorkerRole,
					machineID:   machineIDs[i],
				})
			}

			// assert that machines got allocated (label available is removed)
			rtestutils.AssertResources(ctx, t, st, machineIDs, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
				assert.True(machineStatus.Metadata().Labels().Matches(
					resource.LabelTerm{
						Key:    omni.MachineStatusLabelAvailable,
						Op:     resource.LabelOpExists,
						Invert: true,
					},
				), resourceDetails(machineStatus))
			})

			// assert that ClusterMachines got created
			rtestutils.AssertResources(ctx, t, st, machineIDs, func(*omni.ClusterMachine, *assert.Assertions) {})
		})
	}
}

// ScaleClusterDown scales cluster down.
//
// Pass < 0 to scale down machine set.
// Pass 0 to leave machine set as is.
func ScaleClusterDown(testCtx context.Context, st state.State, options ClusterOptions) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
		defer cancel()

		controlPlanes := rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, options.Name), resource.LabelExists(omni.LabelControlPlaneRole)))

		workers := rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, options.Name), resource.LabelExists(omni.LabelWorkerRole)))

		if options.ControlPlanes < 0 {
			finalCount := len(controlPlanes) + options.ControlPlanes

			require.Greaterf(t, finalCount, 0, "can't scale down")
			controlPlanes = controlPlanes[finalCount:]

			t.Logf("Removing machines '%s' from control planes (cluster %q)", controlPlanes, options.Name)

			rtestutils.Destroy[*omni.MachineSetNode](ctx, t, st, controlPlanes)
		}

		if options.Workers < 0 {
			finalCount := len(workers) + options.Workers

			require.GreaterOrEqualf(t, finalCount, 0, "can't scale down")
			workers = workers[finalCount:]

			t.Logf("Removing machines '%s' from workers (cluster %q)", workers, options.Name)

			rtestutils.Destroy[*omni.MachineSetNode](ctx, t, st, workers)
		}
	}
}

// ReplaceControlPlanes replaces controlplane nodes.
func ReplaceControlPlanes(testCtx context.Context, st state.State, options ClusterOptions) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 30*time.Second)
		defer cancel()

		existingControlPlanes := rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st,
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, options.Name), resource.LabelExists(omni.LabelControlPlaneRole)),
		)

		pickUnallocatedMachines(ctx, t, st, len(existingControlPlanes), func(machineIDs []resource.ID) {
			for _, machineID := range machineIDs {
				t.Logf("Adding machine '%s' to control plane (cluster %q)", machineID, options.Name)

				bindMachine(ctx, t, st, bindMachineOptions{
					clusterName: options.Name,
					role:        omni.LabelControlPlaneRole,
					machineID:   machineID,
				})
			}

			t.Logf("Removing machines '%s' from control planes (cluster %q)", existingControlPlanes, options.Name)

			rtestutils.Destroy[*omni.MachineSetNode](ctx, t, st, existingControlPlanes)

			// assert that machines got allocated (label available is removed)
			rtestutils.AssertResources(ctx, t, st, machineIDs, func(machineStatus *omni.MachineStatus, assert *assert.Assertions) {
				assert.True(machineStatus.Metadata().Labels().Matches(
					resource.LabelTerm{
						Key:    omni.MachineStatusLabelAvailable,
						Op:     resource.LabelOpExists,
						Invert: true,
					},
				), resourceDetails(machineStatus))
			})

			// assert that ClusterMachines got created
			rtestutils.AssertResources(ctx, t, st, machineIDs, func(*omni.ClusterMachine, *assert.Assertions) {})
		})
	}
}

// AssertClusterMachinesStage verifies that cluster machines reach a specified phase.
func AssertClusterMachinesStage(testCtx context.Context, st state.State, clusterName string, stage specs.ClusterMachineStatusSpec_Stage) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 6*time.Minute)
		defer cancel()

		require := require.New(t)

		machineIDs := getMachineSetNodes(ctx, t, st, clusterName)
		require.NotEmpty(machineIDs, "no machine set nodes found for cluster %q", clusterName)

		// assert that all machinesetnodes are present as cluster machines
		rtestutils.AssertResources(ctx, t, st, machineIDs, func(*omni.ClusterMachine, *assert.Assertions) {})

		// assert that there are not clustermachines which are not machinesetnodes
		clusterMachines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		require.NoError(err)

		machineIDMap := xslices.ToSet(machineIDs)

		clusterMachines.ForEach(func(r *omni.ClusterMachine) {
			cmID := r.Metadata().ID()

			if _, ok := machineIDMap[cmID]; ok && r.Metadata().Phase() == resource.PhaseRunning {
				// cluster machine matches expected machine set node
				return
			}

			// wait for the cluster machine to be cleaned up
			rtestutils.AssertNoResource[*omni.ClusterMachine](ctx, t, st, cmID)
		})

		// retry with the poller as the set of the machine set nodes can be changed during the lifecycle
		err = retry.Constant(time.Minute*6, retry.WithUnits(time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
			machineIDs := getMachineSetNodes(ctx, t, st, clusterName)

			for _, machine := range machineIDs {
				var status *omni.ClusterMachineStatus

				status, err = safe.ReaderGetByID[*omni.ClusterMachineStatus](ctx, st, machine)
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if status == nil {
					return retry.ExpectedErrorf("machine %q status doesn't exist yet", machine)
				}

				spec := status.TypedSpec().Value

				if spec.Stage != stage {
					return retry.ExpectedErrorf("%s != %s, %s", stage.String(), spec.Stage.String(), resourceDetails(status))
				}
			}

			return nil
		})
		require.NoError(err)
	}
}

// AssertClusterMachinesReady verifies that cluster machines reach ready state.
func AssertClusterMachinesReady(testCtx context.Context, st state.State, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 4*time.Minute)
		defer cancel()

		require := require.New(t)

		machineIDs := getMachineSetNodes(ctx, t, st, clusterName)
		require.NotEmpty(machineIDs)

		rtestutils.AssertResources(ctx, t, st, machineIDs, func(*omni.ClusterMachine, *assert.Assertions) {})

		rtestutils.AssertResources(ctx, t, st, machineIDs, func(status *omni.ClusterMachineStatus, assert *assert.Assertions) {
			spec := status.TypedSpec().Value

			assert.Truef(spec.Ready, "cluster machine status not ready: %s", resourceDetails(status))
		})

		rtestutils.AssertResources(ctx, t, st, machineIDs, func(status *omni.ClusterMachineIdentity, assert *assert.Assertions) {
			spec := status.TypedSpec().Value

			assert.NotEmptyf(spec.NodeIdentity, "no node identity: %s", resourceDetails(status))
		})
	}
}

// AssertClusterStatusReady verifies that cluster status reaches ready state.
func AssertClusterStatusReady(testCtx context.Context, st state.State, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, time.Minute*5)
		defer cancel()

		require := require.New(t)

		rtestutils.AssertResources(ctx, t, st, []string{clusterName}, func(status *omni.ClusterStatus, assert *assert.Assertions) {
			spec := status.TypedSpec().Value

			machineIDs := getMachineSetNodes(ctx, t, st, clusterName)
			require.NotEmpty(machineIDs)

			assert.Truef(spec.Available, "not available: %s", resourceDetails(status))
			assert.Equalf(specs.ClusterStatusSpec_RUNNING, spec.Phase, "cluster is not in phase running: %s", resourceDetails(status))
			assert.Equalf(spec.GetMachines().Total, spec.GetMachines().Healthy, "not all machines are healthy: %s", resourceDetails(status))
			assert.Truef(spec.Ready, "cluster is not ready: %s", resourceDetails(status))
			assert.Truef(spec.ControlplaneReady, "cluster controlplane is not ready: %s", resourceDetails(status))
			assert.Truef(spec.KubernetesAPIReady, "cluster kubernetes API is not ready: %s", resourceDetails(status))
			assert.EqualValuesf(len(machineIDs), spec.GetMachines().Total, "total machines is not the same as in the machine sets: %s", resourceDetails(status))
		})
	}
}

// AssertClusterLoadBalancerReady verifies that cluster load balancer reaches ready state.
func AssertClusterLoadBalancerReady(testCtx context.Context, st state.State, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 30*time.Second)
		defer cancel()

		rtestutils.AssertResources(ctx, t, st, []string{clusterName}, func(status *omni.LoadBalancerStatus, assert *assert.Assertions) {
			spec := status.TypedSpec().Value

			assert.Truef(spec.Healthy, "lb not healthy: %s", resourceDetails(status))
		})
	}
}

// AssertClusterKubernetesVersion verifies that Kubernetes version matches expectations.
func AssertClusterKubernetesVersion(testCtx context.Context, st state.State, clusterName, expectedKubernetesVersion string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 180*time.Second)
		defer cancel()

		rtestutils.AssertResources(ctx, t, st, []string{clusterName}, func(status *omni.KubernetesUpgradeStatus, assert *assert.Assertions) {
			spec := status.TypedSpec().Value

			assert.Equal(expectedKubernetesVersion, spec.LastUpgradeVersion, resourceDetails(status))
			assert.Equal(specs.KubernetesUpgradeStatusSpec_Done, spec.Phase, resourceDetails(status))
		})
	}
}

// AssertClusterBootstrapManifestStatus verifies that Kubernetes boostrap manifests are in sync.
func AssertClusterBootstrapManifestStatus(testCtx context.Context, st state.State, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 60*time.Second)
		defer cancel()

		rtestutils.AssertResources(ctx, t, st, []string{clusterName}, func(status *omni.KubernetesUpgradeManifestStatus, assert *assert.Assertions) {
			spec := status.TypedSpec().Value

			assert.EqualValues(0, spec.OutOfSync, resourceDetails(status))
		})
	}
}

// AssertClusterKubernetesUsage verifies that Kubernetes usage matches expectations.
func AssertClusterKubernetesUsage(testCtx context.Context, st state.State, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 180*time.Second)
		defer cancel()

		rtestutils.AssertResource(ctx, t, st, clusterName, func(status *virtual.KubernetesUsage, assert *assert.Assertions) {
			spec := status.TypedSpec().Value

			assert.NotNil(spec.Cpu, resourceDetails(status))
			assert.NotNil(spec.Mem, resourceDetails(status))
			assert.NotNil(spec.Storage, resourceDetails(status))
			assert.NotNil(spec.Pods, resourceDetails(status))

			assert.Greater(spec.Cpu.Requests, float64(0), resourceDetails(status))
			assert.Greater(spec.Cpu.Capacity, float64(0), resourceDetails(status))

			assert.Greater(spec.Mem.Requests, float64(0), resourceDetails(status))
			assert.Greater(spec.Mem.Capacity, float64(0), resourceDetails(status))

			assert.Greater(spec.Storage.Capacity, float64(0), resourceDetails(status))

			assert.Greater(spec.Pods.Count, int32(0), resourceDetails(status))
			assert.Greater(spec.Pods.Capacity, int32(0), resourceDetails(status))
		}, rtestutils.WithNamespace(resources.VirtualNamespace))
	}
}

func saveAllSupportBundles(t *testing.T, cli *client.Client, dir string) {
	ctx := t.Context()
	omniState := cli.Omni().State()

	list, err := safe.StateListAll[*omni.Cluster](ctx, omniState)
	require.NoError(t, err, "failed to list all clusters")

	for cluster := range list.All() {
		t.Logf("save support bundle for cluster %q", cluster.Metadata().ID())

		var savePath string

		if savePath, err = saveSupportBundle(ctx, cli, dir, cluster.Metadata().ID()); err != nil {
			t.Logf("failed to save support bundle for cluster %q: %v", cluster.Metadata().ID(), err)

			continue
		}

		t.Logf("saved support bundle for cluster %q to %q", cluster.Metadata().ID(), savePath)
	}
}

func saveSupportBundle(ctx context.Context, cli *client.Client, dir, cluster string) (string, error) {
	supportBundle, err := cli.Management().GetSupportBundle(ctx, cluster, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get support bundle before destruction for cluster %q: %w", cluster, err)
	}

	if err = os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory %q: %w", dir, err)
	}

	fileName := time.Now().Format("2006-01-02_15-04-05") + "-support-bundle-" + cluster + ".zip"
	bundlePath := filepath.Join(dir, fileName)

	if err = os.WriteFile(bundlePath, supportBundle, 0o644); err != nil {
		return "", fmt.Errorf("failed to write support bundle file %q: %w", cluster, err)
	}

	return bundlePath, nil
}

// AssertDestroyCluster destroys a cluster and verifies that all dependent resources are gone.
func AssertDestroyCluster(testCtx context.Context, omniState state.State, clusterName string, expectMachinesRemoved, assertInfraMachinesState bool) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 300*time.Second)
		defer cancel()

		patches := rtestutils.ResourceIDs[*omni.ConfigPatch](ctx, t, omniState, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelCluster, clusterName),
		))

		extensionsConfigurations := rtestutils.ResourceIDs[*omni.ExtensionsConfiguration](ctx, t, omniState, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelCluster, clusterName),
		))

		extraKernelArgsConfigurations := rtestutils.ResourceIDs[*omni.ExtraKernelArgsConfiguration](ctx, t, omniState, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelCluster, clusterName),
		))

		machineSets := rtestutils.ResourceIDs[*omni.MachineSet](ctx, t, omniState, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelCluster, clusterName),
		))

		clusterMachineIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, omniState, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelCluster, clusterName),
		))

		t.Log("destroying cluster", clusterName)

		_, err := omniState.Teardown(ctx, resource.NewMetadata(resources.DefaultNamespace, omni.ClusterType, clusterName, resource.VersionUndefined))

		require.NoError(t, err)

		rtestutils.AssertNoResource[*omni.Cluster](ctx, t, omniState, clusterName)
		rtestutils.AssertNoResource[*omni.ImportedClusterSecrets](ctx, t, omniState, clusterName)

		for _, id := range patches {
			rtestutils.AssertNoResource[*omni.ConfigPatch](ctx, t, omniState, id)
		}

		for _, id := range extensionsConfigurations {
			rtestutils.AssertNoResource[*omni.ExtensionsConfiguration](ctx, t, omniState, id)
		}

		for _, id := range extraKernelArgsConfigurations {
			rtestutils.AssertNoResource[*omni.ExtraKernelArgsConfiguration](ctx, t, omniState, id)
		}

		for _, id := range machineSets {
			rtestutils.AssertNoResource[*omni.MachineSet](ctx, t, omniState, id)
		}

		if expectMachinesRemoved {
			for _, id := range clusterMachineIDs {
				rtestutils.AssertNoResource[*omni.MachineStatus](ctx, t, omniState, id)
			}

			return
		}

		// wait for all machines to be returned to the pool as 'available' or be part of a different cluster
		rtestutils.AssertResources(ctx, t, omniState, clusterMachineIDs, func(machine *omni.MachineStatus, asrt *assert.Assertions) {
			_, isAvailable := machine.Metadata().Labels().Get(omni.MachineStatusLabelAvailable)
			machineCluster, machineBound := machine.Metadata().Labels().Get(omni.LabelCluster)
			asrt.True(isAvailable || (machineBound && machineCluster != clusterName),
				"machine %q: available %v, bound %v, cluster %q", machine.Metadata().ID(), isAvailable, machineBound, machineCluster,
			)
		})

		if assertInfraMachinesState {
			rtestutils.AssertResources(ctx, t, omniState, clusterMachineIDs, func(res *infra.Machine, assertion *assert.Assertions) {
				assertion.Empty(res.TypedSpec().Value.ClusterTalosVersion) // unallocated
				assertion.Empty(res.TypedSpec().Value.Extensions)

				if assertion.NotEmpty(res.TypedSpec().Value.WipeId) { // the machine should be marked for wipe
					t.Logf("machine %q is marked for wipe: %s", res.Metadata().ID(), res.TypedSpec().Value.WipeId)
				}
			})

			// the provider will wipe the machine and sets the Installed field to false
			// after the machine is wiped, ReadyToUse field will be set to true
			rtestutils.AssertResources(ctx, t, omniState, clusterMachineIDs, func(res *infra.MachineStatus, assertion *assert.Assertions) {
				assertion.False(res.TypedSpec().Value.Installed)
				assertion.True(res.TypedSpec().Value.ReadyToUse)
			})
		}
	}
}

// AssertBreakAndDestroyControlPlane breaks the control plane of the given cluster
// by freezing all control plane machines, then destroys its control plane.
func AssertBreakAndDestroyControlPlane(testCtx context.Context, st state.State, clusterName string, options Options) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 300*time.Second)
		defer cancel()

		rtestutils.AssertResource[*omni.ClusterBootstrapStatus](ctx, t, st, clusterName, func(status *omni.ClusterBootstrapStatus, assert *assert.Assertions) {
			assert.True(status.TypedSpec().Value.GetBootstrapped())
		})

		// break the control plane
		frozenMachineIDs := freezeMachinesOfType(ctx, t, st, clusterName, options.FreezeAMachineFunc, omni.LabelControlPlaneRole)

		// remove the broken machines
		rtestutils.Teardown[*siderolink.Link](ctx, t, st, frozenMachineIDs)

		// destroy the control plane
		rtestutils.Destroy[*omni.MachineSet](ctx, t, st, []string{omni.ControlPlanesResourceID(clusterName)})
		rtestutils.Destroy[*siderolink.Link](ctx, t, st, frozenMachineIDs)

		// assert that the bootstrapped flag is set to false
		rtestutils.AssertResource[*omni.ClusterBootstrapStatus](ctx, t, st, clusterName, func(status *omni.ClusterBootstrapStatus, assert *assert.Assertions) {
			assert.False(status.TypedSpec().Value.GetBootstrapped())
		})

		// wipe the frozen machines to bring them back to the pool
		for _, machineID := range frozenMachineIDs {
			wipeMachine(ctx, t, st, machineID, options.WipeAMachineFunc)
		}
	}
}

const nodeLabel = "omni-uuid"

type bindMachineOptions struct {
	clusterName, role, machineID, restoreFromEtcdBackupClusterID string
}

func bindMachine(ctx context.Context, t *testing.T, st state.State, bindOpts bindMachineOptions) {
	configPatch := omni.NewConfigPatch(
		resources.DefaultNamespace,
		fmt.Sprintf("000-%s-%s-install-disk", bindOpts.clusterName, bindOpts.machineID),
		pair.MakePair(omni.LabelCluster, bindOpts.clusterName),
		pair.MakePair(omni.LabelClusterMachine, bindOpts.machineID),
	)

	createOrUpdate(ctx, t, st, configPatch, func(cps *omni.ConfigPatch) error {
		cps.Metadata().Labels().Set(omni.LabelCluster, bindOpts.clusterName)
		cps.Metadata().Labels().Set(omni.LabelClusterMachine, bindOpts.machineID)

		var shortRole string

		switch bindOpts.role {
		case omni.LabelControlPlaneRole:
			shortRole = "cp"
		case omni.LabelWorkerRole:
			shortRole = "w"
		}

		hostname := fmt.Sprintf("%s-%s-%s", bindOpts.clusterName, shortRole, bindOpts.machineID)

		if len(hostname) > 63 {
			// trim left, to keep the UUID intact
			hostname = hostname[len(hostname)-63:]
		}

		patch := map[string]any{
			"machine": map[string]any{
				"install": map[string]any{
					"disk": "/dev/vda",
				},
				"network": map[string]any{
					"hostname": hostname,
				},
				"kubelet": map[string]any{
					"extraArgs": map[string]any{
						"node-labels": fmt.Sprintf("%s=%s", nodeLabel, bindOpts.machineID),
					},
				},
			},
		}

		patchBytes, err := yaml.Marshal(patch)
		if err != nil {
			return err
		}

		return cps.TypedSpec().Value.SetUncompressedData(patchBytes)
	})

	id := omni.WorkersResourceID(bindOpts.clusterName)
	if bindOpts.role == omni.LabelControlPlaneRole {
		id = omni.ControlPlanesResourceID(bindOpts.clusterName)
	}

	ms := omni.NewMachineSet(resources.DefaultNamespace, id)
	ms.Metadata().Labels().Set(omni.LabelCluster, bindOpts.clusterName)
	ms.Metadata().Labels().Set(bindOpts.role, "")

	var bootstrapSpec *specs.MachineSetSpec_BootstrapSpec

	if bindOpts.restoreFromEtcdBackupClusterID != "" && bindOpts.role == omni.LabelControlPlaneRole { // not a fresh cluster - restore from the etcd backup of another cluster
		backupList, err := safe.StateListAll[*omni.EtcdBackup](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, bindOpts.restoreFromEtcdBackupClusterID)))
		require.NoError(t, err)

		require.NotEmpty(t, backupList.Len(), "no etcd backup found for cluster %q", bindOpts.restoreFromEtcdBackupClusterID)

		clusterUUID, err := safe.StateGetByID[*omni.ClusterUUID](ctx, st, bindOpts.restoreFromEtcdBackupClusterID)
		require.NoError(t, err)

		backup := backupList.Get(0)

		bootstrapSpec = &specs.MachineSetSpec_BootstrapSpec{
			ClusterUuid: clusterUUID.TypedSpec().Value.GetUuid(),
			Snapshot:    backup.TypedSpec().Value.GetSnapshot(),
		}
	}

	createOrUpdate(ctx, t, st, ms, func(ms *omni.MachineSet) error {
		ms.Metadata().Labels().Set(omni.LabelCluster, bindOpts.clusterName)
		ms.Metadata().Labels().Set(bindOpts.role, "")

		ms.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling

		if bootstrapSpec != nil {
			ms.TypedSpec().Value.BootstrapSpec = bootstrapSpec
		}

		return nil
	})

	machineSetNode := omni.NewMachineSetNode(resources.DefaultNamespace, bindOpts.machineID, ms)

	_, ok := machineSetNode.Metadata().Labels().Get(omni.LabelCluster)
	require.Truef(t, ok, "the machine label cluster is not set on the machine set node")

	createOrUpdate(ctx, t, st, machineSetNode, func(*omni.MachineSetNode) error {
		return nil
	})
}

func getMachineSetNodes(ctx context.Context, t *testing.T, st state.State, clusterName string) []string {
	require := require.New(t)

	machineIDs := rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
	require.NotEmpty(machineIDs)

	return machineIDs
}

// machineAllocationLock makes sure that only one test allocates machines at a time.
var machineAllocationLock sync.Mutex

func pickUnallocatedMachines(ctx context.Context, t *testing.T, st state.State, count int, f func([]resource.ID)) {
	machineAllocationLock.Lock()
	defer machineAllocationLock.Unlock()

	result := make([]resource.ID, 0, count)

	err := retry.Constant(time.Minute).RetryWithContext(ctx, func(ctx context.Context) error {
		machineIDs := rtestutils.ResourceIDs[*omni.MachineStatus](ctx, t, st, state.WithLabelQuery(resource.LabelExists(omni.MachineStatusLabelAvailable)))

		if len(machineIDs) < count {
			return retry.ExpectedErrorf("not enough machines: available %d, requested %d", len(machineIDs), count)
		}

		for _, j := range rand.Perm(len(machineIDs))[:count] {
			result = append(result, machineIDs[j])
		}

		return nil
	})

	require.NoError(t, err)

	f(result)
}

func createOrUpdate[T resource.Resource](ctx context.Context, t *testing.T, s state.State, res T, update func(T) error, createOpts ...state.CreateOption) {
	require := require.New(t)

	cb := func(r T) error {
		for key, value := range res.Metadata().Labels().Raw() {
			r.Metadata().Labels().Set(key, value)
		}

		return update(r)
	}

	// try getting the resource first, and if it exists, skip attempting to create,
	// as relying on create to fail with conflict might not give the expected result due to validation errors
	_, err := s.Get(ctx, res.Metadata())
	notFound := state.IsNotFoundError(err)

	if err != nil && !notFound {
		require.NoError(err)
	}

	if notFound {
		toCreate := res.DeepCopy().(T) //nolint:forcetypeassert,errcheck

		require.NoError(cb(toCreate))

		err = s.Create(ctx, toCreate, createOpts...)
		if err == nil {
			return
		}

		if !state.IsConflictError(err) {
			require.NoError(err)
		}
	}

	if _, err = safe.StateUpdateWithConflicts(ctx, s, res.Metadata(), cb); err != nil {
		require.NoError(err)
	}
}

func waitMachineSetNodesSync(ctx context.Context, t *testing.T, st state.State, options ClusterOptions) {
	machineSets := []resource.ID{
		omni.ControlPlanesResourceID(options.Name),
		omni.WorkersResourceID(options.Name),
	}

	rtestutils.AssertResources(ctx, t, st, machineSets, func(status *omni.MachineSetStatus, assert *assert.Assertions) {
		spec := status.TypedSpec().Value

		ids := rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelMachineSet, status.Metadata().ID()),
		))

		assert.Equal(int(spec.Machines.Requested), len(ids), resourceDetails(status))
	})
}

func updateMachineClassMachineSets(ctx context.Context, t *testing.T, st state.State, options ClusterOptions, machineClass *omni.MachineClass) {
	machineAllocationLock.Lock()
	defer machineAllocationLock.Unlock()

	for _, role := range []string{omni.LabelControlPlaneRole, omni.LabelWorkerRole} {
		id := omni.WorkersResourceID(options.Name)
		machineCount := options.Workers

		if role == omni.LabelControlPlaneRole {
			id = omni.ControlPlanesResourceID(options.Name)
			machineCount = options.ControlPlanes
		}

		ms := omni.NewMachineSet(resources.DefaultNamespace, id)

		createOrUpdate(ctx, t, st, ms, func(r *omni.MachineSet) error {
			r.Metadata().Labels().Set(omni.LabelCluster, options.Name)
			r.Metadata().Labels().Set(role, "")

			switch {
			case machineClass != nil:
				r.TypedSpec().Value.MachineAllocation = &specs.MachineSetSpec_MachineAllocation{
					MachineCount: uint32(machineCount),
					Name:         machineClass.Metadata().ID(),
				}
			case r.TypedSpec().Value.MachineAllocation != nil:
				r.TypedSpec().Value.MachineAllocation.MachineCount += uint32(machineCount)
			}

			require.NotNilf(t, r.TypedSpec().Value.MachineAllocation, "the machine set doesn't have machine class set")

			r.TypedSpec().Value.UpdateStrategy = specs.MachineSetSpec_Rolling

			return nil
		})
	}

	waitMachineSetNodesSync(ctx, t, st, options)

	ids := rtestutils.ResourceIDs[*omni.MachineSetNode](ctx, t, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, options.Name)))

	// populate uuid patches for each machine matching the machine class
	for _, machineID := range ids {
		configPatch := omni.NewConfigPatch(
			resources.DefaultNamespace,
			fmt.Sprintf("000-%s-uuid-patch", machineID),
			pair.MakePair(omni.LabelCluster, options.Name),
			pair.MakePair(omni.LabelClusterMachine, machineID),
		)

		createOrUpdate(ctx, t, st, configPatch, func(cps *omni.ConfigPatch) error {
			cps.Metadata().Labels().Set(omni.LabelCluster, options.Name)
			cps.Metadata().Labels().Set(omni.LabelClusterMachine, machineID)

			return cps.TypedSpec().Value.SetUncompressedData([]byte(fmt.Sprintf(`machine:
  kubelet:
    extraArgs:
      node-labels: %s=%s`, nodeLabel, machineID)))
		})
	}
}

func AssertClusterIsImported(ctx context.Context, st state.State, clusterID string, secretsBundle []byte) TestFunc {
	return func(t *testing.T) {
		rtestutils.AssertResource[*omni.ClusterStatus](ctx, t, st, clusterID, func(status *omni.ClusterStatus, assert *assert.Assertions) {
			_, ok := status.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
			assert.True(ok, "cluster status doesn't have cluster tainted label")
		})

		var bundle *talossecrets.Bundle
		require.NoError(t, yaml.Unmarshal(secretsBundle, &bundle))

		talosConfig := omni.NewTalosConfig(resources.DefaultNamespace, clusterID)
		clientCert, err := talossecrets.NewAdminCertificateAndKey(time.Now(), bundle.Certs.OS, talosrole.MakeSet(talosrole.Admin), time.Hour)
		require.NoError(t, err)

		talosConfig.TypedSpec().Value.Key = base64.StdEncoding.EncodeToString(clientCert.Key)
		talosConfig.TypedSpec().Value.Crt = base64.StdEncoding.EncodeToString(clientCert.Crt)
		talosConfig.TypedSpec().Value.Ca = base64.StdEncoding.EncodeToString(bundle.Certs.OS.Crt)

		endpoints, err := safe.ReaderGetByID[*omni.ClusterEndpoint](ctx, st, clusterID)
		require.NoError(t, err)

		talosCli, err := talosclient.New(ctx,
			talosclient.WithCluster(clusterID),
			talosclient.WithConfig(omni.NewTalosClientConfig(talosConfig, endpoints.TypedSpec().Value.ManagementAddresses...)),
		)
		require.NoError(t, err)

		_, err = talosCli.Version(ctx)
		require.NoError(t, err)
	}
}

func AssertClusterImportIsAborted(ctx context.Context, options *TestOptions, clusterID string, secretsBundle []byte) TestFunc {
	return func(t *testing.T) {
		st := options.omniClient.Omni().State()
		httpEndpoint := options.HTTPEndpoint
		c := options.omniClient

		cluster, err := safe.StateGetByID[*omni.Cluster](ctx, st, clusterID)
		require.NoError(t, err)

		_, err = st.UpdateWithConflicts(ctx, cluster.Metadata(), func(r resource.Resource) error {
			r.Metadata().Annotations().Set(omni.ClusterLocked, "")

			return nil
		})
		require.NoError(t, err)

		clusterStatus, err := safe.StateGetByID[*omni.ClusterStatus](ctx, st, clusterID)
		require.NoError(t, err)

		_, ok := clusterStatus.Metadata().Labels().Get(omni.LabelClusterTaintedByImporting)
		require.True(t, ok, "cluster status doesn't have cluster tainted label")

		clusterMachines, err := safe.StateListAll[*omni.ClusterMachine](ctx, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterID)))
		require.NoError(t, err)
		require.NotEmpty(t, clusterMachines.Len(), "no cluster machines found for imported cluster %q", clusterID)

		var bundle *talossecrets.Bundle
		require.NoError(t, yaml.Unmarshal(secretsBundle, &bundle))

		talosConfig := omni.NewTalosConfig(resources.DefaultNamespace, clusterID)
		clientCert, err := talossecrets.NewAdminCertificateAndKey(time.Now(), bundle.Certs.OS, talosrole.MakeSet(talosrole.Admin), time.Hour)
		require.NoError(t, err)

		talosConfig.TypedSpec().Value.Key = base64.StdEncoding.EncodeToString(clientCert.Key)
		talosConfig.TypedSpec().Value.Crt = base64.StdEncoding.EncodeToString(clientCert.Crt)
		talosConfig.TypedSpec().Value.Ca = base64.StdEncoding.EncodeToString(bundle.Certs.OS.Crt)

		endpoints, err := safe.ReaderGetByID[*omni.ClusterEndpoint](ctx, st, clusterID)
		require.NoError(t, err)

		talosCli, err := talosclient.New(ctx,
			talosclient.WithCluster(clusterID),
			talosclient.WithConfig(omni.NewTalosClientConfig(talosConfig, endpoints.TypedSpec().Value.ManagementAddresses...)),
		)
		require.NoError(t, err)

		_, err = talosCli.Version(ctx)
		require.NoError(t, err)

		saName := "test-" + uuid.NewString()
		key := createServiceAccount(t.Context(), t, c, saName, role.Admin)

		stdout, _, err := runCmd(omnictlPath, httpEndpoint, key, "cluster", "import", "abort", clusterID)
		require.NotNil(t, stdout)
		stdoutStr := stdout.String()
		require.Contains(t, stdoutStr, "Aborting import operation for cluster")
		require.Contains(t, stdoutStr, "Import operation was aborted successfully for cluster")

		rtestutils.AssertNoResource[*omni.Cluster](ctx, t, st, clusterID)

		_, err = talosCli.Version(ctx)
		require.NoError(t, err)

		for cm := range clusterMachines.All() {
			wipeMachine(ctx, t, st, cm.Metadata().ID(), options.WipeAMachineFunc)
		}
	}
}

func prepareClusterImport(t *testing.T, name string, options *TestOptions) (ClusterOptions, []byte) {
	clusterOptions := ClusterOptions{
		Name:          name,
		ControlPlanes: 1,
		Workers:       0,

		MachineOptions:             options.MachineOptions,
		SkipExtensionCheckOnCreate: true,
	}

	bundle, err := talossecrets.NewBundle(talossecrets.NewFixedClock(time.Now()), config.TalosVersion1_11)
	require.NoError(t, err)

	bundleYaml, err := yaml.Marshal(bundle)
	require.NoError(t, err)

	ics := omni.NewImportedClusterSecrets(resources.DefaultNamespace, clusterOptions.Name)
	ics.TypedSpec().Value.Data = string(bundleYaml)

	require.NoError(t, options.omniClient.Omni().State().Create(t.Context(), ics))

	options.claimMachines(t, clusterOptions.ControlPlanes+clusterOptions.Workers)

	return clusterOptions, bundleYaml
}
