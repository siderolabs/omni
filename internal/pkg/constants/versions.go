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
//
// It must stay within the same minor as DefaultTalosVersion. Canceling an upgrade downgrades the machine
// that already took the new version, a control plane cannot cross a Talos minor downward, and the upgrade
// cancellation test skips itself on a cross-minor pair, so letting these versions drift apart would
// silently drop that coverage from every suite.
const AnotherTalosVersion = "1.14.0-rc.2"

// StableTalosVersion is used in the integration tests for Talos upgrade between minor versions.
const StableTalosVersion = "1.13.9"

// MinDiscoveredTalosVersion makes Omni pull the versions from this point.
const MinDiscoveredTalosVersion = "1.3.0"

// DefaultKubernetesVersion is pre-selected in the UI and used in the integration tests.
//
// tsgen:DefaultKubernetesVersion
const DefaultKubernetesVersion = "1.37.0"

// DefaultTalosVersion to be used in the tests.
const DefaultTalosVersion = constants.DefaultTalosVersion

// AnotherKubernetesVersion is used in the integration tests for Kubernetes upgrade.
const AnotherKubernetesVersion = "1.36.4"

// MinDiscoveredKubernetesVersion makes Omni pull the versions from this point.
const MinDiscoveredKubernetesVersion = "1.23.0"

// DenylistedTalosVersions is a list of versions which should never show up in the version picker.
var DenylistedTalosVersions = Denylist{
	"1.13.1": {}, // kernel modules that are shipped as extensions when using Imager/Image Factory are broken
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
