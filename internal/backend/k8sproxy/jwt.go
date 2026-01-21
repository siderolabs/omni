// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package k8sproxy

import (
	"github.com/golang-jwt/jwt/v4"
)

// claims represents a JWT claims.
//
// Kubernetes proxy expects all three claims to be present in the token.
type claims struct {
	jwt.RegisteredClaims

	// Cluster is the name of the cluster.
	// For legacy JWTs in which ClusterUUID is not present, this field is used as the source of truth.
	// For all new JWTs issued, ClusterUUID field is used as the source of truth, and this field is solely checked if it matches with the UUID.
	Cluster string `json:"cluster"`

	// ClusterUUID is the UUID of the cluster.
	ClusterUUID string `json:"cluster_uuid"`

	// Groups are the groups the subject belongs to.
	Groups []string `json:"groups"`
}

func (claims *claims) Valid() error {
	if err := claims.RegisteredClaims.Valid(); err != nil {
		return err
	}

	if claims.ExpiresAt == nil {
		return jwt.NewValidationError("expiration is empty", jwt.ValidationErrorClaimsInvalid)
	}

	if claims.Cluster == "" {
		return jwt.NewValidationError("cluster is empty", jwt.ValidationErrorClaimsInvalid)
	}

	if claims.Subject == "" {
		return jwt.NewValidationError("subject is empty", jwt.ValidationErrorClaimsInvalid)
	}

	if len(claims.Groups) == 0 {
		return jwt.NewValidationError("groups is empty", jwt.ValidationErrorClaimsInvalid)
	}

	return nil
}
