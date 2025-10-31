// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package accesspolicy_test

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
)

//go:embed testdata/acl-valid.yaml
var aclValidRaw []byte

//go:embed testdata/acl-valid-match-selector.yaml
var aclValidMatchSelectorRaw []byte

//go:embed testdata/acl-invalid-metadata.yaml
var aclInvalidMetadataRaw []byte

func TestCheck(t *testing.T) {
	accessPolicy := getAccessPolicy(t, aclValidRaw)

	checkResult, err := accesspolicy.Check(accessPolicy,
		omni.NewCluster(resources.DefaultNamespace, "cluster-group-1-cluster-1").Metadata(),
		auth.NewIdentity(resources.DefaultNamespace, "user-group-1-user-1").Metadata())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"k8s-group-1", "k8s-group-2"}, checkResult.KubernetesImpersonateGroups)

	checkResult, err = accesspolicy.Check(accessPolicy,
		omni.NewCluster(resources.DefaultNamespace, "cluster-group-1-cluster-1").Metadata(),
		auth.NewIdentity(resources.DefaultNamespace, "user-group-1-user-2").Metadata())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"k8s-group-1", "k8s-group-2"}, checkResult.KubernetesImpersonateGroups)

	checkResult, err = accesspolicy.Check(accessPolicy,
		omni.NewCluster(resources.DefaultNamespace, "cluster-group-1-cluster-1").Metadata(),
		auth.NewIdentity(resources.DefaultNamespace, "standalone-user-1").Metadata())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"k8s-group-1", "k8s-group-2"}, checkResult.KubernetesImpersonateGroups)

	checkResult, err = accesspolicy.Check(accessPolicy,
		omni.NewCluster(resources.DefaultNamespace, "standalone-cluster-1").Metadata(),
		auth.NewIdentity(resources.DefaultNamespace, "standalone-user-1").Metadata())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"k8s-group-1", "k8s-group-2"}, checkResult.KubernetesImpersonateGroups)

	checkResult, err = accesspolicy.Check(accessPolicy,
		omni.NewCluster(resources.DefaultNamespace, "cluster-group-1-cluster-1").Metadata(),
		auth.NewIdentity(resources.DefaultNamespace, "user-group-2-user-1").Metadata())
	require.NoError(t, err)
	assert.Empty(t, checkResult.KubernetesImpersonateGroups)

	checkResult, err = accesspolicy.Check(accessPolicy,
		omni.NewCluster(resources.DefaultNamespace, "cluster-group-2-cluster-1").Metadata(),
		auth.NewIdentity(resources.DefaultNamespace, "user-group-2-user-1").Metadata())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"k8s-group-3", "k8s-group-4"}, checkResult.KubernetesImpersonateGroups)

	checkResult, err = accesspolicy.Check(accessPolicy,
		omni.NewCluster(resources.DefaultNamespace, "cluster-group-2-cluster-1").Metadata(),
		auth.NewIdentity(resources.DefaultNamespace, "user-group-2-user-2").Metadata())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"k8s-group-3", "k8s-group-4"}, checkResult.KubernetesImpersonateGroups)

	checkResult, err = accesspolicy.Check(accessPolicy,
		omni.NewCluster(resources.DefaultNamespace, "cluster-group-2-cluster-1").Metadata(),
		auth.NewIdentity(resources.DefaultNamespace, "user-group-2-user-3").Metadata())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"k8s-group-3", "k8s-group-4"}, checkResult.KubernetesImpersonateGroups)

	checkResult, err = accesspolicy.Check(accessPolicy,
		omni.NewCluster(resources.DefaultNamespace, "cluster-group-2-cluster-1").Metadata(),
		auth.NewIdentity(resources.DefaultNamespace, "standalone-user-2").Metadata())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"k8s-group-3", "k8s-group-4"}, checkResult.KubernetesImpersonateGroups)

	checkResult, err = accesspolicy.Check(accessPolicy,
		omni.NewCluster(resources.DefaultNamespace, "standalone-cluster-2").Metadata(),
		auth.NewIdentity(resources.DefaultNamespace, "standalone-user-2").Metadata())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"k8s-group-3", "k8s-group-4"}, checkResult.KubernetesImpersonateGroups)

	checkResult, err = accesspolicy.Check(accessPolicy,
		omni.NewCluster(resources.DefaultNamespace, "cluster-group-2-cluster-1").Metadata(),
		auth.NewIdentity(resources.DefaultNamespace, "user-group-1-user-1").Metadata())
	require.NoError(t, err)
	assert.Empty(t, checkResult.KubernetesImpersonateGroups)
}

