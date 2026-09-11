// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import "github.com/siderolabs/omni/client/pkg/omni/resources/omni"

const (
	// LabelJoinTokenFingerprint is set on the JoinTokenStatus and keeps the non-secret handle of the
	// join token, so that a signed join token can point at the token which signed it without
	// carrying the secret. See jointoken.Fingerprint.
	LabelJoinTokenFingerprint = omni.SystemLabelPrefix + "join-token-fingerprint"
)
