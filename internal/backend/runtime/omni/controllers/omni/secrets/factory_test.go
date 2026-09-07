// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package secrets_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/secretrotation"
)

type fakeKubernetesClientFactory struct{}

func (f fakeKubernetesClientFactory) NewClient(config *rest.Config) (secretrotation.KubernetesClient, error) {
	// Create a test server that returns ready nodes
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a list of ready nodes for the /api/v1/nodes endpoint
		nodeList := &corev1.NodeList{
			Kind:       "NodeList",
			APIVersion: "v1",
			Items: []corev1.Node{
				{
					Name: "node-1",
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				{
					Name: "node-2",
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")

		err := json.NewEncoder(w).Encode(nodeList)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))

	// Configure the REST client to use the test server
	config.Host = server.URL
	config.TLSClientConfig = rest.TLSClientConfig{
		Insecure: true,
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		server.Close()

		return nil, err
	}

	return kubernetesClient{clientSet: clientSet, server: server}, nil
}

type kubernetesClient struct {
	clientSet *kubernetes.Clientset
	server    *httptest.Server
}

func (k kubernetesClient) Clientset() *kubernetes.Clientset {
	return k.clientSet
}

func (k kubernetesClient) Close() {
	if k.server != nil {
		k.server.Close()
	}
}
