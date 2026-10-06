// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package kubernetes_test

import (
	"context"
	_ "embed"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/omni/client/api/common"
	omnikubeconfig "github.com/siderolabs/omni/client/pkg/kubeconfig"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
)

//go:embed testdata/oidc-kubeconfig1.yaml
var oidcKubeconfig1 []byte

//go:embed testdata/oidc-kubeconfig2.yaml
var oidcKubeconfig2 []byte

//go:embed testdata/oidc-kubeconfig3.yaml
var oidcKubeconfig3 []byte

//go:embed testdata/oidc-kubeconfig4.yaml
var oidcKubeconfig4 []byte

//go:embed testdata/oidc-kubeconfig5.yaml
var oidcKubeconfig5 []byte

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
	assertValidKubeconfig(t, kubeconfig)

	kubeconfig, err = r.GetOIDCKubeconfig(&common.Context{
		Name: "cluster1",
	}, "")
	require.NoError(t, err)

	assert.Equal(t, string(oidcKubeconfig2), string(kubeconfig))
	assertValidKubeconfig(t, kubeconfig)
}

func TestOIDCKubeconfigWithExtraOptions(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := kubernetes.New(nil, logger, "http://localhost:8080/oidc", "default", "https://localhost:8095")

	kubeconfig, err := r.GetOIDCKubeconfig(&common.Context{
		Name: "cluster1",
	}, "test@example.com")
	require.NoError(t, err)

	assert.Equal(t, string(oidcKubeconfig1), string(kubeconfig))
	assertValidKubeconfig(t, kubeconfig)

	kubeconfig, err = r.GetOIDCKubeconfig(&common.Context{
		Name: "cluster1",
	}, "", "key=test")
	require.NoError(t, err)

	// not validated on purpose: the management API never passes such an option, so the client rejects it
	assert.Equal(t, string(oidcKubeconfig3), string(kubeconfig))

	// the full set of options the management API can pass, in the order it passes them
	kubeconfig, err = r.GetOIDCKubeconfig(&common.Context{
		Name: "cluster1",
	}, "test@example.com", "grant-type=auto", "oidc-redirect-url=urn:ietf:wg:oauth:2.0:oob", "token-cache-dir=/tmp/oidc-cache")
	require.NoError(t, err)
	assertValidKubeconfig(t, kubeconfig)
}

func TestOIDCKubeconfigWithCacheDir(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := kubernetes.New(nil, logger, "http://localhost:8080/oidc", "default", "https://localhost:8095")

	kubeconfig, err := r.GetOIDCKubeconfig(&common.Context{
		Name: "cluster1",
	}, "test@example.com", "token-cache-dir=/tmp/oidc-cache")
	require.NoError(t, err)

	assert.Equal(t, string(oidcKubeconfig4), string(kubeconfig))
	assertValidKubeconfig(t, kubeconfig)
}

func TestOIDCKubeconfigWithCacheIsolation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := kubernetes.New(nil, logger, "http://localhost:8080/oidc", "default", "https://localhost:8095")

	kubeconfig, err := r.GetOIDCKubeconfig(&common.Context{
		Name: "cluster1",
	}, "test@example.com", "token-cache-dir=~/.kube/cache/oidc-login/default-cluster1-test@example.com")
	require.NoError(t, err)

	assert.Equal(t, string(oidcKubeconfig5), string(kubeconfig))
	assertValidKubeconfig(t, kubeconfig)
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
	assertValidKubeconfig(t, kubeconfig)

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
	assertValidKubeconfig(t, kubeconfig)

	config, err = clientcmd.Load(kubeconfig)
	require.NoError(t, err)

	require.NotEmpty(t, config.Clusters)
	require.Equal(t, "https://10.1.0.2:6443", config.Clusters["cluster1"].Server)
}

// assertValidKubeconfig makes sure whatever Omni generates passes the validation the client applies to it.
func assertValidKubeconfig(t *testing.T, data []byte) {
	t.Helper()

	require.NoError(t, omnikubeconfig.Validate(data))
}
