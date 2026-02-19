// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package constants

import (
	"fmt"

	"github.com/blang/semver/v4"

	"github.com/siderolabs/omni/client/pkg/constants"
)

// AnotherTalosVersion is used in the integration tests for Talos upgrade.
const AnotherTalosVersion = "1.12.3"

// StableTalosVersion is used in the integration tests for Talos upgrade between minor versions.
const StableTalosVersion = "1.11.6"

// MinDiscoveredTalosVersion makes Omni pull the versions from this point.
const MinDiscoveredTalosVersion = "1.3.0"

// DefaultKubernetesVersion is pre-selected in the UI and used in the integration tests.
//
// tsgen:DefaultKubernetesVersion
const DefaultKubernetesVersion = "1.35.1"

// DefaultTalosVersion to be used in the tests.
const DefaultTalosVersion = constants.DefaultTalosVersion

// AnotherKubernetesVersion is used in the integration tests for Kubernetes upgrade.
const AnotherKubernetesVersion = "1.33.6"

// MinKubernetesVersion allowed to be used when creating the cluster.
const MinKubernetesVersion = "1.24.0"

// DenylistedTalosVersions is a list of versions which should never show up in the version picker.
var DenylistedTalosVersions = Denylist{
	"1.4.2": {}, // issue with the number of open files limit
	"1.4.3": {}, // issue with the number of open files limit
}

// Denylist helper.
type Denylist map[string]struct{}

// IsAllowed checks if the version of Talos is allowed.
func (d Denylist) IsAllowed(version string) bool {
	if _, ok := d[version]; ok {
		return false
	}

	ver, err := semver.ParseTolerant(version)
	if err != nil {
		return false
	}

	pattern := fmt.Sprintf("%d.%d.*", ver.Major, ver.Minor)

	if _, ok := d[pattern]; ok {
		return false
	}

	return true
}
