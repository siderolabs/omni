// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package constants

const (
	// ExposedServiceLabelAnnotationKey is the annotation to define the human-readable label of Kubernetes Services to expose them to Omni.
	//
	// tsgen:ExposedServiceLabelAnnotationKey
	ExposedServiceLabelAnnotationKey = "omni-kube-service-exposer.sidero.dev/label"

	// ExposedServicePortAnnotationKey is the annotation to define the port of Kubernetes Services to expose them to Omni.
	//
	// tsgen:ExposedServicePortAnnotationKey
	ExposedServicePortAnnotationKey = "omni-kube-service-exposer.sidero.dev/port"

	// ExposedServiceIconAnnotationKey is the annotation to define the icon of Kubernetes Services to expose them to Omni.
	//
	// tsgen:ExposedServiceIconAnnotationKey
	ExposedServiceIconAnnotationKey = "omni-kube-service-exposer.sidero.dev/icon"

	// ExposedServicePrefixAnnotationKey is the annotation to define the prefix of Kubernetes Services to expose them to Omni.
	// When it is not defined, a prefix will be generated automatically.
	//
	// tsgen:ExposedServicePrefixAnnotationKey
	ExposedServicePrefixAnnotationKey = "omni-kube-service-exposer.sidero.dev/prefix"
)
