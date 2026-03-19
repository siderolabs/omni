// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package exposedservice_test

import (
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
		kubernetesService{"default", "test-service-1", "11111", "", ""},
		kubernetesService{"default", "test-service-2", "22222", "", ""},
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
	assertReconcile(t, cluster, exposedServices[0], kubernetesServices[0], true)

	// add a new service
	kubernetesServices = append(kubernetesServices, makeKubernetesServices(kubernetesService{"default", "test-service-3", "33333", "foobar", "Some Label"})...)

	reconciler, err = exposedservice.NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL, false, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2, "expected two ExposedServices after adding a new one")

	assertReconcile(t, cluster, exposedServices[0], kubernetesServices[0], true)
	assertReconcile(t, cluster, exposedServices[1], kubernetesServices[1], true)
}

func TestReconcilerConflictResolution(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cluster := testClusterName
	workloadProxySubdomain := "test"
	advertisedAPIURL := "https://api.test"

	var exposedServices []*omni.ExposedService

	kubernetesServices := makeKubernetesServices(
		kubernetesService{"default", "test-service-1", "11111", "testprefix", ""},
		kubernetesService{"default", "test-service-2", "11111", "testprefix", ""},
	)

	reconciler, err := exposedservice.NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL, false, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2, "expected two ExposedServices to be created")
	assertReconcile(t, cluster, exposedServices[0], kubernetesServices[0], true)
	assertReconcile(t, cluster, exposedServices[1], kubernetesServices[1], false)
	assert.Contains(t, exposedServices[1].TypedSpec().Value.Error, "used by another service")

	// resolve the conflict
	kubernetesServices[0].Annotations[constants.ExposedServicePrefixAnnotationKey] = "newprefix"

	reconciler, err = exposedservice.NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL, false, exposedServices, kubernetesServices, logger)
	require.NoError(t, err)

	exposedServices, err = reconciler.ReconcileServices()
	require.NoError(t, err)

	require.Len(t, exposedServices, 2, "expected two ExposedServices to be created after conflict resolution")
	assertReconcile(t, cluster, exposedServices[0], kubernetesServices[0], true)
	assertReconcile(t, cluster, exposedServices[1], kubernetesServices[1], true)
}

func TestReconcilerUseOmniSubdomain(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cluster := testClusterName
	workloadProxySubdomain := testProxySubdomain
	advertisedAPIURL := "https://omni.example.com"

	var exposedServices []*omni.ExposedService

	kubernetesServices := makeKubernetesServices(
		kubernetesService{"default", "test-service-1", "11111", "my-grafana", "Grafana"},
		kubernetesService{"default", "test-service-2", "22222", "", ""},
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
		kubernetesService{"default", "test-service-1", "11111", "grafana", "Grafana"},
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
		kubernetesService{"default", "test-service-1", "11111", "grafana", "Grafana"},
		kubernetesService{"default", "test-service-2", "22222", "my-dashboard", "Dashboard"},
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
		kubernetesService{"default", "test-service-1", "11111", "grafana", "Grafana"},
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
				kubernetesService{"default", "test-service-1", "11111", tc.prefix, ""},
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

//nolint:unparam
func assertReconcile(t *testing.T, cluster string, svc *omni.ExposedService, kubernetesSvc *corev1.Service, success bool) {
	t.Helper()

	expectedID := cluster + "/" + kubernetesSvc.Name + "." + kubernetesSvc.Namespace

	assert.Equal(t, expectedID, svc.Metadata().ID())

	if !success {
		assert.NotEmpty(t, svc.TypedSpec().Value.Error)

		return
	}

	assert.Empty(t, svc.TypedSpec().Value.Error)
	assert.Equal(t, kubernetesSvc.Annotations[constants.ExposedServicePortAnnotationKey], strconv.Itoa(int(svc.TypedSpec().Value.Port)))

	prefix, ok := kubernetesSvc.Annotations[constants.ExposedServicePrefixAnnotationKey]
	assert.Equal(t, ok, svc.TypedSpec().Value.HasExplicitAlias)

	if ok {
		assert.Contains(t, svc.TypedSpec().Value.Url, prefix)
	}

	if ok {
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
