// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package exposedservice provides helpers for controllers to manage ExposedServices.
package exposedservice

import (
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	"github.com/siderolabs/omni/client/pkg/constants"
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
		_, isAnnotated := service.GetObjectMeta().GetAnnotations()[constants.ExposedServicePortAnnotationKey]

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

	for _, key := range []string{
		constants.ExposedServiceLabelAnnotationKey, constants.ExposedServicePortAnnotationKey,
		constants.ExposedServiceIconAnnotationKey, constants.ExposedServicePrefixAnnotationKey,
	} {
		if oldAnnotations[key] != newAnnotations[key] {
			return true
		}
	}

	// no change in exposed service related annotations
	return false
}
