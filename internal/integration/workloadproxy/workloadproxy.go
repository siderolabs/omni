// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package workloadproxy provides tests for the workload proxy (exposed services) functionality in Omni.
package workloadproxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgokubernetes "k8s.io/client-go/kubernetes"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/services/workloadproxy"
	"github.com/siderolabs/omni/internal/integration/kubernetes"
)

type serviceContext struct {
	res        *omni.ExposedService
	svc        *corev1.Service
	deployment *deploymentContext
}

type deploymentContext struct {
	deployment *appsv1.Deployment
	cluster    *clusterContext
	services   []serviceContext
}

type clusterContext struct {
	kubeClient  clientgokubernetes.Interface
	clusterID   string
	deployments []deploymentContext
}

//go:embed testdata/sidero-labs-icon.svg
var sideroLabsIconSVG []byte

// TestOptions configures the scale of the workload proxy test.
type TestOptions struct {
	NumWorkloads           int
	NumReplicasPerWorkload int
	NumServicesPerWorkload int
}

// Test tests the exposed services functionality in Omni.
//
//nolint:prealloc
func Test(ctx context.Context, t *testing.T, omniClient *client.Client, serviceAccountKey string, opts TestOptions, clusterIDs ...string) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	t.Cleanup(cancel)

	if len(clusterIDs) == 0 {
		require.Fail(t, "no cluster IDs provided for the test, please provide at least one cluster ID")
	}

	sa, err := serviceaccount.Decode(serviceAccountKey)
	require.NoError(t, err)

	ctx = kubernetes.WrapContext(ctx, t)
	logger := zaptest.NewLogger(t)

	clusters := make([]clusterContext, 0, len(clusterIDs))

	for _, clusterID := range clusterIDs {
		cluster := prepareServices(ctx, t, logger, omniClient, clusterID, opts)

		clusters = append(clusters, cluster)
	}

	var (
		allServices            []serviceContext       //nolint:prealloc
		allExposedServices     []*omni.ExposedService //nolint:prealloc
		deploymentsToScaleDown []deploymentContext
	)

	for _, cluster := range clusters {
		for i, deployment := range cluster.deployments {
			allServices = append(allServices, deployment.services...)

			for _, service := range deployment.services {
				allExposedServices = append(allExposedServices, service.res)
			}

			if i%2 == 0 { // scale down every second deployment
				deploymentsToScaleDown = append(deploymentsToScaleDown, deployment)
			}
		}
	}

	rand.Shuffle(len(allExposedServices), func(i, j int) {
		allExposedServices[i], allExposedServices[j] = allExposedServices[j], allExposedServices[i]
	})

	testAccess(ctx, t, logger, sa.Key, allExposedServices, http.StatusOK)

	inaccessibleExposedServices := make([]*omni.ExposedService, 0, len(allExposedServices))

	for _, deployment := range deploymentsToScaleDown {
		logger.Info("scale deployment down to zero replicas", zap.String("deployment", deployment.deployment.Name), zap.String("clusterID", deployment.cluster.clusterID))

		kubernetes.ScaleDeployment(ctx, t, deployment.cluster.kubeClient, deployment.deployment.Namespace, deployment.deployment.Name, 0)

		for _, service := range deployment.services {
			inaccessibleExposedServices = append(inaccessibleExposedServices, service.res)
		}
	}

	testAccess(ctx, t, logger, sa.Key, inaccessibleExposedServices, http.StatusBadGateway)

	for _, deployment := range deploymentsToScaleDown {
		logger.Info("scale deployment back up", zap.String("deployment", deployment.deployment.Name), zap.String("clusterID", deployment.cluster.clusterID))

		kubernetes.ScaleDeployment(ctx, t, deployment.cluster.kubeClient, deployment.deployment.Namespace, deployment.deployment.Name, 1)
	}

	testAccess(ctx, t, logger, sa.Key, allExposedServices, http.StatusOK)
	testToggleFeature(ctx, t, logger, omniClient, sa.Key, clusters[0])
	testToggleKubernetesServiceAnnotation(ctx, t, logger, omniClient, sa.Key, allServices[:len(allServices)/2])
}

