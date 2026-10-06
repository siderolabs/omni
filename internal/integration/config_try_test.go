// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// tryModeMinTalosVersion is the first Talos version Omni applies config changes to in try mode.
var tryModeMinTalosVersion = semver.MustParse("1.14.0")

// blackholeOmniPatch returns a config patch that routes the machine's traffic to Omni into a blackhole.
func blackholeOmniPatch(ctx context.Context, t *testing.T, st state.State) string {
	apiConfig, err := safe.StateGetByID[*siderolink.APIConfig](ctx, st, siderolink.ConfigID)
	require.NoError(t, err)

	apiURL, err := url.Parse(apiConfig.TypedSpec().Value.MachineApiAdvertisedUrl)
	require.NoError(t, err)

	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", apiURL.Hostname())
	require.NoError(t, err)

	docs := make([]string, 0, len(ips))

	for _, ip := range ips {
		ip = ip.Unmap()

		docs = append(docs, fmt.Sprintf("apiVersion: v1alpha1\nkind: BlackholeRouteConfig\nname: %s\n", netip.PrefixFrom(ip, ip.BitLen())))
	}

	return strings.Join(docs, "---\n")
}

// skipUnlessTryMode skips the test when the machine would get the blackhole patch as a plain apply, or cannot run it at all.
func skipUnlessTryMode(t *testing.T, machineStatus *omni.MachineStatus) {
	if machineIsEmulated(machineStatus) {
		t.Skip("emulated machines do not roll back a config applied in try mode")
	}

	version, err := semver.ParseTolerant(machineStatus.TypedSpec().Value.TalosVersion)
	require.NoError(t, err)

	if version.LT(tryModeMinTalosVersion) {
		t.Skipf("try mode requires Talos %s or newer, the machine runs %q", tryModeMinTalosVersion, machineStatus.TypedSpec().Value.TalosVersion)
	}
}

// destroyPatchOnCleanup removes the patch even when the test fails, as it would cut the machine off for the following tests.
func destroyPatchOnCleanup(ctx context.Context, t *testing.T, st state.State, id resource.ID) {
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()

		if err := st.Destroy(cleanupCtx, omni.NewConfigPatch(id).Metadata()); err != nil && !state.IsNotFoundError(err) {
			t.Logf("failed to destroy config patch %q: %s", id, err)
		}
	})
}

// AssertBlackholeConfigPatchIsRolledBack applies a config patch that cuts a worker off from Omni, and checks that the
// machine rolls it back on its own and that Omni stops trying it. Removing the patch clears the error.
func AssertBlackholeConfigPatchIsRolledBack(testCtx context.Context, options *TestOptions, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 10*time.Minute)
		defer cancel()

		st := options.omniClient.Omni().State()

		workerIDs := rtestutils.ResourceIDs[*omni.ClusterMachine](ctx, t, st, state.WithLabelQuery(
			resource.LabelEqual(omni.LabelCluster, clusterName),
			resource.LabelExists(omni.LabelWorkerRole),
		))
		require.NotEmpty(t, workerIDs)

		machineID := workerIDs[0]

		machineStatus, err := safe.StateGetByID[*omni.MachineStatus](ctx, st, machineID)
		require.NoError(t, err)

		skipUnlessTryMode(t, machineStatus)

		var shaBefore string

		rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.ClusterMachineConfigStatus, assertion *assert.Assertions) {
			shaBefore = res.TypedSpec().Value.ClusterMachineConfigSha256

			assertion.NotEmpty(shaBefore)
			assertion.Nil(res.TypedSpec().Value.ConfigTry, resourceDetails(res))
		})

		patchID := fmt.Sprintf("000-config-patch-test-blackhole-%d", time.Now().Unix())
		patch := omni.NewConfigPatch(patchID)

		destroyPatchOnCleanup(ctx, t, st, patchID)

		createOrUpdate(ctx, t, st, patch, func(p *omni.ConfigPatch) error {
			p.Metadata().Labels().Set(omni.LabelCluster, clusterName)
			p.Metadata().Labels().Set(omni.LabelClusterMachine, machineID)

			return p.TypedSpec().Value.SetUncompressedData([]byte(blackholeOmniPatch(ctx, t, st)))
		})

		rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.ClusterMachineConfigStatus, assertion *assert.Assertions) {
			assertion.Contains(res.TypedSpec().Value.LastConfigError, "rolled back", resourceDetails(res))
			assertion.EqualValues(2, res.TypedSpec().Value.ConfigTry.GetAttempts(), resourceDetails(res))
			assertion.Equal(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256, "the blackhole config must never be recorded")
		})

		rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MachineStatus, assertion *assert.Assertions) {
			_, connected := res.Metadata().Labels().Get(omni.MachineStatusLabelConnected)

			assertion.True(connected, "the machine should be back on its previous config and connected")
		})

		rtestutils.Destroy[*omni.ConfigPatch](ctx, t, st, []string{patchID})

		rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.ClusterMachineConfigStatus, assertion *assert.Assertions) {
			assertion.Empty(res.TypedSpec().Value.LastConfigError, resourceDetails(res))
			assertion.Nil(res.TypedSpec().Value.ConfigTry, resourceDetails(res))
			assertion.Equal(shaBefore, res.TypedSpec().Value.ClusterMachineConfigSha256)
		})

		rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.ClusterMachineStatus, assertion *assert.Assertions) {
			assertion.Equal(specs.ClusterMachineStatusSpec_RUNNING, res.TypedSpec().Value.GetStage())
			assertion.True(res.TypedSpec().Value.GetReady())
			assertion.True(res.TypedSpec().Value.GetConfigUpToDate())
		})
	}
}

