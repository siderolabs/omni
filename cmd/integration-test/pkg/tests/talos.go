// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/backoff"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// clearConnectionRefused clears cached gRPC 'connection refused' error from Talos controlplane apid instances.
//
// When some node is unavailable, gRPC "caches" the error for backoff.DefaultConfig.MaxDelay.
func clearConnectionRefused(ctx context.Context, t *testing.T, c *talosclient.Client, numControlplanes int, nodes ...string) {
	ctx, cancel := context.WithTimeout(ctx, backoff.DefaultConfig.MaxDelay)
	defer cancel()

	require.NoError(t, retry.Constant(backoff.DefaultConfig.MaxDelay, retry.WithUnits(time.Second)).Retry(func() error {
		for range numControlplanes {
			_, err := c.Version(talosclient.WithNodes(ctx, nodes...))
			if err == nil {
				continue
			}

			if strings.Contains(err.Error(), "connection refused") {
				return retry.ExpectedError(err)
			}

			if strings.Contains(err.Error(), "connection reset by peer") {
				return retry.ExpectedError(err)
			}

			t.Logf("clear connection refused err %s", err)

			return err
		}

		return nil
	}))
}

// AssertTalosMaintenanceAPIAccessViaOmni verifies that cluster-wide `talosconfig` gives access to Talos nodes running in maintenance mode.
func AssertTalosMaintenanceAPIAccessViaOmni(testCtx context.Context, omniClient *client.Client, talosAPIKeyPrepare TalosAPIKeyPrepareFunc) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, backoff.DefaultConfig.MaxDelay+10*time.Second)
		t.Cleanup(cancel)

		require.NoError(t, talosAPIKeyPrepare(ctx, "default"))

		data, err := omniClient.Management().Talosconfig(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		config, err := clientconfig.FromBytes(data)
		require.NoError(t, err)

		maintenanceClient, err := talosclient.New(ctx, talosclient.WithConfig(config))
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, maintenanceClient.Close())
		})

		machines, err := safe.ReaderListAll[*omni.MachineStatus](ctx, omniClient.Omni().State(),
			state.WithLabelQuery(resource.LabelExists(omni.MachineStatusLabelAvailable)),
		)
		require.NoError(t, err)
		require.Greater(t, machines.Len(), 0)

		_, err = maintenanceClient.MachineClient.Version(talosclient.WithNode(ctx, machines.Get(0).Metadata().ID()), &emptypb.Empty{})
		require.NoError(t, err)
	}
}

