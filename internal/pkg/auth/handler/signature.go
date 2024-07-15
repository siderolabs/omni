// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package handler

import (
	"context"
	"errors"
	"net/http"

	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

var errInvalidSignature = errors.New("invalid signature")

// Signature represents a signature handler.
type Signature struct {
	authenticatorFunc auth.AuthenticatorFunc
	next              http.Handler
	logger            *zap.Logger
}

// NewSignature returns a new signature handler.
func NewSignature(handler http.Handler, authenticatorFunc auth.AuthenticatorFunc, logger *zap.Logger) *Signature {
	return &Signature{
		next:              handler,
		authenticatorFunc: authenticatorFunc,
		logger:            logger,
	}
}

// ServeHTTP implements the http.Handler interface.
func (s *Signature) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	req, err := s.intercept(request)
	if errors.Is(err, errInvalidSignature) {
		writer.WriteHeader(http.StatusUnauthorized)

		return
	}

	if err != nil {
		s.logger.Error("failed to intercept request", zap.Error(err))

		writer.WriteHeader(http.StatusInternalServerError)

		return
	}

	s.next.ServeHTTP(writer, req)
}

func (s *Signature) intercept(request *http.Request) (*http.Request, error) {
	msg, err := message.NewHTTP(request)
	if err != nil {
		return nil, err
	}

	signature, err := msg.Signature()
	if errors.Is(err, message.ErrNotFound) { // missing signature, pass it through
		return request, nil
	}

	if err != nil {
		s.logger.Warn("failed to get signature", zap.Error(err))

		return nil, errInvalidSignature
	}

	ctx := request.Context()

	// set the initial identity based on the signature header, it can't be trusted yet, but it gives
	// a better context if the auth fails
	grpc_ctxtags.Extract(ctx).
		Set("authenticator.identity", signature.Identity)

	authenticator, err := s.authenticatorFunc(request.Context(), signature.KeyFingerprint)
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return nil, context.Canceled
		}

		s.logger.Warn("failed to get authenticator", zap.Error(err))

		return nil, errInvalidSignature
	}

	err = msg.VerifySignature(authenticator.Verifier)
	if err != nil {
		s.logger.Info("failed to verify message", zap.Error(err))

		return nil, errInvalidSignature
	}

	grpc_ctxtags.Extract(ctx).
		Set("authenticator.user_id", authenticator.UserID).
		Set("authenticator.identity", authenticator.Identity).
		Set("authenticator.role", string(authenticator.Role))

	ctx = ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: authenticator.Identity})
	ctx = ctxstore.WithValue(ctx, auth.UserIDContextKey{UserID: authenticator.UserID})
	ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: authenticator.Role})

	return request.WithContext(ctx), nil
}
