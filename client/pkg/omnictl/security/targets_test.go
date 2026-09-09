// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package security

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	imagefactorypb "github.com/siderolabs/omni/client/api/omni/imagefactory"
)

// White-box tests of resolveTargets/resolveVersions and the unexported target/targetFlags/
// versionFlags types they build. Converting to security_test would need constructor and getter
// scaffolding for every unexported field these tests construct and compare - a type alias can't
// expose them - for no real benefit in a leaf CLI package with no external API surface.

// fakeArtifactTargetsClient is a stand-in for *imagefactory.Client, recording the request it
// received and returning the canned response/error it was given.
type fakeArtifactTargetsClient struct {
	resp *imagefactorypb.ClusterArtifactTargetsResponse
	err  error

	gotRequest *imagefactorypb.ClusterArtifactTargetsRequest
}

func (f *fakeArtifactTargetsClient) ClusterArtifactTargets(
	_ context.Context, req *imagefactorypb.ClusterArtifactTargetsRequest,
) (*imagefactorypb.ClusterArtifactTargetsResponse, error) {
	f.gotRequest = req

	return f.resp, f.err
}

func TestResolveTargetsExplicit(t *testing.T) {
	client := &fakeArtifactTargetsClient{}

	targets, err := resolveTargets(t.Context(), client, targetFlags{
		schematicID:  "abc123",
		talosVersion: "1.9.0",
		arch:         "amd64",
	})
	require.NoError(t, err)
	require.Nil(t, client.gotRequest, "explicit mode must not call the server")

	require.Equal(t, []target{{
		schematicID: "abc123",
		arch:        imagefactorypb.Arch_AMD64,
		versions:    []string{"1.9.0"},
	}}, targets)
}

func TestResolveTargetsExplicitInvalidArch(t *testing.T) {
	_, err := resolveTargets(t.Context(), &fakeArtifactTargetsClient{}, targetFlags{
		schematicID:  "abc123",
		talosVersion: "1.9.0",
		arch:         "bogus",
	})
	require.ErrorContains(t, err, `invalid --arch "bogus"`)
}

func TestResolveTargetsCluster(t *testing.T) {
	client := &fakeArtifactTargetsClient{
		resp: &imagefactorypb.ClusterArtifactTargetsResponse{
			CurrentTalosVersion:   "1.9.0",
			UpgradeTargetVersions: []string{"1.9.2", "1.10.1"},
			Targets: []*imagefactorypb.ArtifactTarget{
				{SchematicId: "cp", Arch: imagefactorypb.Arch_AMD64, MachineCount: 2, IncludesControlPlane: true},
				{SchematicId: "worker", Arch: imagefactorypb.Arch_ARM64, MachineCount: 1},
			},
		},
	}

	t.Run("current version only", func(t *testing.T) {
		targets, err := resolveTargets(t.Context(), client, targetFlags{clusterID: "my-cluster"})
		require.NoError(t, err)
		require.Equal(t, "my-cluster", client.gotRequest.GetClusterId())

		require.Equal(t, []target{
			{schematicID: "cp", arch: imagefactorypb.Arch_AMD64, role: "includes control plane", machineCount: 2, versions: []string{"1.9.0"}},
			{schematicID: "worker", arch: imagefactorypb.Arch_ARM64, role: "worker", machineCount: 1, versions: []string{"1.9.0"}},
		}, targets)
	})

	t.Run("with upgrade paths", func(t *testing.T) {
		targets, err := resolveTargets(t.Context(), client, targetFlags{clusterID: "my-cluster", upgradePaths: true})
		require.NoError(t, err)

		for _, target := range targets {
			require.Equal(t, []string{"1.9.0", "1.9.2", "1.10.1"}, target.versions)
		}
	})
}

func TestTargetFlagsUpgradePathsRequiresClusterID(t *testing.T) {
	err := targetFlags{
		schematicID:  "abc123",
		talosVersion: "1.9.0",
		arch:         "amd64",
		upgradePaths: true,
	}.validate()
	require.ErrorIs(t, err, errUpgradePathsRequiresClusterID)

	require.NoError(t, targetFlags{clusterID: "my-cluster", upgradePaths: true}.validate())
}

func TestResolveVersionsExplicit(t *testing.T) {
	client := &fakeArtifactTargetsClient{}

	versions, err := resolveVersions(t.Context(), client, versionFlags{talosVersion: "1.9.0"})
	require.NoError(t, err)
	require.Nil(t, client.gotRequest, "explicit mode must not call the server")
	require.Equal(t, []string{"1.9.0"}, versions)
}

func TestResolveVersionsCluster(t *testing.T) {
	client := &fakeArtifactTargetsClient{
		resp: &imagefactorypb.ClusterArtifactTargetsResponse{
			CurrentTalosVersion:   "1.9.0",
			UpgradeTargetVersions: []string{"1.9.2", "1.10.1"},
		},
	}

	versions, err := resolveVersions(t.Context(), client, versionFlags{clusterID: "my-cluster"})
	require.NoError(t, err)
	require.Equal(t, []string{"1.9.0"}, versions)

	versions, err = resolveVersions(t.Context(), client, versionFlags{clusterID: "my-cluster", upgradePaths: true})
	require.NoError(t, err)
	require.Equal(t, []string{"1.9.0", "1.9.2", "1.10.1"}, versions)
}

func TestVersionFlagsUpgradePathsRequiresClusterID(t *testing.T) {
	err := versionFlags{talosVersion: "1.9.0", upgradePaths: true}.validate()
	require.ErrorIs(t, err, errUpgradePathsRequiresClusterID)

	require.NoError(t, versionFlags{clusterID: "my-cluster", upgradePaths: true}.validate())
}

func TestResolvedFromCluster(t *testing.T) {
	// The question is "did the command resolve its own targets", not "did it resolve more than
	// one" - a cluster running a single schematic still counts. See skipMissing.
	require.True(t, targetFlags{clusterID: "my-cluster"}.resolvedFromCluster())
	require.False(t, targetFlags{schematicID: "abc123", talosVersion: "1.9.0", arch: "amd64"}.resolvedFromCluster())
	require.True(t, versionFlags{clusterID: "my-cluster"}.resolvedFromCluster())
	require.False(t, versionFlags{talosVersion: "1.9.0"}.resolvedFromCluster())
}

func TestParseArch(t *testing.T) {
	arch, err := parseArch("amd64")
	require.NoError(t, err)
	require.Equal(t, imagefactorypb.Arch_AMD64, arch)

	arch, err = parseArch("arm64")
	require.NoError(t, err)
	require.Equal(t, imagefactorypb.Arch_ARM64, arch)

	_, err = parseArch("riscv64")
	require.Error(t, err)
}
