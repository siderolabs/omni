// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/dustin/go-humanize"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// etcFileConfigMinTalosVersion is the minimum Talos version that knows the EtcFileConfig document.
var etcFileConfigMinTalosVersion = semver.MustParse("1.14.0")

const (
	dummyIfacePatchTemplate = `machine:
  network:
    interfaces:
      - interface: %s
        dummy: true`
)

// AssertLargeImmediateConfigApplied tests that config patch that be applied immediately (without reboot) gets applied to all the nodes.
//
// Config patch is generated to be large to test the edge case.
// The patch is removed on finalize.
func AssertLargeImmediateConfigApplied(testCtx context.Context, options *TestOptions, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 3*time.Minute)
		defer cancel()

		talosClient := getTalosClientForCluster(ctx, t, options, clusterName)

		nodeIPs, err := talosNodeIPs(ctx, talosClient.COSI)
		require.NoError(t, err)

		st := options.omniClient.Omni().State()

		epochSeconds := time.Now().Unix()
		id := fmt.Sprintf("000-config-patch-test-dummy-iface-%d", epochSeconds)
		iface := fmt.Sprintf("dummy%d", epochSeconds)
		configPatchYAML := fmt.Sprintf(dummyIfacePatchTemplate, iface)

		var sb strings.Builder

		for range 40_000 {
			sb.WriteString("################################################################################\n")
		}

		sb.WriteString(configPatchYAML)
		sb.WriteString("\n")

		sizeHumanReadable := humanize.Bytes(uint64(sb.Len()))

		require.LessOrEqual(t, sb.Len(), 4*1024*1024, "generated config patch is too large (%v), abort", sizeHumanReadable)

		t.Logf("creating large config patch with size: %v", sizeHumanReadable)

		configPatch := omni.NewConfigPatch(id, pair.MakePair(omni.LabelCluster, clusterName))

		// apply the large config patch that creates a dummy interface
		createOrUpdate(ctx, t, st, configPatch, func(p *omni.ConfigPatch) error {
			return p.TypedSpec().Value.SetUncompressedData([]byte(configPatchYAML))
		})

		t.Logf("created large config patch with dummy interface: %q", iface)

		// assert that the patch is propagated to all clustermachines
		cmIDs := rtestutils.ResourceIDs[*omni.ClusterMachineConfigPatches](ctx, t, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))

		rtestutils.AssertResources(ctx, t, st, cmIDs, func(cm *omni.ClusterMachineConfigPatches, assertion *assert.Assertions) {
			assertion.True(clusterMachinePatchesContainsString(t, cm, iface), "cluster machine %q patches don't contain string %q", cm.Metadata().ID(), iface)
		})

		linkStatus := network.NewLinkStatus(network.NamespaceName, "")

		containsDummyIface := func(node string) (bool, error) {
			links, linksErr := talosClient.COSI.List(talosclient.WithNode(ctx, node), linkStatus.Metadata())
			if linksErr != nil {
				return false, linksErr
			}

			for _, res := range links.Items {
				if res.Metadata().ID() == iface {
					return true, nil
				}
			}

			return false, nil
		}

		for _, node := range nodeIPs {
			err = retry.Constant(3*time.Minute, retry.WithUnits(1*time.Second)).RetryWithContext(ctx, func(context.Context) error {
				contains, containsErr := containsDummyIface(node)
				if containsErr != nil {
					return containsErr
				}

				if !contains {
					return retry.ExpectedError(fmt.Errorf("%q: dummy interface %q not found", node, iface))
				}

				return nil
			})

			assert.NoError(t, err)
		}

		rtestutils.Destroy[*omni.ConfigPatch](ctx, t, st, []string{configPatch.Metadata().ID()})

		t.Logf("destroyed config patch with dummy interface: %q", iface)

		// assert that the patch deletion is propagated to all clustermachines
		rtestutils.AssertResources(ctx, t, st, cmIDs, func(cm *omni.ClusterMachineConfigPatches, assertion *assert.Assertions) {
			assertion.False(clusterMachinePatchesContainsString(t, cm, iface), "cluster machine %q patches contain string %q", cm.Metadata().ID(), iface)
		})
	}
}

