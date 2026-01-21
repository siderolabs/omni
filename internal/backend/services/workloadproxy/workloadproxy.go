// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package workloadproxy provides functions for proxying traffic to workload clusters.
package workloadproxy

const (
	// LegacyHostPrefix is the prefix used to distinguish subdomain requests in the legacy domain format which should be proxied to the workload clusters.
	LegacyHostPrefix = "p"

	// PublicKeyIDCookie is the name of the cookie used for workload proxy request authentication that contains the public key ID.
	//
	// tsgen:workloadProxyPublicKeyIdCookie
	PublicKeyIDCookie = "publicKeyId"

	// PublicKeyIDSignatureBase64Cookie is the name of the cookie used for workload proxy request authentication that contains the signed & base64'd public key ID.
	//
	// tsgen:workloadProxyPublicKeyIdSignatureBase64Cookie
	PublicKeyIDSignatureBase64Cookie = "publicKeyIdSignatureBase64"
)
