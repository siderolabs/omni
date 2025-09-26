/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package resource is copied from kubectl, used to collect resource usage for pods.
package resource

import (
	corev1 "k8s.io/api/core/v1"
)

// PodRequestsAndLimits returns a dictionary of all defined resources summed up for all
// containers of the pod. If pod overhead is non-nil, the pod overhead is added to the
// total container resource requests and to the total container limits which have a
// non-zero quantity.
func PodRequestsAndLimits(pod *corev1.Pod) (reqs, limits corev1.ResourceList) {
	reqs, limits = corev1.ResourceList{}, corev1.ResourceList{}
	for _, container := range pod.Spec.Containers {
		addResourceList(reqs, container.Resources.Requests)
		addResourceList(limits, container.Resources.Limits)
	}

	// init containers define the minimum of any resource
	for _, container := range pod.Spec.InitContainers {
		maxResourceList(reqs, container.Resources.Requests)
		maxResourceList(limits, container.Resources.Limits)
	}

	// Add overhead for running a pod to the sum of requests and to non-zero limits:
	if pod.Spec.Overhead != nil {
		addResourceList(reqs, pod.Spec.Overhead)

		for name, quantity := range pod.Spec.Overhead {
			if value, ok := limits[name]; ok && !value.IsZero() {
				value.Add(quantity)

				limits[name] = value
			}
		}
	}

	return reqs, limits
}

// addResourceList adds the resources in newList to list.
func addResourceList(list, added corev1.ResourceList) {
	for name, quantity := range added {
		value, ok := list[name]
		if !ok {
			list[name] = quantity.DeepCopy()

			continue
		}

		value.Add(quantity)
		list[name] = value
	}
}

// maxResourceList sets list to the greater of list/newList for every resource
// either list.
func maxResourceList(list, added corev1.ResourceList) {
	for name, quantity := range added {
		value, ok := list[name]
		if !ok {
			list[name] = quantity.DeepCopy()

			continue
		}

		if quantity.Cmp(value) > 0 {
			list[name] = quantity.DeepCopy()
		}
	}
}