// testToggleFeature tests toggling off/on the workload proxy feature for a cluster.
func testToggleFeature(ctx context.Context, t *testing.T, logger *zap.Logger, omniClient *client.Client, saKey *pgp.Key, cluster clusterContext) {
	logger.Info("test turning off and on the feature for the cluster", zap.String("clusterID", cluster.clusterID))

	setFeatureToggle := func(enabled bool) {
		_, err := safe.StateUpdateWithConflicts(ctx, omniClient.Omni().State(), omni.NewCluster(cluster.clusterID).Metadata(), func(res *omni.Cluster) error {
			res.TypedSpec().Value.Features.EnableWorkloadProxy = enabled

			return nil
		})
		require.NoErrorf(t, err, "failed to turn off workload proxy feature for cluster %q", cluster.clusterID)
	}

	setFeatureToggle(false)

	var services []*omni.ExposedService

	for _, deployment := range cluster.deployments {
		for _, service := range deployment.services {
			services = append(services, service.res)
		}
	}

	if len(services) > 4 {
		services = services[:4]
	}

	testAccess(ctx, t, logger, saKey, services, http.StatusNotFound)

	setFeatureToggle(true)

	testAccess(ctx, t, logger, saKey, services[:4], http.StatusOK)
}

func testToggleKubernetesServiceAnnotation(ctx context.Context, t *testing.T, logger *zap.Logger, omniClient *client.Client, saKey *pgp.Key, services []serviceContext) {
	logger.Info("test toggling Kubernetes service annotation for exposed services", zap.Int("numServices", len(services)))

	for _, service := range services {
		kubernetes.UpdateService(ctx, t, service.deployment.cluster.kubeClient, service.svc.Namespace, service.svc.Name, func(svc *corev1.Service) {
			delete(svc.Annotations, constants.ExposedServicePortAnnotationKey)
		})
	}

	omniState := omniClient.Omni().State()
	for _, service := range services {
		rtestutils.AssertNoResource[*omni.ExposedService](ctx, t, omniState, service.res.Metadata().ID())
	}

	exposedServices := xslices.Map(services, func(svc serviceContext) *omni.ExposedService { return svc.res })

	testAccess(ctx, t, logger, saKey, exposedServices, http.StatusNotFound)

	for _, service := range services {
		kubernetes.UpdateService(ctx, t, service.deployment.cluster.kubeClient, service.svc.Namespace, service.svc.Name, func(svc *corev1.Service) {
			svc.Annotations[constants.ExposedServicePortAnnotationKey] = service.svc.Annotations[constants.ExposedServicePortAnnotationKey]
		})
	}

	updatedServicesMap := make(map[resource.ID]*omni.ExposedService)

	for _, service := range services {
		rtestutils.AssertResource[*omni.ExposedService](ctx, t, omniState, service.res.Metadata().ID(), func(r *omni.ExposedService, assertion *assert.Assertions) {
			assertion.Equal(service.res.TypedSpec().Value.Port, r.TypedSpec().Value.Port, "exposed service port should be restored after toggling the annotation back on")
			assertion.Equal(service.res.TypedSpec().Value.Label, r.TypedSpec().Value.Label, "exposed service label should be restored after toggling the annotation back on")
			assertion.Equal(service.res.TypedSpec().Value.IconBase64, r.TypedSpec().Value.IconBase64, "exposed service icon should be restored after toggling the annotation back on")
			assertion.Equal(service.res.TypedSpec().Value.HasExplicitAlias, r.TypedSpec().Value.HasExplicitAlias, "exposed service has explicit alias")
			assertion.Empty(r.TypedSpec().Value.Error, "exposed service should not have an error after toggling the annotation back on")

			if r.TypedSpec().Value.HasExplicitAlias {
				assertion.Equal(service.res.TypedSpec().Value.Url, r.TypedSpec().Value.Url, "exposed service URL should be restored after toggling the annotation back on")
			}

			updatedServicesMap[r.Metadata().ID()] = r
		})
	}

	updatedServices := maps.Values(updatedServicesMap)

	testAccess(ctx, t, logger, saKey, updatedServices, http.StatusOK)
}

