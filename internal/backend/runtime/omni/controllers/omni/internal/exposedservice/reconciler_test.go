// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package exposedservice_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/exposedservice"
)

const (
	testProxySubdomain = "proxy"
	testClusterName    = "test-cluster"
)

func TestReconcilerAddRemove(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cluster := testClusterName
	workloadProxySubdomain := "test"
	advertisedAPIURL := "https://api.test"

	var exposedServices []*omni.ExposedService

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "test-service-1", port: "11111"},
		kubernetesService{ns: "default", name: "test-service-2", port: "22222"},
	)

	reconciler, err := exposedservice.NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL, false, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2, "expected two ExposedServices to be created")
	assert.Empty(t, exposedServices[0].TypedSpec().Value.Error)
	assert.Empty(t, exposedServices[1].TypedSpec().Value.Error)

	// remove one service
	kubernetesServices = kubernetesServices[:1]

	reconciler, err = exposedservice.NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL, false, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 1, "expected one ExposedService to remain after removal")
	assertReconcile(t, cluster, exposedServices[0], kubernetesServices[0], 11111, true)

	// add a new service
	kubernetesServices = append(kubernetesServices, makeKubernetesServices(
		kubernetesService{ns: "default", name: "test-service-3", port: "33333", prefix: "foobar", label: "Some Label"},
	)...)

	reconciler, err = exposedservice.NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL, false, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2, "expected two ExposedServices after adding a new one")

	assertReconcile(t, cluster, exposedServices[0], kubernetesServices[0], 11111, true)
	assertReconcile(t, cluster, exposedServices[1], kubernetesServices[1], 33333, true)
}

func TestReconcilerConflictResolution(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cluster := testClusterName
	workloadProxySubdomain := "test"
	advertisedAPIURL := "https://api.test"

	var exposedServices []*omni.ExposedService

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "test-service-1", port: "11111", prefix: "testprefix"},
		kubernetesService{ns: "default", name: "test-service-2", port: "11111", prefix: "testprefix"},
	)

	reconciler, err := exposedservice.NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL, false, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2, "expected two ExposedServices to be created")
	assertReconcile(t, cluster, exposedServices[0], kubernetesServices[0], 11111, true)
	assertReconcile(t, cluster, exposedServices[1], kubernetesServices[1], 11111, false)
	assert.Contains(t, exposedServices[1].TypedSpec().Value.Error, "used by another service")

	// resolve the conflict
	kubernetesServices[0].Annotations[constants.ExposedServicePrefixAnnotationKey] = "newprefix"

	reconciler, err = exposedservice.NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL, false, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2, "expected two ExposedServices to be created after conflict resolution")
	assertReconcile(t, cluster, exposedServices[0], kubernetesServices[0], 11111, true)
	assertReconcile(t, cluster, exposedServices[1], kubernetesServices[1], 11111, true)
}

func TestReconcilerUseOmniSubdomain(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cluster := testClusterName
	workloadProxySubdomain := testProxySubdomain
	advertisedAPIURL := "https://omni.example.com"

	var exposedServices []*omni.ExposedService

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "test-service-1", port: "11111", prefix: "my-grafana", label: "Grafana"},
		kubernetesService{ns: "default", name: "test-service-2", port: "22222"},
	)

	reconciler, err := exposedservice.NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL, true, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2)

	// Explicit alias with dashes should work
	assert.Empty(t, exposedServices[0].TypedSpec().Value.Error)
	assert.Equal(t, "https://my-grafana."+testProxySubdomain+".omni.example.com", exposedServices[0].TypedSpec().Value.Url)
	assert.True(t, exposedServices[0].TypedSpec().Value.HasExplicitAlias)

	// Generated alias should also work
	assert.Empty(t, exposedServices[1].TypedSpec().Value.Error)
	assert.Contains(t, exposedServices[1].TypedSpec().Value.Url, "."+testProxySubdomain+".omni.example.com")
	assert.False(t, exposedServices[1].TypedSpec().Value.HasExplicitAlias)
}

