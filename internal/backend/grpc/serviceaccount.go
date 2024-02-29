// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
	"github.com/siderolabs/omni/internal/backend/oidc/external"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func (s *managementServer) serviceAccountKubeconfig(ctx context.Context, req *management.KubeconfigRequest) (*management.KubeconfigResponse, error) {
	if _, err := auth.CheckGRPC(ctx, auth.WithRole(role.Operator)); err != nil {
		return nil, err
	}

	cluster := router.ExtractContext(ctx).Name

	if err := s.validateServiceAccountRequest(cluster, req); err != nil {
		return nil, err
	}

	clusterUUID, err := safe.StateGetByID[*omni.ClusterUUID](ctx, s.omniState, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster UUID: %w", err)
	}

	signedToken, err := s.generateServiceAccountJWT(req, cluster, clusterUUID.TypedSpec().Value.GetUuid())
	if err != nil {
		return nil, err
	}

	kubeconfig, err := s.buildServiceAccountKubeconfig(cluster, req.GetServiceAccountUser(), signedToken)
	if err != nil {
		return nil, err
	}

	return &management.KubeconfigResponse{
		Kubeconfig: kubeconfig,
	}, nil
}

func (s *managementServer) validateServiceAccountRequest(cluster string, req *management.KubeconfigRequest) error {
	if cluster == "" {
		return status.Error(codes.InvalidArgument, "cluster name is not in context")
	}

	if req.GetServiceAccountUser() == "" {
		return status.Error(codes.InvalidArgument, "service account user name is not set")
	}

	if req.GetServiceAccountTtl() == nil {
		return status.Error(codes.InvalidArgument, "service account ttl is not set")
	}

	ttl := req.GetServiceAccountTtl().AsDuration()

	if ttl <= 0 {
		return status.Error(codes.InvalidArgument, "service account ttl is must be positive")
	}

	if ttl > external.ServiceAccountTokenLifetime {
		return status.Errorf(codes.InvalidArgument, "service account ttl is too long (max allowed: %s)", external.ServiceAccountTokenLifetime)
	}

	return nil
}

func (s *managementServer) generateServiceAccountJWT(req *management.KubeconfigRequest, clusterName, clusterUUID string) (string, error) {
	signingKey, err := s.jwtSigningKeyProvider.GetCurrentSigningKey()
	if err != nil {
		return "", err
	}

	signingMethod := jwt.GetSigningMethod(signingKey.Algorithm)

	now := time.Now()
	token := jwt.NewWithClaims(signingMethod, jwt.MapClaims{
		"iat":          now.Unix(),
		"iss":          fmt.Sprintf("omni-%s-service-account-issuer", config.Config.Name),
		"exp":          now.Add(req.GetServiceAccountTtl().AsDuration()).Unix(),
		"sub":          req.GetServiceAccountUser(),
		"groups":       req.GetServiceAccountGroups(),
		"cluster":      clusterName,
		"cluster_uuid": clusterUUID,
	})

	token.Header["kid"] = signingKey.KeyID

	return token.SignedString(signingKey.Key)
}

func (s *managementServer) buildServiceAccountKubeconfig(cluster, user, token string) ([]byte, error) {
	clusterName := config.Config.Name + "-" + cluster + "-" + user
	contextName := clusterName

	conf := clientcmdapi.Config{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: contextName,
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server: config.Config.KubernetesProxyURL,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:   clusterName,
				Namespace: "default",
				AuthInfo:  clusterName,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			clusterName: {
				Token: token,
			},
		},
	}

	return clientcmd.Write(conf)
}