// AssertConfigPatchWritingFileIsApplied tests that a config patch which writes a file lands on the machine.
//
// Talos 1.14 and later apply configuration to the running machine, and manage user owned files under /etc
// through a dedicated document, so no reboot is involved on either side. The document also makes the test
// need Talos 1.14 or later, since older versions do not know it. The test does not try to prove the
// absence of a reboot, which it cannot do without watching for a transient state, it only checks that the
// machine is healthy once the file is there.
func AssertConfigPatchWritingFileIsApplied(testCtx context.Context, options *TestOptions, clusterName string) TestFunc {
	return func(t *testing.T) {
		parsedVersion, err := semver.ParseTolerant(options.MachineOptions.TalosVersion)
		require.NoError(t, err)

		parsedVersion.Pre = nil
		parsedVersion.Build = nil

		if parsedVersion.LT(etcFileConfigMinTalosVersion) {
			t.Skipf("EtcFileConfig requires Talos %s or newer, test runs on %q", etcFileConfigMinTalosVersion, options.MachineOptions.TalosVersion)
		}

		omniClient := options.omniClient

		ctx, cancel := context.WithTimeout(testCtx, 3*time.Minute)
		defer cancel()

		talosClient := getTalosClientForCluster(ctx, t, options, clusterName)

		st := omniClient.Omni().State()

		nodeList, err := nodes(ctx, t, options, clusterName, resource.LabelExists(omni.LabelWorkerRole))
		require.NoError(t, err)
		require.Greater(t, len(nodeList), 0)

		node := nodeList[0]

		nodeID := node.machine.Metadata().ID()

		epochSeconds := time.Now().Unix()
		id := fmt.Sprintf("000-config-patch-test-file-%d", epochSeconds)

		// the document name is relative to /etc, so keep it separate from the path used to read the file back
		name := fmt.Sprintf("config-patch-test-file-%d.txt", epochSeconds)
		file := "/etc/" + name

		configPatchYAML := fmt.Sprintf(`apiVersion: v1alpha1
kind: EtcFileConfig
name: %s
contents: test`, name)

		configPatch := omni.NewConfigPatch(id,
			pair.MakePair(omni.LabelCluster, clusterName),
			pair.MakePair(omni.LabelClusterMachine, nodeID))

		// apply the config patch that creates a file
		createOrUpdate(ctx, t, st, configPatch, func(p *omni.ConfigPatch) error {
			return p.TypedSpec().Value.SetUncompressedData([]byte(configPatchYAML))
		})

		t.Logf("created config patch with file: %q", file)

		// assert that the patch is propagated to the cluster machine
		rtestutils.AssertResources(ctx, t, st, []resource.ID{nodeID}, func(cm *omni.ClusterMachineConfigPatches, assertion *assert.Assertions) {
			assertion.True(clusterMachinePatchesContainsString(t, cm, name), "cluster machine %q patches don't contain string %q", cm.Metadata().ID(), name)
		})

		// assert that the file is created on the node
		err = retry.Constant(2*time.Minute, retry.WithUnits(1*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
			exists, existsErr := talosFileExists(ctx, talosClient, node.talosIP, file)
			if existsErr != nil {
				if strings.Contains(existsErr.Error(), "not reachable") || status.Code(existsErr) == codes.Unavailable {
					return retry.ExpectedError(existsErr)
				}

				return existsErr
			}

			if !exists {
				return retry.ExpectedError(fmt.Errorf("%q: file %q is not found", node.talosIP, file))
			}

			t.Logf("file %q is found on node %q", file, node.talosIP)

			return nil
		})

		assert.NoError(t, err)

		// the machine must be healthy once the file is there
		rtestutils.AssertResources(ctx, t, st, []resource.ID{nodeID}, func(cms *omni.ClusterMachineStatus, assertion *assert.Assertions) {
			assertion.Equal(specs.ClusterMachineStatusSpec_RUNNING, cms.TypedSpec().Value.GetStage())
		})

		rtestutils.Destroy[*omni.ConfigPatch](ctx, t, st, []string{configPatch.Metadata().ID()})
	}
}

