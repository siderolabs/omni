// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package exposedservice provides helpers for controllers to manage ExposedServices.
package exposedservice

import (
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

const (
	// ServiceLabelAnnotationKey is the annotation to define the human-readable label of Kubernetes Services to expose them to Omni.
	//
	// tsgen:ServiceLabelAnnotationKey
	ServiceLabelAnnotationKey = "omni-kube-service-exposer.sidero.dev/label"

	// ServicePortAnnotationKey is the annotation to define the port of Kubernetes Services to expose them to Omni.
	//
	// tsgen:ServicePortAnnotationKey
	ServicePortAnnotationKey = "omni-kube-service-exposer.sidero.dev/port"

	// ServiceIconAnnotationKey is the annotation to define the icon of Kubernetes Services to expose them to Omni.
	//
	// tsgen:ServiceIconAnnotationKey
	ServiceIconAnnotationKey = "omni-kube-service-exposer.sidero.dev/icon"

	// ServicePrefixAnnotationKey is the annotation to define the prefix of Kubernetes Services to expose them to Omni.
	// When it is not defined, a prefix will be generated automatically.
	//
	// tsgen:ServicePrefixAnnotationKey
	ServicePrefixAnnotationKey = "omni-kube-service-exposer.sidero.dev/prefix"
)

// IsExposedServiceEvent returns true if there is a change on the Kubernetes Services
// that is relevant to the ExposedServices, i.e., they need to be re-synced.
//
// oldObj is nil for add and delete events, and non-nil for update events.
func IsExposedServiceEvent(k8sObject, oldK8sObject any, logger *zap.Logger) bool {
	isExposedService := func(obj any) bool {
		service, ok := obj.(*corev1.Service)
		if !ok {
			logger.Warn("unexpected type", zap.String("type", fmt.Sprintf("%T", obj)))

			return false
		}

		// check for ServicePortAnnotationKey annotation - the only annotation required for the exposed services
		_, isAnnotated := service.GetObjectMeta().GetAnnotations()[ServicePortAnnotationKey]

		return isAnnotated
	}

	if k8sObject == nil {
		logger.Warn("unexpected nil k8sObject")

		return false
	}

	// this is an add or delete event
	if oldK8sObject == nil {
		return isExposedService(k8sObject)
	}

	// this is an update event

	// if neither of the old or new objects is an exposed service, there is no change
	if !isExposedService(k8sObject) && !isExposedService(oldK8sObject) {
		return false
	}

	oldAnnotations := oldK8sObject.(*corev1.Service).GetObjectMeta().GetAnnotations() //nolint:forcetypeassert,errcheck
	newAnnotations := k8sObject.(*corev1.Service).GetObjectMeta().GetAnnotations()    //nolint:forcetypeassert,errcheck

	for _, key := range []string{ServiceLabelAnnotationKey, ServicePortAnnotationKey, ServiceIconAnnotationKey, ServicePrefixAnnotationKey} {
		if oldAnnotations[key] != newAnnotations[key] {
			return true
		}
	}

	// no change in exposed service related annotations
	return false
}
