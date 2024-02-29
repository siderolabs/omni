// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package auth provides resources related to the authentication.
package auth

import "github.com/siderolabs/omni/client/pkg/omni/resources/registry"

func init() {
	registry.MustRegisterResource(AuthConfigType, &Config{})
	registry.MustRegisterResource(IdentityType, &Identity{})
	registry.MustRegisterResource(PublicKeyType, &PublicKey{})
	registry.MustRegisterResource(UserType, &User{})
	registry.MustRegisterResource(AccessPolicyType, &AccessPolicy{})
	registry.MustRegisterResource(SAMLAssertionType, &SAMLAssertion{})
	registry.MustRegisterResource(SAMLLabelRuleType, &SAMLLabelRule{})
}
