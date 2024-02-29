// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package auth

const (
	// SAMLLabelPrefix is the prefix added to all SAML attributes on the User resource.
	// tsgen:SAMLLabelPrefix
	SAMLLabelPrefix = "saml.omni.sidero.dev/"
)

const (
	// LabelPublicKeyUserID is the label that defines the user ID of the public key.
	LabelPublicKeyUserID = "user-id"

	// LabelIdentityUserID is a label linking identity to the user.
	// tsgen:LabelIdentityUserID
	LabelIdentityUserID = "user-id"

	// LabelIdentityTypeServiceAccount is set when the type of the identity is service account.
	// tsgen:LabelIdentityTypeServiceAccount
	LabelIdentityTypeServiceAccount = "type-service-account"
)

const (
	// LabelSAMLRole is the roles attribute that is copied from SAML assertion.
	LabelSAMLRole = SAMLLabelPrefix + "role"

	// LabelSAMLGroups is the groups attribute that is copied from SAML assertion.
	LabelSAMLGroups = SAMLLabelPrefix + "groups"
)
