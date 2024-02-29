// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package scope

// Perspective is the sub-action of the scope.
type Perspective string

// Perspective constants.
const (
	PerspectiveNone                     = ""
	PerspectiveKubernetesAccess         = "kubernetes-api-access"
	PerspectiveKubeconfigServiceAccount = "kubeconfig-service-account"
	PerspectiveKubernetesUpgrade        = "kubernetes-upgrade"
)