func prepareServices(ctx context.Context, t *testing.T, logger *zap.Logger, omniClient *client.Client, clusterID string, opts TestOptions) clusterContext {
	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	t.Cleanup(cancel)

	omniState := omniClient.Omni().State()
	kubeClient := kubernetes.GetClient(ctx, t, omniClient.Management(), clusterID)

	iconBase64 := base64.StdEncoding.EncodeToString(doGzip(t, sideroLabsIconSVG)) // base64(gzip(Sidero Labs icon SVG))
	expectedIconBase64 := base64.StdEncoding.EncodeToString(sideroLabsIconSVG)

	numWorkloads := opts.NumWorkloads
	numReplicasPerWorkload := opts.NumReplicasPerWorkload
	numServicePerWorkload := opts.NumServicesPerWorkload
	startPort := 12345

	cluster := clusterContext{
		clusterID:   clusterID,
		deployments: make([]deploymentContext, 0, numWorkloads),
		kubeClient:  kubeClient,
	}

	for i := range numWorkloads {
		identifier := fmt.Sprintf("%s-w%02d", clusterID, i)

		deployment := deploymentContext{
			cluster: &cluster,
		}

		firstPort := startPort + i*numServicePerWorkload

		var services []*corev1.Service

		deployment.deployment, services = createKubernetesResources(ctx, t, logger, kubeClient, firstPort, numReplicasPerWorkload, numServicePerWorkload, identifier, iconBase64)

		for _, service := range services {
			expectedID := clusterID + "/" + service.Name + "." + service.Namespace

			expectedPort, err := strconv.Atoi(service.Annotations[constants.ExposedServicePortAnnotationKey])
			require.NoError(t, err)

			expectedLabel := service.Annotations[constants.ExposedServiceLabelAnnotationKey]
			explicitAlias, hasExplicitAlias := service.Annotations[constants.ExposedServicePrefixAnnotationKey]

			var res *omni.ExposedService

			rtestutils.AssertResource[*omni.ExposedService](ctx, t, omniState, expectedID, func(r *omni.ExposedService, assertion *assert.Assertions) {
				assertion.Equal(expectedPort, int(r.TypedSpec().Value.Port))
				assertion.Equal(expectedLabel, r.TypedSpec().Value.Label)
				assertion.Equal(expectedIconBase64, r.TypedSpec().Value.IconBase64)
				assertion.NotEmpty(r.TypedSpec().Value.Url)
				assertion.Empty(r.TypedSpec().Value.Error)
				assertion.Equal(hasExplicitAlias, r.TypedSpec().Value.HasExplicitAlias)

				if hasExplicitAlias {
					assertion.Contains(r.TypedSpec().Value.Url, explicitAlias+"-")
				}

				res = r
			})

			deployment.services = append(deployment.services, serviceContext{
				res:        res,
				svc:        service,
				deployment: &deployment,
			})
		}

		cluster.deployments = append(cluster.deployments, deployment)
	}

	return cluster
}

func testAccess(ctx context.Context, t *testing.T, logger *zap.Logger, saKey *pgp.Key, exposedServices []*omni.ExposedService, expectedStatusCode int) {
	keyID := saKey.Fingerprint()

	signedIDBytes, err := saKey.Sign([]byte(keyID))
	require.NoError(t, err)

	keyIDSignatureBase64 := base64.StdEncoding.EncodeToString(signedIDBytes)

	logger.Debug("using SA key for workload proxy", zap.String("keyID", keyID), zap.String("keyIDSignatureBase64", keyIDSignatureBase64))

	cookies := []*http.Cookie{
		{Name: workloadproxy.PublicKeyIDCookie, Value: keyID},
		{Name: workloadproxy.PublicKeyIDSignatureBase64Cookie, Value: keyIDSignatureBase64},
	}

	clientTransport := cleanhttp.DefaultTransport()
	clientTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	httpClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse // disable follow redirects
		},
		Timeout:   30 * time.Second,
		Transport: clientTransport,
	}
	t.Cleanup(httpClient.CloseIdleConnections)

	parallelTestBatchSize := 8

	var wg sync.WaitGroup

	errs := make([]error, parallelTestBatchSize)

	for i, exposedService := range exposedServices {
		logger.Info("test exposed service",
			zap.String("id", exposedService.Metadata().ID()),
			zap.String("url", exposedService.TypedSpec().Value.Url),
			zap.Int("expectedStatusCode", expectedStatusCode),
		)

		wg.Go(func() {
			if testErr := testAccessParallel(ctx, httpClient, exposedService, expectedStatusCode, cookies...); testErr != nil {
				errs[i%parallelTestBatchSize] = fmt.Errorf("failed to access exposed service %q over %q [%d]: %w",
					exposedService.Metadata().ID(), exposedService.TypedSpec().Value.Url, i, testErr)
			}
		})

		if i == len(exposedServices)-1 || ((i+1)%parallelTestBatchSize == 0) {
			logger.Info("wait for the batch of exposed service tests to finish", zap.Int("batchSize", (i%parallelTestBatchSize)+1))

			wg.Wait()

			for _, testErr := range errs {
				assert.NoError(t, testErr)
			}

			// reset the errors for the next batch
			for j := range errs {
				errs[j] = nil
			}
		}
	}
}

