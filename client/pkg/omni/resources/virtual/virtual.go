// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package virtual contains virtual resources.
package virtual

import "github.com/siderolabs/omni/client/pkg/omni/resources/registry"

func init() {
	registry.MustRegisterResource(CurrentUserType, &CurrentUser{})
	registry.MustRegisterResource(ClusterPermissionsType, &ClusterPermissions{})
	registry.MustRegisterResource(KubernetesUsageType, &KubernetesUsage{})
	registry.MustRegisterResource(PermissionsType, &Permissions{})
}