// AssertTalosAPIAccessViaOmni verifies that both instance-wide and cluster-wide `talosconfig`s work with Omni Talos API proxy.
func AssertTalosAPIAccessViaOmni(testCtx context.Context, omniClient *client.Client, cluster string, talosAPIKeyPrepare TalosAPIKeyPrepareFunc) TestFunc {
	return func(t *testing.T) {
		ms, err := safe.ReaderListAll[*omni.MachineStatus](
			testCtx,
			omniClient.Omni().State(),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)),
		)
		require.NoError(t, err)

		cms, err := safe.StateListAll[*omni.ClusterMachineIdentity](
			testCtx,
			omniClient.Omni().State(),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, cluster)),
		)
		require.NoError(t, err)

		cpms, err := safe.StateListAll[*omni.ClusterMachine](
			testCtx,
			omniClient.Omni().State(),
			state.WithLabelQuery(
				resource.LabelEqual(omni.LabelCluster, cluster),
				resource.LabelExists(omni.LabelControlPlaneRole),
			),
		)
		require.NoError(t, err)

		machineNames, err := safe.Map(ms, func(m *omni.MachineStatus) (string, error) {
			return m.TypedSpec().Value.Network.Hostname, nil
		})
		require.NoError(t, err)

		machineIPs, err := safe.Map(cms, func(m *omni.ClusterMachineIdentity) (string, error) {
			return m.TypedSpec().Value.GetNodeIps()[0], nil
		})
		require.NoError(t, err)

		assert.Equal(t, len(machineNames), len(machineIPs))

		numControlPlanes := cpms.Len()

		assertTalosAPI := func(ctx context.Context, t *testing.T, c *talosclient.Client) {
			clearConnectionRefused(ctx, t, c, numControlPlanes, machineIPs...)

			// WithNodes - using IPs
			version, err := c.Version(talosclient.WithNodes(ctx, machineIPs...))
			assert.NoError(t, err)
			assert.Len(t, version.Messages, len(machineNames))

			assert.Equal(t,
				xslices.ToSet(machineIPs),
				xslices.ToSet(xslices.Map(version.Messages, func(m *machine.Version) string {
					return m.GetMetadata().GetHostname()
				})),
			)

			// WithNodes - using node names
			version, err = c.Version(talosclient.WithNodes(ctx, machineNames...))
			assert.NoError(t, err)
			assert.Len(t, version.Messages, len(machineNames))

			assert.Equal(t,
				xslices.ToSet(machineIPs),
				xslices.ToSet(xslices.Map(version.Messages, func(m *machine.Version) string {
					return m.GetMetadata().GetHostname()
				})),
			)

			// WithNode - using IP
			hostname, err := c.MachineClient.Hostname(talosclient.WithNode(ctx, machineIPs[0]), &emptypb.Empty{})
			assert.NoError(t, err)
			assert.Equal(t, machineNames[0], hostname.Messages[0].Hostname)
			assert.Empty(t, hostname.Messages[0].GetMetadata().GetHostname())

			// WithNode - using node name
			hostname, err = c.MachineClient.Hostname(talosclient.WithNode(ctx, machineNames[0]), &emptypb.Empty{})
			assert.NoError(t, err)
			assert.Equal(t, machineNames[0], hostname.Messages[0].Hostname)
			assert.Empty(t, hostname.Messages[0].GetMetadata().GetHostname())
		}

		t.Run("InstanceWideTalosconfig", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(testCtx, backoff.DefaultConfig.MaxDelay+10*time.Second)
			t.Cleanup(cancel)

			data, err := omniClient.Management().Talosconfig(ctx)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			config, err := clientconfig.FromBytes(data)
			require.NoError(t, err)

			require.NoError(t, talosAPIKeyPrepare(ctx, "default"))

			c, err := talosclient.New(ctx, talosclient.WithConfig(config), talosclient.WithCluster(cluster))
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, c.Close())
			})

			assertTalosAPI(ctx, t, c)
		})

		t.Run("ClusterWideTalosconfig", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(testCtx, backoff.DefaultConfig.MaxDelay+10*time.Second)
			t.Cleanup(cancel)

			data, err := omniClient.Management().WithCluster(cluster).Talosconfig(ctx)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			config, err := clientconfig.FromBytes(data)
			require.NoError(t, err)

			require.NoError(t, talosAPIKeyPrepare(ctx, fmt.Sprintf("%s-%s", "default", cluster)))

			c, err := talosclient.New(ctx, talosclient.WithConfig(config))
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, c.Close())
			})

			assertTalosAPI(ctx, t, c)
		})
	}
}