func testAccessParallel(ctx context.Context, httpClient *http.Client, exposedService *omni.ExposedService, expectedStatusCode int, cookies ...*http.Cookie) error {
	svcURL := exposedService.TypedSpec().Value.Url

	// test redirect to the authentication page when cookies are not set
	if err := testAccessSingle(ctx, httpClient, svcURL, http.StatusSeeOther, ""); err != nil {
		return fmt.Errorf("failed to access exposed service %q over %q: %w", exposedService.Metadata().ID(), svcURL, err)
	}

	expectedBodyContent := ""
	reqPerExposedService := 128
	numRetries := 9
	label := exposedService.TypedSpec().Value.Label

	if expectedStatusCode == http.StatusOK {
		expectedBodyContent = label[:strings.LastIndex(label, "-")] // for the label "integration-workload-proxy-2-w03-s01", the content must be "integration-workload-proxy-2-w03"
	}

	var wg sync.WaitGroup

	wg.Add(reqPerExposedService)

	lock := sync.Mutex{}
	svcErrs := make(map[string]error, reqPerExposedService)

	for range reqPerExposedService {
		go func() {
			defer wg.Done()

			if err := testAccessSingleWithRetries(ctx, httpClient, svcURL, expectedStatusCode, expectedBodyContent, numRetries, cookies...); err != nil {
				lock.Lock()

				svcErrs[err.Error()] = err

				lock.Unlock()
			}
		}()
	}

	wg.Wait()

	return errors.Join(maps.Values(svcErrs)...)
}

func testAccessSingleWithRetries(ctx context.Context, httpClient *http.Client, svcURL string, expectedStatusCode int, expectedBodyContent string, retries int, cookies ...*http.Cookie) error {
	if retries <= 0 {
		return fmt.Errorf("retries must be greater than 0, got %d", retries)
	}

	var err error

	for range retries {
		if err = testAccessSingle(ctx, httpClient, svcURL, expectedStatusCode, expectedBodyContent, cookies...); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while trying to access %q: %w", svcURL, ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}

	return fmt.Errorf("failed to access %q after %d retries: %w", svcURL, retries, err)
}

func testAccessSingle(ctx context.Context, httpClient *http.Client, svcURL string, expectedStatusCode int, expectedBodyContent string, cookies ...*http.Cookie) error {
	req, err := prepareRequest(ctx, svcURL)
	if err != nil {
		return fmt.Errorf("failed to prepare request: %w", err)
	}

	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do request: %w", err)
	}

	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	bodyStr := string(body)

	if resp.StatusCode != expectedStatusCode {
		return fmt.Errorf("unexpected status code: expected %d, got %d, body: %q", expectedStatusCode, resp.StatusCode, bodyStr)
	}

	if expectedBodyContent == "" {
		return nil
	}

	if !strings.Contains(string(body), expectedBodyContent) {
		return fmt.Errorf("content %q not found in body: %q", expectedBodyContent, bodyStr)
	}

	return nil
}