// AssertConfigPatchWithInvalidConfig tests that invalid config patch is not applied and the error is reported in the status.
func AssertConfigPatchWithInvalidConfig(testCtx context.Context, cli *client.Client, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 8*time.Minute)
		defer cancel()

		st := cli.Omni().State()

		cmIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](
			ctx, t, st,
			state.WithLabelQuery(
				resource.LabelEqual(omni.LabelCluster, clusterName),
				resource.LabelExists(omni.LabelControlPlaneRole),
			),
		)
		require.NotEmpty(t, cmIDs)

		cmID := cmIDs[0]

		epochSeconds := time.Now().Unix()
		id := fmt.Sprintf("000-config-patch-test-broken-%d", epochSeconds)
		configPatchYAML := `apiVersion: v1alpha1
kind: SwapVolumeConfig
name: swap
provisioning:
  maxSize: 5GB
  minSize: 5GB`

		configPatch := omni.NewConfigPatch(id,
			pair.MakePair(omni.LabelCluster, clusterName),
			pair.MakePair(omni.LabelClusterMachine, cmID))

		// apply the broken config patch
		createOrUpdate(ctx, t, st, configPatch, func(p *omni.ConfigPatch) error {
			return p.TypedSpec().Value.SetUncompressedData([]byte(configPatchYAML))
		})

		rtestutils.AssertResources(ctx, t, st, []resource.ID{cmID}, func(cmcs *omni.ClusterMachineConfigStatus, assertion *assert.Assertions) {
			assertion.Contains(cmcs.TypedSpec().Value.GetLastConfigError(), "disk selector is required")
		})

		// remove broken config patch
		rtestutils.Destroy[*omni.ConfigPatch](ctx, t, st, []string{configPatch.Metadata().ID()})

		// wait until k8s nodes come back
		rtestutils.AssertResources(ctx, t, st, []resource.ID{cmID}, func(cms *omni.ClusterMachineStatus, assertion *assert.Assertions) {
			assertion.Equal(specs.ClusterMachineStatusSpec_RUNNING, cms.TypedSpec().Value.GetStage())
			assertion.True(cms.TypedSpec().Value.GetReady())
		})
	}
}

// AssertConfigPatchMachineSet applies config patch on a single machine set.
//
// The patch is removed at the end.
func AssertConfigPatchMachineSet(testCtx context.Context, cli *client.Client, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 30*time.Second)
		defer cancel()

		st := cli.Omni().State()

		workerMachineSetName := omni.WorkersResourceID(clusterName)
		controlPlaneMachineSetName := omni.ControlPlanesResourceID(clusterName)

		epochSeconds := time.Now().Unix()
		id := fmt.Sprintf("000-config-patch-test-dummy-iface-%d", epochSeconds)
		iface := fmt.Sprintf("dummy%d", epochSeconds)
		configPatchYAML := fmt.Sprintf(dummyIfacePatchTemplate, iface)

		configPatch := omni.NewConfigPatch(
			id,
			pair.MakePair(omni.LabelCluster, clusterName),
			pair.MakePair(omni.LabelMachineSet, workerMachineSetName),
		)

		// apply the config patch that creates a dummy interface
		createOrUpdate(ctx, t, st, configPatch, func(p *omni.ConfigPatch) error {
			return p.TypedSpec().Value.SetUncompressedData([]byte(configPatchYAML))
		})

		// assert that the patch is propagated to all worker machine set ClusterMachines
		workerIDs := rtestutils.ResourceIDs[*omni.ClusterMachineConfigPatches](ctx, t, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, workerMachineSetName)))

		rtestutils.AssertResources(ctx, t, st, workerIDs, func(cm *omni.ClusterMachineConfigPatches, assertion *assert.Assertions) {
			assertion.True(clusterMachinePatchesContainsString(t, cm, iface), "cluster machine %q patches don't contain string %q", cm.Metadata().ID(), iface)
		})

		// assert that the patch is *NOT* propagated to all controlplane machine set ClusterMachines
		controlPlaneIDs := rtestutils.ResourceIDs[*omni.ClusterMachineConfigPatches](ctx, t, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineSet, controlPlaneMachineSetName)))

		rtestutils.AssertResources(ctx, t, st, controlPlaneIDs, func(cm *omni.ClusterMachineConfigPatches, assertion *assert.Assertions) {
			assertion.False(clusterMachinePatchesContainsString(t, cm, iface), "cluster machine %q patches contain string %q", cm.Metadata().ID(), iface)
		})

		// cleanup
		rtestutils.Destroy[*omni.ConfigPatch](ctx, t, st, []string{configPatch.Metadata().ID()})
	}
}