// AssertEtcdMembershipMatchesOmniResources checks that etcd members are in sync with the Omni MachineStatus information.
func AssertEtcdMembershipMatchesOmniResources(testCtx context.Context, client *client.Client, cluster string, talosAPIKeyPrepare TalosAPIKeyPrepareFunc) TestFunc {
	return func(t *testing.T) {
		require := require.New(t)
		assert := assert.New(t)

		ctx, cancel := context.WithTimeout(testCtx, 90*time.Second)
		defer cancel()

		require.NoError(talosAPIKeyPrepare(ctx, "default"))

		data, err := client.Management().Talosconfig(ctx)
		require.NoError(err)
		assert.NotEmpty(data)

		config, err := clientconfig.FromBytes(data)
		require.NoError(err)

		c, err := talosclient.New(ctx, talosclient.WithConfig(config), talosclient.WithCluster(cluster))
		require.NoError(err)

		t.Cleanup(func() {
			require.NoError(c.Close())
		})

		machineIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, client.Omni().State(),
			state.WithLabelQuery(
				resource.LabelEqual(omni.LabelCluster, cluster),
				resource.LabelExists(omni.LabelControlPlaneRole),
			),
		)

		clearConnectionRefused(ctx, t, c, len(machineIDs), machineIDs...)

		resp, err := c.EtcdMemberList(ctx, &machine.EtcdMemberListRequest{})
		require.NoError(err)

		err = retry.Constant(time.Minute*2, retry.WithUnits(time.Second)).Retry(func() error {
			clusterMachines := map[string]any{}

			for _, machineID := range machineIDs {
				var machineStatus *omni.MachineStatus

				machineStatus, err = safe.StateGet[*omni.MachineStatus](ctx, client.Omni().State(), omni.NewMachineStatus(resources.DefaultNamespace, machineID).Metadata())
				if err != nil {
					return retry.ExpectedError(err)
				}

				clusterMachines[machineStatus.TypedSpec().Value.Network.Hostname] = struct{}{}
			}

			for _, m := range resp.Messages {
				if len(clusterMachines) != len(m.Members) {
					memberIDs := xslices.Map(m.Members, func(m *machine.EtcdMember) string { return etcd.FormatMemberID(m.Id) })

					return retry.ExpectedErrorf("the count of members doesn't match the count of machines, expected %d, got: %d, members list: %s", len(clusterMachines), len(m.Members), memberIDs)
				}

				for _, member := range m.Members {
					_, ok := clusterMachines[member.Hostname]

					if !ok {
						return retry.ExpectedErrorf("found etcd member which doesn't have associated machine status")
					}
				}
			}

			return nil
		})

		require.NoError(err)
	}
}

// AssertTalosMembersMatchOmni checks that Talos discovery service members are in sync with the machines attached to the cluster.
func AssertTalosMembersMatchOmni(testCtx context.Context, client *client.Client, clusterName string, talosAPIKeyPrepare TalosAPIKeyPrepareFunc) TestFunc {
	return func(t *testing.T) {
		require := require.New(t)

		ctx, cancel := context.WithTimeout(testCtx, backoff.DefaultConfig.BaseDelay+120*time.Second)
		defer cancel()

		require.NoError(talosAPIKeyPrepare(ctx, "default"))

		data, err := client.Management().Talosconfig(ctx)
		require.NoError(err)

		config, err := clientconfig.FromBytes(data)
		require.NoError(err)

		c, err := talosclient.New(ctx, talosclient.WithConfig(config), talosclient.WithCluster(clusterName))
		require.NoError(err)

		t.Cleanup(func() {
			require.NoError(c.Close())
		})

		machineIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, client.Omni().State(),
			state.WithLabelQuery(
				resource.LabelEqual(omni.LabelCluster, clusterName),
			),
		)

		// map of Nodenames to Node Identities
		clusterMachines := map[string]string{}

		for _, machineID := range machineIDs {
			var clusterMachineIdentity *omni.ClusterMachineIdentity

			clusterMachineIdentity, err = safe.StateGet[*omni.ClusterMachineIdentity](ctx, client.Omni().State(),
				omni.NewClusterMachineIdentity(resources.DefaultNamespace, machineID).Metadata(),
			)

			require.NoError(err)

			machineStatus := clusterMachineIdentity.TypedSpec().Value

			clusterMachines[machineStatus.Nodename] = machineStatus.NodeIdentity
		}

		// check that every Omni machine is in Talos as a member
		rtestutils.AssertResources(ctx, t, c.COSI, maps.Keys(clusterMachines), func(member *cluster.Member, asrt *assert.Assertions) {
			asrt.Equal(clusterMachines[member.Metadata().ID()], member.TypedSpec().NodeID, resourceDetails(member))
		})

		// check that length of resources matches expectations (i.e. there are no extra members)
		rtestutils.AssertLength[*cluster.Member](ctx, t, c.COSI, len(clusterMachines))
	}
}

