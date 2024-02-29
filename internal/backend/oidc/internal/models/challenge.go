// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package models

import "github.com/zitadel/oidc/pkg/oidc"

// OIDCCodeChallenge is a code challenge as defined in https://tools.ietf.org/html/rfc7636.
type OIDCCodeChallenge struct {
	Challenge string
	Method    string
}

func codeChallengeToOIDC(challenge *OIDCCodeChallenge) *oidc.CodeChallenge {
	if challenge == nil {
		return nil
	}

	challengeMethod := oidc.CodeChallengeMethodPlain

	if challenge.Method == "S256" {
		challengeMethod = oidc.CodeChallengeMethodS256
	}

	return &oidc.CodeChallenge{
		Challenge: challenge.Challenge,
		Method:    challengeMethod,
	}
}
