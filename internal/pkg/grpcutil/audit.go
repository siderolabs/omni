// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpcutil

import (
	"context"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/audit/auditlog"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
	grpc_ctxtags "github.com/siderolabs/omni/internal/pkg/grpcutil/grpctags"
)

// SetAuditInCtx sets audit data in the context.
func SetAuditInCtx(ctx context.Context) context.Context {
	m := grpc_ctxtags.Extract(ctx).Values()

	return ctxstore.WithValue(ctx, &auditlog.Data{
		Session: auditlog.Session{
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
