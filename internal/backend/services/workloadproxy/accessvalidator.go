// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/authz"
	"github.com/siderolabs/omni/internal/pkg/authz/grants"
)

// SignatureAccessValidator validates a signature using PGP keys and roles/ACLs.
type SignatureAccessValidator struct {
	authenticate auth.AuthenticatorFunc
	state        state.State
}

// NewSignatureAccessValidator creates a new PGP signature validator.
func NewSignatureAccessValidator(authenticate auth.AuthenticatorFunc, st state.State) (*SignatureAccessValidator, error) {
	if authenticate == nil {
		return nil, errors.New("authenticator is nil")
	}

	if st == nil {
		return nil, errors.New("state is nil")
	}

	return &SignatureAccessValidator{
		authenticate: authenticate,
		state:        st,
	}, nil
}

// ValidateAccess validates the access to an exposed service in the given cluster ID,
// using the PGP public keys in the Omni database.
func (p *SignatureAccessValidator) ValidateAccess(ctx context.Context, publicKeyID, publicKeyIDSignatureBase64 string, clusterID resource.ID) error {
	singatureBytes, err := base64.StdEncoding.DecodeString(publicKeyIDSignatureBase64)
	if err != nil {
		return err
	}

	authenticator, err := p.authenticate(ctx, publicKeyID)
	if err != nil {
		return err
	}

	if err = authenticator.Verifier.Verify([]byte(publicKeyID), singatureBytes); err != nil {
		return err
	}

	ctx = auth.ContextWithAuthenticator(ctx, authenticator, publicKeyID)

	// check that the caller can read the exposed services of the cluster
	return grants.CheckResourceGroup(ctx, p.state, authz.ExposedServices, authz.Read, authz.Target{Cluster: clusterID})
}
