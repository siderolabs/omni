// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes_test

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/pkg/omni/resources/k8s"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
)

func TestResourceConversion(t *testing.T) {
	pod := &v1.Pod{}
	pod.Name = "test"
	pod.Namespace = "test"

	data, err := json.Marshal(pod)
	assert.NoError(t, err)

	typedRes := typed.NewResource[k8s.KubernetesResourceSpec, k8s.KubernetesResourceExtension](
		resource.NewMetadata("default", k8s.KubernetesResourceType, "pod", resource.VersionUndefined),
		k8s.KubernetesResourceSpec(data),
	)

	res, err := kubernetes.UnstructuredFromResource(typedRes)
	assert.NoError(t, err)
	name, ok, err := unstructured.NestedString(res.Object, "metadata", "name")
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, pod.Name, name)
}

//go:embed testdata/oidc-kubeconfig1.yaml
var oidcKubeconfig1 []byte

//go:embed testdata/oidc-kubeconfig2.yaml
var oidcKubeconfig2 []byte

func TestOIDCKubeconfig(t *testing.T) {
	r, err := kubernetes.New(nil)
	require.NoError(t, err)

	kubeconfig, err := r.GetOIDCKubeconfig(&common.Context{
		Name: "cluster1",
	}, "test@example.com")
	require.NoError(t, err)

	assert.Equal(t, string(oidcKubeconfig1), string(kubeconfig))

	kubeconfig, err = r.GetOIDCKubeconfig(&common.Context{
		Name: "cluster1",
	}, "")
	require.NoError(t, err)

	assert.Equal(t, string(oidcKubeconfig2), string(kubeconfig))
}