func TestReconcilerUseOmniSubdomainWithPort(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cluster := testClusterName
	workloadProxySubdomain := testProxySubdomain
	advertisedAPIURL := "https://omni.example.com:8099"

	var exposedServices []*omni.ExposedService

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "test-service-1", port: "11111", prefix: "grafana", label: "Grafana"},
	)

	reconciler, err := exposedservice.NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL, true, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 1)
	assert.Empty(t, exposedServices[0].TypedSpec().Value.Error)
	assert.Equal(t, "https://grafana."+testProxySubdomain+".omni.example.com:8099", exposedServices[0].TypedSpec().Value.Url)
}

func TestReconcilerUseOmniSubdomainEmptySubdomain(t *testing.T) {
	logger := zaptest.NewLogger(t)

	var exposedServices []*omni.ExposedService

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "test-service-1", port: "11111", prefix: "grafana", label: "Grafana"},
		kubernetesService{ns: "default", name: "test-service-2", port: "22222", prefix: "my-dashboard", label: "Dashboard"},
	)

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2)
	assert.Empty(t, exposedServices[0].TypedSpec().Value.Error)
	assert.Equal(t, "https://grafana.omni.example.com", exposedServices[0].TypedSpec().Value.Url)

	assert.Empty(t, exposedServices[1].TypedSpec().Value.Error)
	assert.Equal(t, "https://my-dashboard.omni.example.com", exposedServices[1].TypedSpec().Value.Url)
}

func TestReconcilerUseOmniSubdomainEmptySubdomainWithPort(t *testing.T) {
	logger := zaptest.NewLogger(t)

	var exposedServices []*omni.ExposedService

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "test-service-1", port: "11111", prefix: "grafana", label: "Grafana"},
	)

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com:8099", true, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 1)
	assert.Empty(t, exposedServices[0].TypedSpec().Value.Error)
	assert.Equal(t, "https://grafana.omni.example.com:8099", exposedServices[0].TypedSpec().Value.Url)
}

func TestReconcilerInvalidAliases(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name             string
		prefix           string
		expectError      string
		useOmniSubdomain bool
	}{
		{name: "dot in alias - useOmniSubdomain", prefix: "my.service", useOmniSubdomain: true, expectError: "not a valid DNS label"},
		{name: "dot in alias - default mode", prefix: "my.service", useOmniSubdomain: false, expectError: "not a valid DNS label"},
		{name: "underscore in alias - useOmniSubdomain", prefix: "my_service", useOmniSubdomain: true, expectError: "not a valid DNS label"},
		{name: "underscore in alias - default mode", prefix: "my_service", useOmniSubdomain: false, expectError: "not a valid DNS label"},
		{name: "dash in alias - default mode", prefix: "my-service", useOmniSubdomain: false, expectError: "contains a dash"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger := zaptest.NewLogger(t)

			var exposedServices []*omni.ExposedService

			kubernetesServices := makeKubernetesServices(
				kubernetesService{ns: "default", name: "test-service-1", port: "11111", prefix: tc.prefix},
			)

			reconciler, err := exposedservice.NewReconciler(testClusterName, testProxySubdomain, "https://omni.example.com", tc.useOmniSubdomain, exposedServices, kubernetesServices, logger)
			require.NoError(t, err)

			exposedServices, err = reconciler.ReconcileServices()
			require.NoError(t, err)

			require.Len(t, exposedServices, 1)
			assert.Contains(t, exposedServices[0].TypedSpec().Value.Error, tc.expectError)
		})
	}
}

func TestReconcilerMultiPort(t *testing.T) {
	logger := zaptest.NewLogger(t)

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "grafana", port: "30080,30443:8080,30444:https"},
	)

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true, nil, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err := reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 3, "expected one ExposedService per host port")

	// Resources are returned in ascending host-port order; IDs include the host port; auto-generated label disambiguates by port.
	expectedPorts := []uint32{30080, 30443, 30444}
	for i, es := range exposedServices {
		assert.Equal(t, expectedPorts[i], es.TypedSpec().Value.Port)
		assert.Equal(t, fmt.Sprintf("%s/grafana.default/%d", testClusterName, expectedPorts[i]), es.Metadata().ID())
		assert.Equal(t, fmt.Sprintf("grafana.default:%d", expectedPorts[i]), es.TypedSpec().Value.Label)
		assert.Empty(t, es.TypedSpec().Value.Error)
		assert.False(t, es.TypedSpec().Value.HasExplicitAlias)
	}
}