// AssertTalosVersion verifies Talos version on the nodes.
func AssertTalosVersion(testCtx context.Context, client *client.Client, clusterName, expectedVersion string, talosAPIKeyPrepare TalosAPIKeyPrepareFunc) TestFunc {
	return func(t *testing.T) {
		require := require.New(t)

		ctx, cancel := context.WithTimeout(testCtx, backoff.DefaultConfig.BaseDelay+90*time.Second)
		defer cancel()

		machineIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, client.Omni().State(), state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))

		cms, err := safe.StateListAll[*omni.ClusterMachineIdentity](
			testCtx,
			client.Omni().State(),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)
		require.NoError(err)

		machineIPs, err := safe.Map(cms, func(m *omni.ClusterMachineIdentity) (string, error) {
			if len(m.TypedSpec().Value.GetNodeIps()) == 0 {
				return "", fmt.Errorf("no ips discovered")
			}

			return m.TypedSpec().Value.GetNodeIps()[0], nil
		})
		require.NoError(err)

		// assert using Omni MachineStatus resource
		rtestutils.AssertResources(ctx, t, client.Omni().State(), machineIDs, func(r *omni.MachineStatus, asrt *assert.Assertions) {
			asrt.Equal(expectedVersion, strings.TrimLeft(r.TypedSpec().Value.TalosVersion, "v"), resourceDetails(r))
		})

		// assert issuing Talos API query to each machine
		require.NoError(talosAPIKeyPrepare(ctx, fmt.Sprintf("%s-%s", "default", clusterName)))

		data, err := client.Management().Talosconfig(ctx)
		require.NoError(err)
		require.NotEmpty(data)

		data, err = client.Management().WithCluster(clusterName).Talosconfig(ctx)
		require.NoError(err)
		require.NotEmpty(data)

		config, err := clientconfig.FromBytes(data)
		require.NoError(err)

		c, err := talosclient.New(ctx, talosclient.WithConfig(config), talosclient.WithCluster(clusterName))
		require.NoError(err)

		t.Cleanup(func() {
			require.NoError(c.Close())
		})

		require.NoError(retry.Constant(time.Minute, retry.WithUnits(time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
			clearConnectionRefused(ctx, t, c, len(machineIPs), machineIPs...)

			resp, err := c.Version(talosclient.WithNodes(ctx, machineIPs...))
			if err != nil {
				return err
			}

			for _, m := range resp.Messages {
				expected := "v" + expectedVersion

				if expected != m.Version.Tag {
					return retry.ExpectedErrorf("actual version doesn't match expected: %q != %q", m.Version.Tag, expected)
				}
			}

			return nil
		}))

		// assert using Talos upgrade controller status
		rtestutils.AssertResources(ctx, t, client.Omni().State(), []resource.ID{clusterName}, func(r *omni.TalosUpgradeStatus, asrt *assert.Assertions) {
			asrt.Equal(specs.TalosUpgradeStatusSpec_Done, r.TypedSpec().Value.Phase, resourceDetails(r))
			asrt.Equal(expectedVersion, r.TypedSpec().Value.LastUpgradeVersion, resourceDetails(r))
		})
	}
}

