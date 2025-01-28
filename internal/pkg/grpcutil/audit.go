// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpcutil

import (
	"context"

	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// SetAuditInCtx sets audit data in the context.
func SetAuditInCtx(ctx context.Context) context.Context {
	m := grpc_ctxtags.Extract(ctx).Values()

	return ctxstore.WithValue(ctx, &audit.Data{
		Session: audit.Session{
			UserAgent: typeAssertOrZero[string](m["user_agent"]),
		},
	})
}

func typeAssertOrZero[T any](v any) T {
	if result, ok := v.(T); ok {
		return result
	}

	return *new(T)
}