// testMaintenanceConfigTryMode applies a config patch that cuts a machine in maintenance off from Omni, and checks that
// the machine rolls it back on its own, that Omni stops trying it and shows the error on the machine. Removing the
// patch clears the error.
func testMaintenanceConfigTryMode(t *testing.T, options *TestOptions) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()

	options.claimMachines(t, 1)

	st := options.omniClient.Omni().State()

	machineID := pickUnallocatedMachines(ctx, t, st, 1, func(machineStatus *omni.MachineStatus, _ []*omni.MachineStatus) bool {
		return machineStatus.TypedSpec().Value.Maintenance
	})[0]

	machineStatus, err := safe.StateGetByID[*omni.MachineStatus](ctx, st, machineID)
	require.NoError(t, err)

	skipUnlessTryMode(t, machineStatus)

	t.Logf("cutting off machine in maintenance: %s", machineID)

	var hashBefore string

	rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MaintenanceConfigStatus, assertion *assert.Assertions) {
		hashBefore = res.TypedSpec().Value.LastAppliedConfigHash

		assertion.NotEmpty(hashBefore)
		assertion.Nil(res.TypedSpec().Value.ConfigTry, resourceDetails(res))
	})

	patchID := fmt.Sprintf("000-maintenance-config-test-blackhole-%d", time.Now().Unix())
	patch := omni.NewConfigPatch(patchID)

	destroyPatchOnCleanup(ctx, t, st, patchID)

	createOrUpdate(ctx, t, st, patch, func(p *omni.ConfigPatch) error {
		p.Metadata().Labels().Set(omni.LabelMachine, machineID)

		return p.TypedSpec().Value.SetUncompressedData([]byte(blackholeOmniPatch(ctx, t, st)))
	})

	rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MaintenanceConfigStatus, assertion *assert.Assertions) {
		assertion.Contains(res.TypedSpec().Value.LastConfigError, "rolled back", resourceDetails(res))
		assertion.EqualValues(2, res.TypedSpec().Value.ConfigTry.GetAttempts(), resourceDetails(res))
		assertion.Equal(hashBefore, res.TypedSpec().Value.LastAppliedConfigHash, "the blackhole config must never be recorded")
	})

	rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MachineStatusLink, assertion *assert.Assertions) {
		assertion.Contains(res.TypedSpec().Value.MaintenanceConfigError, "rolled back", resourceDetails(res))
	})

	rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MachineStatus, assertion *assert.Assertions) {
		_, connected := res.Metadata().Labels().Get(omni.MachineStatusLabelConnected)

		assertion.True(connected, "the machine should be back on its previous config and connected")
	})

	rtestutils.Destroy[*omni.ConfigPatch](ctx, t, st, []string{patchID})

	rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MaintenanceConfigStatus, assertion *assert.Assertions) {
		assertion.Empty(res.TypedSpec().Value.LastConfigError, resourceDetails(res))
		assertion.Nil(res.TypedSpec().Value.ConfigTry, resourceDetails(res))
		assertion.Equal(hashBefore, res.TypedSpec().Value.LastAppliedConfigHash)
	})

	rtestutils.AssertResource(ctx, t, st, machineID, func(res *omni.MachineStatusLink, assertion *assert.Assertions) {
		assertion.Empty(res.TypedSpec().Value.MaintenanceConfigError, resourceDetails(res))
	})
}