// AssertTalosUpgradeFlow verifies Talos upgrade flow to the new Talos version.
func AssertTalosUpgradeFlow(testCtx context.Context, st state.State, clusterName, newTalosVersion string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 15*time.Minute)
		defer cancel()

		// assert next version is in the list of upgrade candidate version
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.TalosUpgradeStatus, asrt *assert.Assertions) {
			asrt.Contains(r.TypedSpec().Value.UpgradeVersions, newTalosVersion, resourceDetails(r))
		})

		t.Logf("upgrading cluster %q to %q", clusterName, newTalosVersion)

		// trigger an upgrade
		_, err := safe.StateUpdateWithConflicts(ctx, st, omni.NewCluster(resources.DefaultNamespace, clusterName).Metadata(), func(cluster *omni.Cluster) error {
			cluster.TypedSpec().Value.TalosVersion = newTalosVersion

			return nil
		})
		require.NoError(t, err)

		// upgrade should start
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.TalosUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.TalosUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.NotEmpty(specs.TalosUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Step, resourceDetails(r))
			assert.NotEmpty(specs.TalosUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Status, resourceDetails(r))
		})

		t.Log("upgrade is going")

		// upgrade should finish successfully
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.TalosUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.TalosUpgradeStatusSpec_Done, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.Equal(newTalosVersion, r.TypedSpec().Value.LastUpgradeVersion, resourceDetails(r))
			assert.Empty(r.TypedSpec().Value.Step, resourceDetails(r))
		})
	}
}

// AssertTalosSchematicUpdateFlow verifies Talos schematic update flow.
func AssertTalosSchematicUpdateFlow(testCtx context.Context, client *client.Client, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 15*time.Minute)
		defer cancel()

		res := omni.NewExtensionsConfiguration(resources.DefaultNamespace, clusterName)

		res.Metadata().Labels().Set(omni.LabelCluster, clusterName)

		res.TypedSpec().Value.Extensions = []string{
			"siderolabs/hello-world-service",
			"siderolabs/qemu-guest-agent",
		}

		t.Logf("upgrading cluster schematic %q to have extensions %#v", clusterName, res.TypedSpec().Value.Extensions)

		err := client.Omni().State().Create(ctx, res)
		require.NoError(t, err)

		// upgrade should start
		rtestutils.AssertResources(ctx, t, client.Omni().State(), []resource.ID{clusterName}, func(r *omni.TalosUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.TalosUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.NotEmpty(specs.TalosUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Step, resourceDetails(r))
			assert.NotEmpty(specs.TalosUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Status, resourceDetails(r))
		})

		t.Log("upgrade is going")

		// upgrade should finish successfully
		rtestutils.AssertResources(ctx, t, client.Omni().State(), []resource.ID{clusterName}, func(r *omni.TalosUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.TalosUpgradeStatusSpec_Done, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.Empty(r.TypedSpec().Value.Step, resourceDetails(r))
		})
	}
}

// AssertTalosUpgradeIsRevertible tries to upgrade to invalid Talos version, and verifies that upgrade starts, fails, and can be reverted.
func AssertTalosUpgradeIsRevertible(testCtx context.Context, st state.State, clusterName, currentTalosVersion string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 15*time.Minute)
		defer cancel()

		badTalosVersion := currentTalosVersion + "-bad"

		t.Logf("attempting an upgrade of cluster %q to %q", clusterName, badTalosVersion)

		// trigger an upgrade to a bad version
		_, err := safe.StateUpdateWithConflicts(ctx, st, omni.NewCluster(resources.DefaultNamespace, clusterName).Metadata(), func(cluster *omni.Cluster) error {
			cluster.Metadata().Annotations().Set(constants.DisableValidation, "")

			cluster.TypedSpec().Value.TalosVersion = badTalosVersion

			return nil
		})
		require.NoError(t, err)

		// upgrade should start
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.TalosUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.TalosUpgradeStatusSpec_Upgrading, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.NotEmpty(r.TypedSpec().Value.Step, resourceDetails(r))
			assert.NotEmpty(r.TypedSpec().Value.Status, resourceDetails(r))
			assert.Equal(currentTalosVersion, r.TypedSpec().Value.LastUpgradeVersion, resourceDetails(r))
		})

		t.Log("revert an upgrade")

		_, err = safe.StateUpdateWithConflicts(ctx, st, omni.NewCluster(resources.DefaultNamespace, clusterName).Metadata(), func(cluster *omni.Cluster) error {
			cluster.TypedSpec().Value.TalosVersion = currentTalosVersion

			return nil
		})
		require.NoError(t, err)

		// upgrade should be reverted
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.TalosUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.TalosUpgradeStatusSpec_Done, r.TypedSpec().Value.Phase, resourceDetails(r))
			assert.Equal(currentTalosVersion, r.TypedSpec().Value.LastUpgradeVersion, resourceDetails(r))
			assert.Empty(r.TypedSpec().Value.Step, resourceDetails(r))
		})
	}
}

