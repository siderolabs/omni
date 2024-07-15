// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package k8sproxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/golang-jwt/jwt/v4"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"go.uber.org/zap"
	"k8s.io/client-go/transport"

	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

const authorizationHeader = "Authorization"

// KeyProvider implements a function which returns a public key with a given key ID to verify JWT token.
type KeyProvider func(ctx context.Context, keyID string) (any, error)

// ClusterUUIDResolver resolves a cluster ID to its UUID.
type ClusterUUIDResolver func(ctx context.Context, clusterID resource.ID) (string, error)

// AuthorizeRequest checks for valid token in the request.
func AuthorizeRequest(next http.Handler, keyFunc KeyProvider, clusterUUIDResolver ClusterUUIDResolver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		authorization := req.Header.Get(authorizationHeader)

		bearer, tok, ok := strings.Cut(authorization, " ")
		if !ok || bearer != "Bearer" {
			ctxzap.Error(ctx, "invalid authorization header")

			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		token, err := jwt.ParseWithClaims(tok, &claims{}, func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			keyID, ok := token.Header["kid"].(string)
			if !ok {
				ctxzap.Error(ctx, "invalid token header", zap.Any("header", token.Header))

				return nil, errors.New("invalid token header")
			}

			return keyFunc(ctx, keyID)
		})
		if err != nil {
			ctxzap.Error(ctx, "failed to validate JWT token", zap.Error(err))

			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(err.Error())) //nolint:errcheck

			return
		}

		claims, _ := token.Claims.(*claims) //nolint:errcheck
		clusterName := claims.Cluster
		clusterUUID := claims.ClusterUUID

		grpc_ctxtags.Extract(req.Context()).
			Set("cluster", clusterName).
			Set("cluster_uuid", clusterUUID).
			Set("impersonate.user", claims.Subject).
			Set("impersonate.groups", claims.Groups)

		if clusterName == "" {
			ctxzap.Error(ctx, "cluster name is empty")

			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		// Allow JWTs without cluster UUID for backwards compatibility - use their "cluster" claim as the target cluster.
		// If this is a newer JWT with the "cluster_uuid" claim, get the matching cluster uuid and validate it against the "cluster_uuid" claim.
		if clusterUUID != "" {
			resolvedClusterUUID, err := clusterUUIDResolver(ctx, clusterName)
			if err != nil {
				ctxzap.Error(ctx, "failed to resolve cluster UUID", zap.Error(err))

				w.WriteHeader(http.StatusUnauthorized)

				return
			}

			if resolvedClusterUUID != clusterUUID {
				ctxzap.Error(ctx, "cluster UUID does not match cluster name")

				w.WriteHeader(http.StatusUnauthorized)

				return
			}
		}

		// clone the request before modifying it
		req = req.WithContext(ctxstore.WithValue(ctx, clusterContextKey{ClusterName: clusterName}))

		// clean all headers which are going to be overridden
		req.Header.Del(authorizationHeader)
		req.Header.Del(transport.ImpersonateUserHeader)
		req.Header.Del(transport.ImpersonateGroupHeader)

		req.Header.Add(transport.ImpersonateUserHeader, claims.Subject)

		for _, group := range claims.Groups {
			req.Header.Add(transport.ImpersonateGroupHeader, group)
		}

		next.ServeHTTP(w, req)
	})
}
