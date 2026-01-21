// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package external provides constants which are consumed in other places.
package external

import "time"

// DefaultClientID is the client_id of the default (an only) client.
const DefaultClientID = "native"

// ScopeClusterPrefix defines the scope prefix to specify cluster name.
const ScopeClusterPrefix = "cluster:"

// OIDCTokenLifetime specifies the lifetime of the JWT token.
const OIDCTokenLifetime = 12 * time.Hour

// ServiceAccountTokenLifetime specifies the lifetime of the (kubeconfig) service account token.
const ServiceAccountTokenLifetime = 10 * 365 * 24 * time.Hour

// KeyRotationInterval specifies the interval in which the keys are rotated.
const KeyRotationInterval = 30 * 24 * time.Hour

// KeyCodeRedirectURL is the redirect URL for the keycode authentication method.
const KeyCodeRedirectURL = "urn:ietf:wg:oauth:2.0:oob"