// AssertTalosUpgradeIsCancelable tries to upgrade Talos version, and verifies that upgrade starts on one of the nodes and immediately reverts it back.
func AssertTalosUpgradeIsCancelable(testCtx context.Context, st state.State, clusterName, currentTalosVersion, newTalosVersion string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 15*time.Minute)
		defer cancel()

		t.Logf("apply upgrade to %s", newTalosVersion)

		_, err := safe.StateUpdateWithConflicts(ctx, st, omni.NewCluster(resources.DefaultNamespace, clusterName).Metadata(), func(cluster *omni.Cluster) error {
			cluster.TypedSpec().Value.TalosVersion = newTalosVersion

			return nil
		})
		require.NoError(t, err)

		events := make(chan state.Event)

		require.NoError(t, st.WatchKind(ctx, omni.NewClusterMachineStatus(resources.DefaultNamespace, "").Metadata(), events),
			state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)),
		)

		ids := []string{}

		// wait until any of the machines' state changes to not running
	outer:
		for {
			select {
			case <-ctx.Done():
				require.NoError(t, ctx.Err())
			case ev := <-events:
				//nolint:exhaustive
				switch ev.Type {
				case state.Errored:
					require.NoError(t, ev.Error)
				case state.Updated,
					state.Created:
					res := ev.Resource.(*omni.ClusterMachineStatus) //nolint:forcetypeassert,errcheck

					if res.TypedSpec().Value.Stage != specs.ClusterMachineStatusSpec_RUNNING {
						var cmtv *omni.ClusterMachineTalosVersion

						cmtv, err = safe.ReaderGet[*omni.ClusterMachineTalosVersion](ctx, st, omni.NewClusterMachineTalosVersion(
							resources.DefaultNamespace,
							res.Metadata().ID(),
						).Metadata())

						// no information about the Talos version for the machine, continue looking
						if state.IsNotFoundError(err) {
							continue
						}

						require.NoError(t, err)

						// this machine is not running, but it wasn't updated, continue looking
						if cmtv.TypedSpec().Value.TalosVersion != newTalosVersion {
							continue
						}

						ids = append(ids, res.Metadata().ID())

						break outer
					}
				}
			}
		}

		// wait until the upgraded machine reports the new version
		rtestutils.AssertResources(ctx, t, st, ids, func(r *omni.MachineStatus, assert *assert.Assertions) {
			assert.Equal(newTalosVersion, strings.TrimLeft(r.TypedSpec().Value.TalosVersion, "v"), resourceDetails(r))
		})

		// revert the update
		_, err = safe.StateUpdateWithConflicts(ctx, st, omni.NewCluster(resources.DefaultNamespace, clusterName).Metadata(), func(cluster *omni.Cluster) error {
			cluster.TypedSpec().Value.TalosVersion = currentTalosVersion

			return nil
		})
		require.NoError(t, err)

		rtestutils.AssertResources(ctx, t, st, ids, func(r *omni.ClusterMachineStatus, assert *assert.Assertions) {
			assert.Equal(specs.ClusterMachineStatusSpec_RUNNING, r.TypedSpec().Value.Stage, resourceDetails(r))
		})

		// the upgraded machine version should be reverted back
		rtestutils.AssertResources(ctx, t, st, ids, func(r *omni.MachineStatus, assert *assert.Assertions) {
			assert.Equal(currentTalosVersion, strings.TrimLeft(r.TypedSpec().Value.TalosVersion, "v"), resourceDetails(r))
		})

		// the upgrade should be not running
		rtestutils.AssertResources(ctx, t, st, []resource.ID{clusterName}, func(r *omni.TalosUpgradeStatus, assert *assert.Assertions) {
			assert.Equal(specs.TalosUpgradeStatusSpec_Done, r.TypedSpec().Value.Phase, resourceDetails(r))
		})

		q := state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName))

		// ensure all machines now have the right version
		rtestutils.AssertResources(ctx, t, st, rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, st, q), func(r *omni.MachineStatus, assert *assert.Assertions) {
			assert.Equal(currentTalosVersion, strings.TrimLeft(r.TypedSpec().Value.TalosVersion, "v"), resourceDetails(r))
		})
	}
}

