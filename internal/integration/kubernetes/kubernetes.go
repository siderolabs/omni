// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package kubernetes provides utilities for testing Kubernetes clusters.
package kubernetes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/siderolabs/omni/client/pkg/client/management"
	"github.com/siderolabs/omni/client/pkg/constants"
)

// WrapContext wraps the provided context with a zap logger so that the Kubernetes client sends its logs to that, complying with the go test logging format.
func WrapContext(ctx context.Context, t *testing.T) context.Context {
	logger := zaptest.NewLogger(t)
	ctx = logr.NewContext(ctx, zapr.NewLogger(logger.With(zap.String("component", "k8s_client"))))

	return ctx
}

// GetClient retrieves a Kubernetes clientset for the specified cluster using the management client.
func GetClient(ctx context.Context, t *testing.T, managementClient *management.Client, clusterName string) *kubernetes.Clientset {
	// use service account kubeconfig to bypass oidc flow
	kubeconfigBytes, err := managementClient.WithCluster(clusterName).Kubeconfig(ctx,
		management.WithServiceAccount(24*time.Hour, "integration-test", constants.DefaultAccessGroup),
	)
	require.NoError(t, err)

	kubeconfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(kubeconfigBytes)
	})
	require.NoError(t, err)

	cli, err := kubernetes.NewForConfig(kubeconfig)
	require.NoError(t, err)

	return cli
}

// ScaleDeployment scales a Kubernetes deployment to the specified number of replicas and waits until the deployment is scaled.
func ScaleDeployment(ctx context.Context, t *testing.T, kubeClient kubernetes.Interface, namespace, name string, numReplicas uint8) {
	deployment, err := kubeClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err, "failed to get deployment %q/%q", namespace, name)

	// scale down the deployment to 0 replicas
	deployment.Spec.Replicas = pointer.To(int32(numReplicas))

	if _, err := kubeClient.AppsV1().Deployments(deployment.Namespace).Update(ctx, deployment, metav1.UpdateOptions{}); !apierrors.IsNotFound(err) {
		require.NoError(t, err)
	}

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		dep, err := kubeClient.AppsV1().Deployments(deployment.Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			require.NoError(collect, err)
		}

		if !assert.NoError(collect, err) {
			t.Logf("failed to get deployment %q/%q: %q", deployment.Namespace, deployment.Name, err)

			return
		}

		if !assert.Equal(collect, int(numReplicas), int(dep.Status.ReadyReplicas)) {
			t.Logf("deployment %q/%q does not have expected number of replicas: expected %d, found %d",
				deployment.Namespace, deployment.Name, numReplicas, dep.Status.ReadyReplicas)
		}
	}, 3*time.Minute, 5*time.Second)
}

// UpdateService updates a Kubernetes service to add or modify the specified annotations and waits until the service is updated.
func UpdateService(ctx context.Context, t *testing.T, kubeClient kubernetes.Interface, namespace, name string, f func(*corev1.Service)) {
	service, err := kubeClient.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	require.NoError(t, err, "failed to get service %q/%q", namespace, name)

	f(service)

	_, err = kubeClient.CoreV1().Services(namespace).Update(ctx, service, metav1.UpdateOptions{})
	require.NoError(t, err, "failed to update service %q/%q", namespace, name)
}
