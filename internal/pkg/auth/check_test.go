// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

func TestCheck(t *testing.T) {
	for _, tt := range []struct { //nolint:govet
		name    string
		ctx     context.Context //nolint:containedctx
		opts    []auth.CheckOption
		want    auth.CheckResult
		errorIs error
	}{
		{
			name:    "empty context",
			ctx:     context.Background(),
			errorIs: auth.ErrUnauthenticated,
		},
		{
			name: "auth disabled",
			ctx: ctxstore.WithValue(
				context.Background(),
				auth.EnabledAuthContextKey{
					Enabled: false,
				},
			),
		},
		{
			name: "not authenticated, no requirements",
			ctx: ctxstore.WithValue(
				context.Background(),
				auth.EnabledAuthContextKey{
					Enabled: true,
				},
			),
			want: auth.CheckResult{
				AuthEnabled: true,
				Role:        role.None,
			},
		},
		{
			name: "not authenticated, verified email",
			ctx: ctxstore.WithValue(
				context.Background(),
				auth.EnabledAuthContextKey{
					Enabled: true,
				},
			),
			opts:    []auth.CheckOption{auth.WithVerifiedEmail()},
			errorIs: auth.ErrUnauthenticated,
		},
		{
			name: "not authenticated, none role",
			ctx: ctxstore.WithValue(
				context.Background(),
				auth.EnabledAuthContextKey{
					Enabled: true,
				},
			),
			opts:    []auth.CheckOption{auth.WithValidSignature(true)},
			errorIs: auth.ErrUnauthenticated,
		},
		{
			name: "not authenticated, operator role",
			ctx: ctxstore.WithValue(
				context.Background(),
				auth.EnabledAuthContextKey{
					Enabled: true,
				},
			),
			opts:    []auth.CheckOption{auth.WithRole(role.Operator)},
			errorIs: auth.ErrUnauthenticated,
		},
		{
			name: "verified email",
			ctx: ctxstore.WithValue(
				ctxstore.WithValue(
					context.Background(),
					auth.EnabledAuthContextKey{
						Enabled: true,
					},
				),
				auth.VerifiedEmailContextKey{
					Email: "user@example.com",
				},
			),
			opts: []auth.CheckOption{auth.WithVerifiedEmail()},
			want: auth.CheckResult{
				AuthEnabled:   true,
				VerifiedEmail: "user@example.com",
				Role:          role.None,
			},
		},
		{
			name: "role okay",
			ctx: ctxstore.WithValue(
				ctxstore.WithValue(
					context.Background(),
					auth.EnabledAuthContextKey{
						Enabled: true,
					},
				),
				auth.RoleContextKey{
					Role: role.Operator,
				},
			),
			opts: []auth.CheckOption{auth.WithRole(role.Operator)},
			want: auth.CheckResult{
				AuthEnabled:       true,
				HasValidSignature: true,
				Role:              role.Operator,
			},
		},
		{
			name: "role mismatch",
			ctx: ctxstore.WithValue(
				ctxstore.WithValue(
					context.Background(),
					auth.EnabledAuthContextKey{
						Enabled: true,
					},
				),
				auth.RoleContextKey{
					Role: role.Operator,
				},
			),
			opts:    []auth.CheckOption{auth.WithRole(role.Admin)},
			errorIs: auth.ErrUnauthorized,
		},
		{
			name: "role and verified email",
			ctx: ctxstore.WithValue(
				ctxstore.WithValue(
					ctxstore.WithValue(
						ctxstore.WithValue(
							context.Background(),
							auth.EnabledAuthContextKey{
								Enabled: true,
							},
						),
						auth.RoleContextKey{
							Role: role.Operator,
						},
					),
					auth.VerifiedEmailContextKey{
						Email: "user@example.com",
					},
				),
				auth.IdentityContextKey{
					Identity: "user2@example.com",
				},
			),
			opts: []auth.CheckOption{auth.WithRole(role.Operator), auth.WithVerifiedEmail()},
			want: auth.CheckResult{
				AuthEnabled:       true,
				HasValidSignature: true,
				Role:              role.Operator,
				Identity:          "user2@example.com",
				VerifiedEmail:     "user@example.com",
			},
		},
		{
			name: "valid signature",
			ctx: ctxstore.WithValue(
				ctxstore.WithValue(
					context.Background(),
					auth.EnabledAuthContextKey{
						Enabled: true,
					},
				),
				auth.RoleContextKey{
					Role: role.None,
				},
			),
			opts: []auth.CheckOption{},
			want: auth.CheckResult{
				AuthEnabled:       true,
				HasValidSignature: true,
				Role:              role.None,
			},
		},
		{
			name: "missing signature",
			ctx: ctxstore.WithValue(
				ctxstore.WithValue(
					context.Background(),
					auth.EnabledAuthContextKey{
						Enabled: true,
					},
				),
				auth.VerifiedEmailContextKey{
					Email: "me@example.com",
				},
			),
			opts:    []auth.CheckOption{auth.WithValidSignature(true)},
			errorIs: auth.ErrUnauthenticated,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := auth.Check(tt.ctx, tt.opts...)

			if tt.errorIs != nil {
				assert.ErrorIs(t, err, tt.errorIs)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}