// AssertConfigPatchSingleClusterMachine applies a config patch on a single cluster machine.
//
// The patch is removed at the end.
func AssertConfigPatchSingleClusterMachine(testCtx context.Context, cli *client.Client, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 30*time.Second)
		defer cancel()

		st := cli.Omni().State()

		// get a single clustermachine
		cmIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, st, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, clusterName)))
		require.NotEmpty(t, cmIDs)

		cmID := cmIDs[0]

		epochSeconds := time.Now().Unix()
		id := fmt.Sprintf("000-config-patch-test-dummy-iface-%d", epochSeconds)
		iface := fmt.Sprintf("dummy%d", epochSeconds)
		configPatchYAML := fmt.Sprintf(dummyIfacePatchTemplate, iface)

		configPatch := omni.NewConfigPatch(id,
			pair.MakePair(omni.LabelCluster, clusterName),
			pair.MakePair(omni.LabelClusterMachine, cmID))

		// apply the config patch
		createOrUpdate(ctx, t, st, configPatch, func(p *omni.ConfigPatch) error {
			return p.TypedSpec().Value.SetUncompressedData([]byte(configPatchYAML))
		})

		// assert that the patch is propagated to the clustermachine
		rtestutils.AssertResources(ctx, t, st, []resource.ID{cmID}, func(r *omni.ClusterMachineConfigPatches, assertion *assert.Assertions) {
			assertion.True(clusterMachinePatchesContainsString(t, r, iface), "cluster machine %q patches don't contain string %q", r.Metadata().ID(), iface)
		})

		// assert that the patch is *NOT* propagated to other clustermachines of the cluster
		otherCMIDs := cmIDs[1:]

		rtestutils.AssertResources(ctx, t, st, otherCMIDs, func(r *omni.ClusterMachineConfigPatches, assertion *assert.Assertions) {
			assertion.False(clusterMachinePatchesContainsString(t, r, iface), "cluster machine %q patches contain string %q", r.Metadata().ID(), iface)
		})

		// cleanup
		rtestutils.Destroy[*omni.ConfigPatch](ctx, t, st, []string{configPatch.Metadata().ID()})
	}
}

func clusterMachinePatchesContainsString(t *testing.T, clusterMachineConfigPatches *omni.ClusterMachineConfigPatches, str string) bool {
	patches, err := clusterMachineConfigPatches.TypedSpec().Value.GetUncompressedPatches()
	require.NoError(t, err)

	for _, patch := range patches {
		if strings.Contains(patch, str) {
			return true
		}
	}

	return false
}

func talosFileExists(ctx context.Context, talosClient *talosclient.Client, node, filePath string) (bool, error) {
	resp, err := talosClient.MachineClient.List(talosclient.WithNode(ctx, node), &machine.ListRequest{Root: filePath})
	if err != nil {
		return false, err
	}

	for {
		var fileInfo *machine.FileInfo

		fileInfo, err = resp.Recv()
		if errors.Is(err, io.EOF) {
			return false, nil
		}

		if err != nil {
			if strings.Contains(err.Error(), "no such file or directory") {
				return false, nil
			}

			return false, err
		}

		if fileInfo.GetError() != "" {
			return false, errors.New(fileInfo.GetError())
		}

		if fileInfo.Name == filepath.Clean(filePath) {
			return true, nil
		}
	}
}