func TestReconcilerMultiPortPerPortAnnotations(t *testing.T) {
	logger := zaptest.NewLogger(t)

	svc := makeKubernetesServices(
		kubernetesService{ns: "default", name: "app", port: "30080,30443"},
	)[0]
	svc.Annotations[constants.ExposedServicePrefixAnnotationKey+"-30080"] = "ui"
	svc.Annotations[constants.ExposedServicePrefixAnnotationKey+"-30443"] = "api"
	svc.Annotations[constants.ExposedServiceLabelAnnotationKey+"-30080"] = "App UI"
	svc.Annotations[constants.ExposedServiceLabelAnnotationKey+"-30443"] = "App API"

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true, nil, []*corev1.Service{svc}, logger)
	require.NoError(t, err)

	exposedServices, err := reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2)

	assert.Equal(t, "App UI", exposedServices[0].TypedSpec().Value.Label)
	assert.Equal(t, "https://ui.omni.example.com", exposedServices[0].TypedSpec().Value.Url)
	assert.True(t, exposedServices[0].TypedSpec().Value.HasExplicitAlias)

	assert.Equal(t, "App API", exposedServices[1].TypedSpec().Value.Label)
	assert.Equal(t, "https://api.omni.example.com", exposedServices[1].TypedSpec().Value.Url)
	assert.True(t, exposedServices[1].TypedSpec().Value.HasExplicitAlias)
}

func TestReconcilerMultiPortPerPortFallsBackToBase(t *testing.T) {
	logger := zaptest.NewLogger(t)

	svc := makeKubernetesServices(
		kubernetesService{ns: "default", name: "app", port: "30080,30443", label: "Shared Label"},
	)[0]
	svc.Annotations[constants.ExposedServiceLabelAnnotationKey+"-30443"] = "Override For 30443"

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true, nil, []*corev1.Service{svc}, logger)
	require.NoError(t, err)

	exposedServices, err := reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2)
	assert.Equal(t, "Shared Label", exposedServices[0].TypedSpec().Value.Label, "30080 should fall back to base label")
	assert.Equal(t, "Override For 30443", exposedServices[1].TypedSpec().Value.Label)
}

func TestReconcilerMultiPortLenientPrefixCollision(t *testing.T) {
	logger := zaptest.NewLogger(t)

	svc := makeKubernetesServices(
		kubernetesService{ns: "default", name: "app", port: "30080,30443", prefix: "shared"},
	)[0]

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true, nil, []*corev1.Service{svc}, logger)
	require.NoError(t, err)

	exposedServices, err := reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2)

	// Lowest-numbered port wins the base prefix; the rest auto-generate, no error.
	assert.Empty(t, exposedServices[0].TypedSpec().Value.Error)
	assert.Equal(t, "https://shared.omni.example.com", exposedServices[0].TypedSpec().Value.Url)
	assert.True(t, exposedServices[0].TypedSpec().Value.HasExplicitAlias)

	assert.Empty(t, exposedServices[1].TypedSpec().Value.Error)
	assert.NotEqual(t, "https://shared.omni.example.com", exposedServices[1].TypedSpec().Value.Url)
	assert.False(t, exposedServices[1].TypedSpec().Value.HasExplicitAlias)
}

func TestReconcilerMultiPortBadEntriesSkipped(t *testing.T) {
	logger := zaptest.NewLogger(t)

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "app", port: "30080,not-a-port,30443:8080,99999"},
	)

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true, nil, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err := reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2, "only valid host ports should produce ExposedServices")
	assert.Equal(t, uint32(30080), exposedServices[0].TypedSpec().Value.Port)
	assert.Equal(t, uint32(30443), exposedServices[1].TypedSpec().Value.Port)
}

func TestReconcilerMultiPortServicePortPartIgnored(t *testing.T) {
	// Omni only cares about the host port; the right-hand side of ":" is consumed by
	// kube-service-exposer on the cluster and must not affect Omni's view.
	logger := zaptest.NewLogger(t)

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "app", port: "30080:http,30443:8080"},
	)

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true, nil, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err := reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2)
	assert.Equal(t, uint32(30080), exposedServices[0].TypedSpec().Value.Port)
	assert.Equal(t, uint32(30443), exposedServices[1].TypedSpec().Value.Port)
}

