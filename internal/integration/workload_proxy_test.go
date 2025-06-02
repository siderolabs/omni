// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/workloadproxy"
	"github.com/siderolabs/omni/internal/pkg/clientconfig"
)

//go:embed testdata/sidero-labs-icon.svg
var sideroLabsIconSVG []byte

// AssertWorkloadProxy tests the workload proxy feature.
func AssertWorkloadProxy(testCtx context.Context, apiClient *client.Client, clusterName string) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 150*time.Second)
		t.Cleanup(cancel)

		st := apiClient.Omni().State()
		kubeClient := getKubernetesClient(ctx, t, apiClient.Management(), clusterName)
		port := 12345
		label := "test-nginx"
		icon := base64.StdEncoding.EncodeToString(doGzip(t, sideroLabsIconSVG)) // base64(gzip(Sidero Labs icon SVG))
		deployment, service := workloadProxyManifests(port, label, icon)
		deps := kubeClient.AppsV1().Deployments(deployment.Namespace)

		_, err := deps.Create(ctx, &deployment, metav1.CreateOptions{})
		if !apierrors.IsAlreadyExists(err) {
			require.NoError(t, err)
		}

		// assert that deployment has a running (Ready) pod
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			dep, depErr := deps.Get(ctx, deployment.Name, metav1.GetOptions{})
			if errors.Is(depErr, context.Canceled) || errors.Is(depErr, context.DeadlineExceeded) {
				require.NoError(collect, depErr)
			}

			if !assert.NoError(collect, depErr) {
				t.Logf("failed to get deployment %q/%q: %q", deployment.Namespace, deployment.Name, depErr)

				return
			}

			if !assert.Greater(collect, dep.Status.ReadyReplicas, int32(0)) {
				t.Logf("deployment %q/%q has no ready replicas", dep.Namespace, dep.Name)
			}
		}, 2*time.Minute, 1*time.Second)

		_, err = kubeClient.CoreV1().Services(service.Namespace).Create(ctx, &service, metav1.CreateOptions{})
		if !apierrors.IsAlreadyExists(err) {
			require.NoError(t, err)
		}

		expectedID := clusterName + "/" + service.Name + "." + service.Namespace
		expectedIconBase64 := base64.StdEncoding.EncodeToString(sideroLabsIconSVG)

		var svcURL string

		rtestutils.AssertResource[*omni.ExposedService](ctx, t, st, expectedID, func(r *omni.ExposedService, assertion *assert.Assertions) {
			assertion.Equal(port, int(r.TypedSpec().Value.Port))
			assertion.Equal("test-nginx", r.TypedSpec().Value.Label)
			assertion.Equal(expectedIconBase64, r.TypedSpec().Value.IconBase64)
			assertion.NotEmpty(r.TypedSpec().Value.Url)

			svcURL = r.TypedSpec().Value.Url
		})

		clientTransport := cleanhttp.DefaultPooledTransport()
		clientTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		httpClient := &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse // disable follow redirects
			},
			Timeout:   5 * time.Second,
			Transport: clientTransport,
		}
		t.Cleanup(httpClient.CloseIdleConnections)

		t.Logf("do a GET request to the exposed service URL, expect a redirect to authentication page")

		doRequestAssertResponseWithRetries(ctx, t, httpClient, svcURL, http.StatusSeeOther, "", "")

		t.Logf("do the same request but with proper cookies set, expect Omni to proxy the request to nginx")

		keyID, keyIDSignatureBase64, err := clientconfig.RegisterKeyGetIDSignatureBase64(ctx, apiClient)
		require.NoError(t, err)

		t.Logf("public key ID: %q, signature base64: %q", keyID, keyIDSignatureBase64)

		cookies := []*http.Cookie{
			{Name: workloadproxy.PublicKeyIDCookie, Value: keyID},
			{Name: workloadproxy.PublicKeyIDSignatureBase64Cookie, Value: keyIDSignatureBase64},
		}

		doRequestAssertResponseWithRetries(ctx, t, httpClient, svcURL, http.StatusOK, "Welcome to nginx!", "", cookies...)

		// Remove the service, assert that ExposedService resource is removed as well
		require.NoError(t, kubeClient.CoreV1().Services(service.Namespace).Delete(ctx, service.Name, metav1.DeleteOptions{}))

		rtestutils.AssertNoResource[*omni.ExposedService](ctx, t, st, expectedID)

		t.Logf("do a GET request to the exposed service URL, expect 404 Not Found")

		doRequestAssertResponseWithRetries(ctx, t, httpClient, svcURL, http.StatusNotFound, "", "", cookies...)
	}
}

