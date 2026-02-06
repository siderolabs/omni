// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes_test

import (
	"context"
	_ "embed"
	"encoding/json"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/pkg/omni/resources/k8s"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
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

//go:embed testdata/oidc-kubeconfig3.yaml
var oidcKubeconfig3 []byte

//go:embed testdata/admin-kubeconfig.yaml
var adminKubeconfig []byte

func TestOIDCKubeconfig(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := kubernetes.New(nil, logger, "http://localhost:8080/oidc", "default", "https://localhost:8095")

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

func TestOIDCKubeconfigWithExtraOptions(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := kubernetes.New(nil, logger, "http://localhost:8080/oidc", "default", "https://localhost:8095")

	kubeconfig, err := r.GetOIDCKubeconfig(&common.Context{
		Name: "cluster1",
	}, "test@example.com")
	require.NoError(t, err)

	assert.Equal(t, string(oidcKubeconfig1), string(kubeconfig))

	kubeconfig, err = r.GetOIDCKubeconfig(&common.Context{
		Name: "cluster1",
	}, "", "key=test")
	require.NoError(t, err)

	assert.Equal(t, string(oidcKubeconfig3), string(kubeconfig))
}

func TestBreakGlassKubeconfig(t *testing.T) {
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	logger := zaptest.NewLogger(t)
	r := kubernetes.New(st, logger, "", "", "")

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	_, err := r.BreakGlassKubeconfig(ctx, "cluster1")
	require.Error(t, err)
	require.True(t, state.IsNotFoundError(err))

	kubeconfigResource := omni.NewKubeconfig("cluster1")

	kubeconfigResource.TypedSpec().Value.Data = adminKubeconfig

	require.NoError(t, st.Create(ctx, kubeconfigResource))

	kubeconfig, err := r.BreakGlassKubeconfig(ctx, "cluster1")
	require.NoError(t, err)

	config, err := clientcmd.Load(kubeconfig)
	require.NoError(t, err)

	require.NotEmpty(t, config.Clusters)

	m1 := omni.NewClusterMachineIdentity("3")
	m2 := omni.NewClusterMachineIdentity("2")
	m3 := omni.NewClusterMachineIdentity("1")

	m1.Metadata().Labels().Set(omni.LabelCluster, "cluster1")
	m3.Metadata().Labels().Set(omni.LabelCluster, "cluster1")

	m1.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")
	m2.Metadata().Labels().Set(omni.LabelControlPlaneRole, "")

	m1.TypedSpec().Value.NodeIps = []string{"10.1.0.2"}
	m2.TypedSpec().Value.NodeIps = []string{"10.1.0.3"}
	m3.TypedSpec().Value.NodeIps = []string{"10.1.0.4"}

	require.NoError(t, st.Create(ctx, m1))
	require.NoError(t, st.Create(ctx, m2))
	require.NoError(t, st.Create(ctx, m3))

	kubeconfig, err = r.BreakGlassKubeconfig(ctx, "cluster1")
	require.NoError(t, err)

	config, err = clientcmd.Load(kubeconfig)
	require.NoError(t, err)

	require.NotEmpty(t, config.Clusters)
	require.Equal(t, "https://10.1.0.2:6443", config.Clusters["cluster1"].Server)
}