// AssertMachineShouldBeUpgradedInMaintenanceMode verifies machine upgrade in maintenance mode.
func AssertMachineShouldBeUpgradedInMaintenanceMode(
	testCtx context.Context,
	rootClient *client.Client,
	clusterName, kubernetesVersion, talosVersion1, talosVersion2 string,
	talosAPIKeyPrepare TalosAPIKeyPrepareFunc,
) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 900*time.Second)
		defer cancel()

		require := require.New(t)

		st := rootClient.Omni().State()

		var allocatedMachineIDs []resource.ID

		t.Logf("creating a cluster on version %s", talosVersion1)

		pickUnallocatedMachines(ctx, t, st, 1, func(machineIDs []resource.ID) {
			allocatedMachineIDs = machineIDs

			cluster := omni.NewCluster(resources.DefaultNamespace, clusterName)
			cluster.TypedSpec().Value.TalosVersion = talosVersion1
			cluster.TypedSpec().Value.KubernetesVersion = kubernetesVersion

			require.NoError(st.Create(ctx, cluster))

			t.Logf("Adding machine '%s' to control plane (cluster %q, version %s)", machineIDs[0], clusterName, talosVersion2)
			bindMachine(ctx, t, st, bindMachineOptions{
				clusterName: clusterName,
				role:        omni.LabelControlPlaneRole,
				machineID:   machineIDs[0],
			})

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

		// wait for initial cluster on talosVersion1 to be ready
		AssertClusterStatusReady(testCtx, st, clusterName)(t)
		AssertTalosVersion(testCtx, rootClient, clusterName, talosVersion2, talosAPIKeyPrepare)

		// destroy the cluster
		AssertDestroyCluster(testCtx, st, clusterName)(t)

		t.Logf("creating a cluster on version %s using same machines", talosVersion2)

		// re-create the cluster on talosVersion2
		cluster := omni.NewCluster(resources.DefaultNamespace, clusterName)
		cluster.TypedSpec().Value.TalosVersion = talosVersion2
		cluster.TypedSpec().Value.KubernetesVersion = kubernetesVersion

		require.NoError(st.Create(ctx, cluster))

		t.Logf("Adding machine '%s' to control plane (cluster %q, version %s)", allocatedMachineIDs[0], clusterName, talosVersion2)
		bindMachine(ctx, t, st, bindMachineOptions{
			clusterName: clusterName,
			role:        omni.LabelControlPlaneRole,
			machineID:   allocatedMachineIDs[0],
		})

		// wait for cluster on talosVersion2 to be ready
		AssertClusterStatusReady(testCtx, st, clusterName)(t)
		AssertTalosVersion(testCtx, rootClient, clusterName, talosVersion2, talosAPIKeyPrepare)

		AssertDestroyCluster(testCtx, st, clusterName)(t)
	}
}

