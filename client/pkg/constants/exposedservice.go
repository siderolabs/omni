// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package constants

const (
	// ExposedServiceAnnotationPrefix is the common prefix shared by all annotations that
	// configure how Kubernetes Services are exposed to Omni.
	//
	// The label, icon, and prefix annotations also accept per-host-port suffixed variants
	// (e.g. "<base>-30080") so that a Service exposing multiple host ports can configure
	// each one independently. The unsuffixed variant is used as a fallback.
	ExposedServiceAnnotationPrefix = "omni-kube-service-exposer.sidero.dev/"

	// ExposedServiceLabelAnnotationKey is the annotation to define the human-readable label of Kubernetes Services to expose them to Omni.
	//
	// tsgen:ExposedServiceLabelAnnotationKey
	ExposedServiceLabelAnnotationKey = ExposedServiceAnnotationPrefix + "label"

	// ExposedServicePortAnnotationKey is the annotation to define the port of Kubernetes Services to expose them to Omni.
	//
	// The value is a comma-separated list of entries. Each entry is either a bare host
	// port or a "host-port:service-port" pair, where the service port can be a number or
	// a name. Each entry produces its own ExposedService.
	//
	// tsgen:ExposedServicePortAnnotationKey
	ExposedServicePortAnnotationKey = ExposedServiceAnnotationPrefix + "port"

	// ExposedServiceIconAnnotationKey is the annotation to define the icon of Kubernetes Services to expose them to Omni.
	//
	// tsgen:ExposedServiceIconAnnotationKey
	ExposedServiceIconAnnotationKey = ExposedServiceAnnotationPrefix + "icon"

	// ExposedServicePrefixAnnotationKey is the annotation to define the prefix of Kubernetes Services to expose them to Omni.
	// When it is not defined, a prefix will be generated automatically.
	//
	// tsgen:ExposedServicePrefixAnnotationKey
	ExposedServicePrefixAnnotationKey = ExposedServiceAnnotationPrefix + "prefix"
)
