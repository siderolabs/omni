// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package oidc provides resources related to the OpenID provider.
package oidc

import (
	"github.com/cosi-project/runtime/pkg/resource"

	"github.com/siderolabs/omni/client/pkg/omni/resources/registry"
)

// NamespaceName is the namespace for OIDC resources.
const NamespaceName resource.Namespace = "oidc"

func init() {
	registry.MustRegisterResource(JWTPublicKeyType, &JWTPublicKey{})
}