// prepareRequest prepares a request to the workload proxy.
// It uses the Omni base URL to access Omni with proper name resolution, but set the Host header to the original URL to test the workload proxy logic
//
// sample svcURL: https://j2s7hf-local.proxy-us.localhost:8099/
func prepareRequest(ctx context.Context, svcURL string) (*http.Request, error) {
	parsedURL, err := url.Parse(svcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL %q: %w", svcURL, err)
	}

	hostParts := strings.SplitN(parsedURL.Host, ".", 3)
	if len(hostParts) < 3 {
		return nil, fmt.Errorf("failed to parse host %q: expected at least 3 parts", parsedURL.Host)
	}

	svcHost := parsedURL.Host
	baseHost := hostParts[2]

	parsedURL.Host = baseHost

	baseURL := parsedURL.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Host = svcHost

	return req, nil
}

func createKubernetesResources(ctx context.Context, t *testing.T, logger *zap.Logger, kubeClient clientgokubernetes.Interface,
	firstPort, numReplicas, numServices int, identifier, icon string,
) (*appsv1.Deployment, []*corev1.Service) {
	namespace := "default"

	_, err := kubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      identifier,
			Namespace: namespace,
		},
		Data: map[string]string{
			"index.html": fmt.Sprintf("<!doctype html><meta charset=utf-8><title>%s</title><body>%s</body>", identifier, identifier),
		},
	}, metav1.CreateOptions{})
	if !apierrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      identifier,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(numReplicas)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": identifier,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": identifier,
					},
				},
				Spec: corev1.PodSpec{
					Tolerations: []corev1.Toleration{
						{
							Operator: corev1.TolerationOpExists,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  identifier,
							Image: "nginx:stable-alpine-slim",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "contents",
									MountPath: "/usr/share/nginx/html",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "contents",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: identifier,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if _, err = kubeClient.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			require.NoErrorf(t, err, "failed to create deployment %q in namespace %q", identifier, namespace)
		}

		// if the deployment already exists, update it with the new spec
		deployment, err = kubeClient.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
		if !apierrors.IsNotFound(err) {
			require.NoError(t, err)
		}
	}

	services := make([]*corev1.Service, 0, numServices)

	for i := range numServices {
		svcIdentifier := fmt.Sprintf("%s-s%02d", identifier, i)
		port := firstPort + i

		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      svcIdentifier,
				Namespace: namespace,
				Annotations: map[string]string{
					constants.ExposedServicePortAnnotationKey:  strconv.Itoa(port),
					constants.ExposedServiceLabelAnnotationKey: svcIdentifier,
					constants.ExposedServiceIconAnnotationKey:  icon,
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": identifier,
				},
				Ports: []corev1.ServicePort{
					{
						Port:       80,
						TargetPort: intstr.FromInt32(80),
					},
				},
			},
		}

		if usePrefix := i%2 == 0; usePrefix {
			service.Annotations[constants.ExposedServicePrefixAnnotationKey] = strings.ReplaceAll(svcIdentifier, "-", "")
		}

		services = append(services, &service)

		_, err = kubeClient.CoreV1().Services(namespace).Create(ctx, &service, metav1.CreateOptions{})
		if !apierrors.IsAlreadyExists(err) {
			require.NoError(t, err)
		}
	}

	// assert that all pods of the deployment are ready
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		dep, depErr := kubeClient.AppsV1().Deployments(namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
		if errors.Is(depErr, context.Canceled) || errors.Is(depErr, context.DeadlineExceeded) {
			require.NoError(collect, depErr)
		}

		if !assert.NoError(collect, depErr) {
			logger.Error("failed to get deployment", zap.String("namespace", deployment.Namespace), zap.String("name", deployment.Name), zap.Error(depErr))

			return
		}

		if !assert.Equal(collect, int(dep.Status.ReadyReplicas), numReplicas) {
			logger.Debug("deployment does not have expected number of replicas",
				zap.String("namespace", deployment.Namespace),
				zap.String("name", deployment.Name),
				zap.Int("expected", numReplicas),
				zap.Int("found", int(dep.Status.ReadyReplicas)),
			)
		}
	}, 3*time.Minute, 5*time.Second)

	return deployment, services
}

func doGzip(t *testing.T, input []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	writer := gzip.NewWriter(&buf)

	_, err := writer.Write(input)
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	return buf.Bytes()
}
