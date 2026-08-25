// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/client/api/omni/specs"
)

func TestMigrateAppliedGroup(t *testing.T) {
	x := &specs.ClusterKubernetesManifestsStatusSpec{
		Groups: map[string]*specs.ClusterKubernetesManifestsStatusSpec_GroupStatus{
			"200-cluster-x-a": {
				Mode:  specs.KubernetesManifestGroupSpec_ONE_TIME,
				Phase: specs.ClusterKubernetesManifestsStatusSpec_GroupStatus_APPLIED,
				Manifests: map[string]*specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus{
					"Job/default/migrate": {Phase: specs.ClusterKubernetesManifestsStatusSpec_ManifestStatus_APPLIED},
				},
			},
		},
	}

	x.MigrateAppliedGroup("200-cluster-x-a", "cluster-x-a")

	_, stillHasOld := x.Groups["200-cluster-x-a"]
	assert.False(t, stillHasOld)

	migrated, ok := x.Groups["cluster-x-a"]
	assert.True(t, ok)
	assert.Equal(t, specs.ClusterKubernetesManifestsStatusSpec_GroupStatus_APPLIED, migrated.Phase)
	assert.Contains(t, migrated.Manifests, "Job/default/migrate")
}

func TestMigrateAppliedGroup_NoOpWhenSourceMissing(t *testing.T) {
	x := &specs.ClusterKubernetesManifestsStatusSpec{}
	x.MigrateAppliedGroup("200-cluster-x-a", "cluster-x-a")
	assert.Empty(t, x.Groups)
}

func TestMigrateAppliedGroup_DoesNotClobberExistingNewID(t *testing.T) {
	live := &specs.ClusterKubernetesManifestsStatusSpec_GroupStatus{Phase: specs.ClusterKubernetesManifestsStatusSpec_GroupStatus_PROGRESSING}

	x := &specs.ClusterKubernetesManifestsStatusSpec{
		Groups: map[string]*specs.ClusterKubernetesManifestsStatusSpec_GroupStatus{
			"200-cluster-x-a": {Mode: specs.KubernetesManifestGroupSpec_ONE_TIME, Phase: specs.ClusterKubernetesManifestsStatusSpec_GroupStatus_APPLIED},
			"cluster-x-a":     live,
		},
	}

	x.MigrateAppliedGroup("200-cluster-x-a", "cluster-x-a")

	assert.Same(t, live, x.Groups["cluster-x-a"]) // untouched
	_, stillHasOld := x.Groups["200-cluster-x-a"]
	assert.False(t, stillHasOld) // legacy entry still removed
}

func TestMigrateAppliedGroup_RejectsNonAppliedOrFullMode(t *testing.T) {
	x := &specs.ClusterKubernetesManifestsStatusSpec{
		Groups: map[string]*specs.ClusterKubernetesManifestsStatusSpec_GroupStatus{
			"200-cluster-x-a": {Mode: specs.KubernetesManifestGroupSpec_ONE_TIME, Phase: specs.ClusterKubernetesManifestsStatusSpec_GroupStatus_PROGRESSING},
			"200-cluster-x-b": {Mode: specs.KubernetesManifestGroupSpec_FULL, Phase: specs.ClusterKubernetesManifestsStatusSpec_GroupStatus_APPLIED},
		},
	}

	x.MigrateAppliedGroup("200-cluster-x-a", "cluster-x-a")
	x.MigrateAppliedGroup("200-cluster-x-b", "cluster-x-b")

	assert.Len(t, x.Groups, 2) // both left untouched
}
