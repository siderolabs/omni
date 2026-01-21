// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build !sidero.debug

package grpc

import "context"

func debugEmail(context.Context) string {
	return ""
}
