// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package interceptor

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/crewjam/saml"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

var errGRPCInvalidSAML = status.Error(codes.Unauthenticated, "invalid session")

// SAML is a GRPC interceptor that verifies SAML session.
type SAML struct {
	state  state.State
	logger *zap.Logger
}

// NewSAML returns a new SAML interceptor.
func NewSAML(state state.State, logger *zap.Logger) *SAML {
	return &SAML{
		state:  state,
		logger: logger,
	}
}

// Unary returns a new unary SAML interceptor.
func (i *SAML) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, err := i.intercept(ctx)
		if err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// Stream returns a new stream SAML interceptor.
func (i *SAML) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, err := i.intercept(ss.Context())
		if err != nil {
			return err
		}

		return handler(srv, &grpc_middleware.WrappedServerStream{
			ServerStream:   ss,
			WrappedContext: ctx,
		})
	}
}

func (i *SAML) intercept(ctx context.Context) (context.Context, error) {
	msgVal, ok := ctxstore.Value[auth.GRPCMessageContextKey](ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "missing or invalid message in context")
	}

	values := msgVal.Message.Metadata.Get(auth.SamlSessionHeaderKey)
	if len(values) == 0 {
		return ctx, nil
	}

	session, err := i.getSession(ctx, values[0])
	if err != nil {
		return nil, errGRPCInvalidSAML
	}

	ctx = ctxstore.WithValue(ctx, auth.VerifiedEmailContextKey{Email: session.TypedSpec().Value.Email})

	return ctx, nil
}

func (i *SAML) getSession(ctx context.Context, sessionID string) (*authres.SAMLAssertion, error) {
	ctx = actor.MarkContextAsInternalActor(ctx)

	acs, err := safe.StateGet[*authres.SAMLAssertion](ctx, i.state, authres.NewSAMLAssertion(resources.DefaultNamespace, sessionID).Metadata())
	if err != nil {
		i.logger.Info("invalid session", zap.Error(err))

		return nil, errGRPCInvalidSAML
	}

	var assertion saml.Assertion

	err = json.Unmarshal(acs.TypedSpec().Value.Data, &assertion)
	if err != nil {
		return nil, err
	}

	if acs.TypedSpec().Value.Used {
		i.logger.Info("invalid session", zap.Error(errors.New("session was already used")))

		return nil, errGRPCInvalidSAML
	}

	if assertion.IssueInstant.Add(saml.MaxIssueDelay).Before(time.Now().UTC()) {
		i.logger.Info("invalid session", zap.Error(errors.New("SAML assertion expired")))

		return nil, errGRPCInvalidSAML
	}

	_, err = safe.StateUpdateWithConflicts(ctx, i.state, acs.Metadata(), func(r *authres.SAMLAssertion) error {
		r.TypedSpec().Value.Used = true

		return nil
	})

	return acs, err
}
