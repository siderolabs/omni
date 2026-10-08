// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package workloadproxy provides functions for proxying traffic to workload clusters.
package workloadproxy

import (
	"net/http"
	"strings"
)

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

// dropProxyCookies removes the workload proxy cookies from the Cookie header, keeping the rest as is.
func dropProxyCookies(header http.Header) {
	lines := header.Values("Cookie")

	header.Del("Cookie")

	for _, line := range lines {
		var kept []string

		for part := range strings.SplitSeq(line, ";") {
			part = strings.TrimSpace(part)

			if name, _, _ := strings.Cut(part, "="); part == "" || name == PublicKeyIDCookie || name == PublicKeyIDSignatureBase64Cookie {
				continue
			}

			kept = append(kept, part)
		}

		if len(kept) > 0 {
			header.Add("Cookie", strings.Join(kept, "; "))
		}
	}
}