func TestReconcilerLegacyBareIDPreservedSinglePort(t *testing.T) {
	// A pre-existing single-port ExposedService uses the legacy "<cluster>/<svc>.<ns>" ID
	// (no host port suffix). After upgrade, a single-port reconcile must reuse that ID so
	// the existing alias and URL stay stable.
	logger := zaptest.NewLogger(t)

	legacy := omni.NewExposedService(testClusterName + "/grafana.default")
	legacy.Metadata().Labels().Set(omni.LabelCluster, testClusterName)
	legacy.Metadata().Labels().Set(omni.LabelExposedServiceAlias, "g1ar4n")
	legacy.TypedSpec().Value.Port = 30080
	legacy.TypedSpec().Value.HasExplicitAlias = false

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "grafana", port: "30080"},
	)

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true,
		[]*omni.ExposedService{legacy}, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err := reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 1)
	assert.Equal(t, testClusterName+"/grafana.default", exposedServices[0].Metadata().ID(), "legacy ID must be reused, not migrated to a suffixed one")

	alias, _ := exposedServices[0].Metadata().Labels().Get(omni.LabelExposedServiceAlias)
	assert.Equal(t, "g1ar4n", alias, "alias must be preserved across upgrade")
	assert.Equal(t, "https://g1ar4n.omni.example.com", exposedServices[0].TypedSpec().Value.Url)
}

func TestReconcilerLegacyBareIDPreservedSingleToMulti(t *testing.T) {
	// An existing single-port service goes multi-port. The legacy port keeps the bare ID
	// (and its URL); the new port gets a suffixed ID.
	logger := zaptest.NewLogger(t)

	legacy := omni.NewExposedService(testClusterName + "/api.default")
	legacy.Metadata().Labels().Set(omni.LabelCluster, testClusterName)
	legacy.Metadata().Labels().Set(omni.LabelExposedServiceAlias, "ap1xyz")
	legacy.TypedSpec().Value.Port = 30080
	legacy.TypedSpec().Value.HasExplicitAlias = false

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "api", port: "30080,30443"},
	)

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true,
		[]*omni.ExposedService{legacy}, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err := reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2)

	// 30080 keeps the legacy bare ID and its alias
	assert.Equal(t, testClusterName+"/api.default", exposedServices[0].Metadata().ID())
	assert.Equal(t, uint32(30080), exposedServices[0].TypedSpec().Value.Port)
	alias0, _ := exposedServices[0].Metadata().Labels().Get(omni.LabelExposedServiceAlias)
	assert.Equal(t, "ap1xyz", alias0, "legacy port must keep its alias")

	// 30443 gets a suffixed ID and a fresh alias
	assert.Equal(t, testClusterName+"/api.default/30443", exposedServices[1].Metadata().ID())
	assert.Equal(t, uint32(30443), exposedServices[1].TypedSpec().Value.Port)
	alias1, _ := exposedServices[1].Metadata().Labels().Get(omni.LabelExposedServiceAlias)
	assert.NotEqual(t, "ap1xyz", alias1, "new port must get its own alias")
}

func TestReconcilerLegacyBareIDDroppedWhenPortDoesNotMatch(t *testing.T) {
	// If none of the desired ports match the legacy resource's spec.port, the legacy
	// resource is not in servicesToKeep and the tracker (in the caller) will destroy it.
	// Each desired port gets a fresh suffixed ID.
	logger := zaptest.NewLogger(t)

	legacy := omni.NewExposedService(testClusterName + "/api.default")
	legacy.Metadata().Labels().Set(omni.LabelCluster, testClusterName)
	legacy.Metadata().Labels().Set(omni.LabelExposedServiceAlias, "ap1xyz")
	legacy.TypedSpec().Value.Port = 30080

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "api", port: "30443,30444"},
	)

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true,
		[]*omni.ExposedService{legacy}, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err := reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2)

	for _, es := range exposedServices {
		assert.NotEqual(t, testClusterName+"/api.default", es.Metadata().ID(), "legacy ID must not be reused when no port matches")
	}
}