func TestValidateFailingTests(t *testing.T) {
	accessPolicy := getAccessPolicy(t, aclValidRaw)

	err := accesspolicy.Validate(accessPolicy)
	assert.NoError(t, err)

	accessPolicy.TypedSpec().Value.Tests[0].Expected.Kubernetes.Impersonate.Groups = []string{"k8s-group-1", "k8s-group-2", "k8s-group-3"}
	accessPolicy.TypedSpec().Value.Tests[1].Expected.Kubernetes.Impersonate.Groups = []string{"k8s-group-3", "k8s-group-4", "k8s-group-5"}

	err = accesspolicy.Validate(accessPolicy)
	assert.ErrorContains(t, err, "2 errors occurred")
	assert.ErrorContains(t, err, `access policy test "test-1" failed`)
	assert.ErrorContains(t, err, `access policy test "test-2" failed`)
}

func TestValidateInvalidMetadata(t *testing.T) {
	accessPolicy := getAccessPolicy(t, aclInvalidMetadataRaw)

	err := accesspolicy.Validate(accessPolicy)
	assert.ErrorContains(t, err, "2 errors occurred")
	assert.ErrorContains(t, err, `access policy ID mismatch`)
	assert.ErrorContains(t, err, `access policy namespace mismatch`)
}

func TestValidateInvalidGroups(t *testing.T) {
	accessPolicy := getAccessPolicy(t, aclValidRaw)

	err := accesspolicy.Validate(accessPolicy)
	assert.NoError(t, err)

	accessPolicy.TypedSpec().Value.UserGroups["user-group-1"].Users[0].Name = ""

	err = accesspolicy.Validate(accessPolicy)
	assert.ErrorContains(t, err, `"user-group-1" contains an empty user`)

	// revert to valid state
	accessPolicy = getAccessPolicy(t, aclValidRaw)

	accessPolicy.TypedSpec().Value.UserGroups["user-group-1"].Users[0].Match = "some-matcher"
	accessPolicy.TypedSpec().Value.UserGroups["user-group-2"].Users[0].LabelSelectors = []string{"some-selector"}

	accessPolicy.TypedSpec().Value.ClusterGroups["cluster-group-1"].Clusters[0].Match = "some-matcher"

	err = accesspolicy.Validate(accessPolicy)
	assert.ErrorContains(t, err, "3 errors occurred")
	assert.ErrorContains(t, err, `"user-group-1" contains a user with mutually exclusive fields set`)
	assert.ErrorContains(t, err, `"user-group-2" contains a user with mutually exclusive fields set`)
	assert.ErrorContains(t, err, `"cluster-group-1" contains a cluster with mutually exclusive fields set`)
}

func TestValidateWithMatchAndSelector(t *testing.T) {
	accessPolicy := getAccessPolicy(t, aclValidMatchSelectorRaw)

	err := accesspolicy.Validate(accessPolicy)
	assert.NoError(t, err)
}

func TestInvalidRole(t *testing.T) {
	accessPolicy := getAccessPolicy(t, aclValidRaw)

	accessPolicy.TypedSpec().Value.Rules[1].Role = "non-existent"

	err := accesspolicy.Validate(accessPolicy)
	assert.ErrorContains(t, err, "unknown role")
}

func getAccessPolicy(t *testing.T, raw []byte) *auth.AccessPolicy {
	dec := yaml.NewDecoder(bytes.NewReader(raw))

	var res protobuf.YAMLResource

	err := dec.Decode(&res)
	require.NoError(t, err)

	policy, ok := res.Resource().(*auth.AccessPolicy)
	require.True(t, ok, "resource is not an AccessPolicy")

	return policy
}
