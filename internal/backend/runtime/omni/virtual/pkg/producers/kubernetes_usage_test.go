// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package producers //nolint:testpackage // exercises unexported calculateUsage.

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
)

func newPodWithCPURequest(phase corev1.PodPhase, cpu string, reason string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-" + string(phase) + "-" + reason},
		Status: corev1.PodStatus{
			Phase:  phase,
			Reason: reason,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse(cpu),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse(cpu),
						},
					},
				},
			},
		},
	}
}

func TestCalculateUsageExcludesTerminalPods(t *testing.T) {
	pods := []*corev1.Pod{
		newPodWithCPURequest(corev1.PodRunning, "1", ""),
		newPodWithCPURequest(corev1.PodFailed, "2", "Evicted"),
		newPodWithCPURequest(corev1.PodSucceeded, "3", ""),
	}

	usage := virtual.NewKubernetesUsage("test-cluster")

	calculateUsage(pods, nil, usage)

	spec := usage.TypedSpec().Value

	require.InDelta(t, 1, spec.Cpu.Requests, 0.001)
	require.InDelta(t, 1, spec.Cpu.Limits, 0.001)
	require.Equal(t, int32(1), spec.Pods.Count)
}
