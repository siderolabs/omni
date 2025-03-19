// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package constants

// AnotherTalosVersion is used in the integration tests for Talos upgrade.
const AnotherTalosVersion = "1.9.0"

// MinDiscoveredTalosVersion makes Omni pull the versions from this point.
const MinDiscoveredTalosVersion = "1.3.0"

// MinTalosVersion allowed to be used when creating the cluster.
const MinTalosVersion = "1.5.0"

// DefaultKubernetesVersion is pre-selected in the UI and used in the integration tests.
//
// tsgen:DefaultKubernetesVersion
const DefaultKubernetesVersion = "1.32.3"

// AnotherKubernetesVersion is used in the integration tests for Kubernetes upgrade.
const AnotherKubernetesVersion = "1.31.7"

// MinKubernetesVersion allowed to be used when creating the cluster.
const MinKubernetesVersion = "1.24.0"

// DenylistedTalosVersions is a list of versions which should never show up in the version picker.
var DenylistedTalosVersions = map[string]struct{}{
	"1.4.2": {}, // issue with the number of open files limit
	"1.4.3": {}, // issue with the number of open files limit
}