func TestReconcilerInvalidAnnotationPreservesExistingResources(t *testing.T) {
	// A malformed annotation value (typo, empty, or all-invalid entries) must not tear
	// down a previously working ExposedService. The reconciler should pass the existing
	// resource through unchanged so the next valid edit reconciles it back into shape.
	logger := zaptest.NewLogger(t)

	existing := omni.NewExposedService(testClusterName + "/grafana.default")
	existing.Metadata().Labels().Set(omni.LabelCluster, testClusterName)
	existing.Metadata().Labels().Set(omni.LabelExposedServiceAlias, "g1ar4n")
	existing.TypedSpec().Value.Port = 30080
	existing.TypedSpec().Value.Url = "https://g1ar4n.omni.example.com"

	svc := makeKubernetesServices(
		kubernetesService{ns: "default", name: "grafana", port: "not-a-port"},
	)

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true,
		[]*omni.ExposedService{existing}, svc, logger)
	require.NoError(t, err)

	exposedServices, err := reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 1, "existing resource must be preserved when annotation is unparseable")
	assert.Equal(t, testClusterName+"/grafana.default", exposedServices[0].Metadata().ID())
	alias, _ := exposedServices[0].Metadata().Labels().Get(omni.LabelExposedServiceAlias)
	assert.Equal(t, "g1ar4n", alias)
	assert.Equal(t, uint32(30080), exposedServices[0].TypedSpec().Value.Port)
	assert.Equal(t, "https://g1ar4n.omni.example.com", exposedServices[0].TypedSpec().Value.Url)
}

func TestReconcilerSinglePortUnchangedDefaultLabel(t *testing.T) {
	// A single-port service keeps the historical default label of "<name>.<ns>" without
	// the port suffix, so existing UIs stay readable.
	logger := zaptest.NewLogger(t)

	kubernetesServices := makeKubernetesServices(
		kubernetesService{ns: "default", name: "grafana", port: "30080"},
	)

	reconciler, err := exposedservice.NewReconciler(testClusterName, "", "https://omni.example.com", true, nil, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err := reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 1)
	assert.Equal(t, "grafana.default", exposedServices[0].TypedSpec().Value.Label)
}

//nolint:unparam
func assertReconcile(t *testing.T, cluster string, svc *omni.ExposedService, kubernetesSvc *corev1.Service, hostPort int, success bool) {
	t.Helper()

	expectedID := fmt.Sprintf("%s/%s.%s/%d", cluster, kubernetesSvc.Name, kubernetesSvc.Namespace, hostPort)

	assert.Equal(t, expectedID, svc.Metadata().ID())

	if !success {
		assert.NotEmpty(t, svc.TypedSpec().Value.Error)

		return
	}

	assert.Empty(t, svc.TypedSpec().Value.Error)
	assert.Equal(t, strconv.Itoa(hostPort), strconv.Itoa(int(svc.TypedSpec().Value.Port)))

	prefix, ok := kubernetesSvc.Annotations[constants.ExposedServicePrefixAnnotationKey]
	assert.Equal(t, ok, svc.TypedSpec().Value.HasExplicitAlias)

	if ok {
		assert.Contains(t, svc.TypedSpec().Value.Url, prefix)
		assert.True(t, svc.TypedSpec().Value.HasExplicitAlias)
	} else {
		assert.False(t, svc.TypedSpec().Value.HasExplicitAlias)
	}

	label := kubernetesSvc.Annotations[constants.ExposedServiceLabelAnnotationKey]
	if label == "" {
		label = kubernetesSvc.Name + "." + kubernetesSvc.Namespace
	}

	assert.Equal(t, label, svc.TypedSpec().Value.Label)
}

type kubernetesService struct {
	ns, name, port, prefix, label string
}

func makeKubernetesServices(kubernetesServices ...kubernetesService) []*corev1.Service {
	services := make([]*corev1.Service, 0, len(kubernetesServices))

	for _, s := range kubernetesServices {
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: s.ns,
				Name:      s.name,
				Annotations: map[string]string{
					constants.ExposedServicePortAnnotationKey: s.port,
				},
			},
		}

		if s.prefix != "" {
			service.Annotations[constants.ExposedServicePrefixAnnotationKey] = s.prefix
		}

		if s.label != "" {
			service.Annotations[constants.ExposedServiceLabelAnnotationKey] = s.label
		}

		services = append(services, &service)
	}

	return services
}
