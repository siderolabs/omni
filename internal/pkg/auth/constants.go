// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth

import "github.com/siderolabs/go-api-signature/pkg/message"

// These constants here are declared only for tsgen.
//
//nolint:unused
const (
	// tsgen:SignatureHeaderKey
	signatureHeaderKey = message.SignatureHeaderKey

	// tsgen:TimestampHeaderKey
	timestampHeaderKey = message.TimestampHeaderKey

	// tsgen:PayloadHeaderKey
	payloadHeaderKey = message.PayloadHeaderKey

	// AuthorizationHeader metadata key.
	// tsgen:authHeader
	authorizationHeaderKey = message.AuthorizationHeaderKey

	// tsgen:authBearerHeaderPrefix
	bearerPrefix = message.BearerPrefix

	// SignatureVersionV1 is the signature version 1
	// tsgen:SignatureVersionV1
	SignatureVersionV1 message.SignatureVersion = message.SignatureVersionV1

	// SamlSessionHeaderKey is the header key for the SAML session token.
	// tsgen:samlSessionHeader
	SamlSessionHeaderKey = "saml-session"
)
