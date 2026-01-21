// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build sidero.debug

package grpc

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// DebugVerifiedEmailHeaderKey is the metadata key that allows specifying a verified email in debug builds.
//
// When this key is found in the metadata, JWT validation is skipped and the email is taken as the truth.
const DebugVerifiedEmailHeaderKey = "x-sidero-debug-verified-email"

func debugEmail(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	emails := md.Get(DebugVerifiedEmailHeaderKey)
	if len(emails) == 0 {
		return ""
	}

	return emails[0]
}