// AssertTalosServiceIsRestarted verifies that Talos service is restarted on the nodes that match given cluster machine label query options.
func AssertTalosServiceIsRestarted(testCtx context.Context, cli *client.Client, clusterName string,
	talosAPIKeyPrepare TalosAPIKeyPrepareFunc, service string, labelQueryOpts ...resource.LabelQueryOption,
) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 1*time.Minute)
		defer cancel()

		require.NoError(t, talosAPIKeyPrepare(ctx, "default"))

		talosCli, err := talosClient(ctx, cli, clusterName)
		require.NoError(t, err)

		labelQueryOpts = append(labelQueryOpts, resource.LabelEqual(omni.LabelCluster, clusterName))

		clusterMachineList, err := safe.StateListAll[*omni.ClusterMachine](ctx, cli.Omni().State(), state.WithLabelQuery(labelQueryOpts...))
		require.NoError(t, err)

		for it := clusterMachineList.Iterator(); it.Next(); {
			clusterMachine := it.Value()
			nodeID := clusterMachine.Metadata().ID()

			t.Logf("Restarting service %q on node %q", service, nodeID)

			_, err = talosCli.MachineClient.ServiceRestart(talosclient.WithNode(ctx, nodeID), &machine.ServiceRestartRequest{
				Id: service,
			})
			require.NoError(t, err)
		}
	}
}

// AssertSupportBundleContents tries to upgrade get Talos/Omni support bundle, and verifies that it has some contents.
func AssertSupportBundleContents(testCtx context.Context, cli *client.Client, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
		defer cancel()

		require := require.New(t)

		progress := make(chan *management.GetSupportBundleResponse_Progress)

		var eg errgroup.Group

		eg.Go(func() error {
			for {
				select {
				case p := <-progress:
					if p == nil {
						return nil
					}

					if p.Error != "" {
						continue
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})

		data, err := cli.Management().GetSupportBundle(ctx, clusterName, progress)
		require.NoError(err)

		require.NoError(eg.Wait())

		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		require.NoError(err)

		readArchiveFile := func(path string) []byte {
			var (
				f    fs.File
				data []byte
			)

			f, err = archive.Open(path)
			require.NoError(err)

			defer f.Close() //nolint:errcheck

			data, err = io.ReadAll(f)
			require.NoError(err)

			return data
		}

		// check some resource that always exists
		require.NotEmpty(readArchiveFile(fmt.Sprintf("omni/resources/Clusters.omni.sidero.dev-%s.yaml", clusterName)))

		// check that all machines have logs
		machines, err := safe.ReaderListAll[*omni.ClusterMachine](ctx, cli.Omni().State(), state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		require.NoError(err)

		machines.ForEach(func(cm *omni.ClusterMachine) {
			require.NotEmpty(readArchiveFile(fmt.Sprintf("omni/machine-logs/%s.log", cm.Metadata().ID())))
		})

		// check kubernetes resources
		require.NotEmpty(readArchiveFile("kubernetesResources/systemPods.yaml"))
		require.NotEmpty(readArchiveFile("kubernetesResources/nodes.yaml"))

		nodes := map[string]struct{}{}

		for _, file := range archive.File {
			if strings.HasPrefix(file.Name, "omni/") || strings.HasPrefix(file.Name, "kubernetesResources/") {
				continue
			}

			base, _, ok := strings.Cut(file.Name, "/")
			if !ok {
				continue
			}

			nodes[base] = struct{}{}
		}

		// check some Talos resources
		for n := range nodes {
			require.NotEmpty(readArchiveFile(fmt.Sprintf("%s/dmesg.log", n)))
			require.NotEmpty(readArchiveFile(fmt.Sprintf("%s/service-logs/machined.log", n)))
			require.NotEmpty(readArchiveFile(fmt.Sprintf("%s/resources/nodenames.kubernetes.talos.dev.yaml", n)))
		}
	}
}