func doRequestAssertResponseWithRetries(ctx context.Context, t *testing.T, httpClient *http.Client, svcURL string, expectedStatusCode int,
	expectedBodyContent, expectedLocationHeaderContent string, cookies ...*http.Cookie,
) {
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		req := prepareWorkloadProxyRequest(ctx, t, svcURL)

		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}

		resp, err := httpClient.Do(req)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			require.NoError(collect, err)
		}

		if !assert.NoError(collect, err) {
			t.Logf("request failed: %v", err)

			return
		}

		defer resp.Body.Close() //nolint:errcheck

		if !assert.Equal(collect, expectedStatusCode, resp.StatusCode) {
			t.Logf("unexpected status code: not %d, but %d", expectedStatusCode, resp.StatusCode)
		}

		if expectedBodyContent != "" {
			body, bodyErr := io.ReadAll(resp.Body)
			require.NoError(collect, bodyErr)

			if !assert.Contains(collect, string(body), expectedBodyContent) {
				t.Logf("content %q not found in body: %q", expectedBodyContent, string(body))
			}
		}

		if expectedLocationHeaderContent != "" {
			location := resp.Header.Get("Location")

			if !assert.Contains(collect, location, expectedLocationHeaderContent) {
				t.Logf("content %q not found in Location header: %q", expectedLocationHeaderContent, location)
			}
		}
	}, 3*time.Minute, 3*time.Second)
}

// prepareWorkloadProxyRequest prepares a request to the workload proxy.
// It uses the Omni base URL to access Omni with proper name resolution, but set the Host header to the original URL to test the workload proxy logic
//
// sample svcURL: https://j2s7hf-local.proxy-us.localhost:8099/
func prepareWorkloadProxyRequest(ctx context.Context, t *testing.T, svcURL string) *http.Request {
	t.Logf("url of the exposed service: %q", svcURL)

	parsedURL, err := url.Parse(svcURL)
	require.NoError(t, err)

	hostParts := strings.SplitN(parsedURL.Host, ".", 3)
	require.GreaterOrEqual(t, len(hostParts), 3)

	svcHost := parsedURL.Host
	baseHost := hostParts[2]

	parsedURL.Host = baseHost

	baseURL := parsedURL.String()

	t.Logf("base URL which will be used to reach exposed service: %q, host: %q", baseURL, svcHost)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	require.NoError(t, err)

	req.Host = svcHost

	return req
}

func workloadProxyManifests(port int, label, icon string) (appsv1.Deployment, corev1.Service) {
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "nginx",
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
							Name:  "nginx",
							Image: "nginx:stable-alpine-slim",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	}

	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
			Annotations: map[string]string{
				"omni-kube-service-exposer.sidero.dev/port":  strconv.Itoa(port),
				"omni-kube-service-exposer.sidero.dev/label": label,
				"omni-kube-service-exposer.sidero.dev/icon":  icon,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "nginx",
			},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt32(80),
				},
			},
		},
	}

	return deployment, service
}

func doGzip(t *testing.T, input []byte) []byte {
	t.Helper()

	var buf strings.Builder

	writer := gzip.NewWriter(&buf)

	_, err := writer.Write(input)
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	return []byte(buf.String())
}
