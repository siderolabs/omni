// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy_test

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/services/workloadproxy"
	"github.com/siderolabs/omni/internal/pkg/auth/authenticator"
)

type mockRoleProvider struct {
	role role.Role

	clusterIDs []resource.ID
}

func (m *mockRoleProvider) RoleForCluster(_ context.Context, id resource.ID) (role.Role, error) {
	m.clusterIDs = append(m.clusterIDs, id)

	return m.role, nil
}

func TestAccessValidator(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	roleProvider := &mockRoleProvider{
		role: role.Reader,
	}

	accessValidator, err := workloadproxy.NewSignatureAccessValidator(authenticator.New(st, false), roleProvider, zaptest.NewLogger(t))
	require.NoError(t, err)

	key, err := pgp.GenerateKey("test", "", "test@example.com", 8*time.Hour)
	require.NoError(t, err)

	armored, err := key.ArmorPublic()
	require.NoError(t, err)

	user := auth.NewUser("test-user-id")
	user.TypedSpec().Value.Role = string(role.Admin)

	require.NoError(t, st.Create(ctx, user))

	publicKey := auth.NewPublicKey("test-public-key-id")
	publicKey.Metadata().Labels().Set(auth.LabelPublicKeyUserID, user.Metadata().ID())

	publicKey.TypedSpec().Value.PublicKey = []byte(armored)
	publicKey.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(8 * time.Hour))
	publicKey.TypedSpec().Value.Confirmed = true //nolint:staticcheck
	publicKey.TypedSpec().Value.Identity = &specs.Identity{Email: "test@example.com"}

	require.NoError(t, st.Create(ctx, publicKey))

	err = accessValidator.ValidateAccess(ctx, publicKey.Metadata().ID(), base64.StdEncoding.EncodeToString([]byte("invalid-test-signature")), "test-cluster")
	require.Error(t, err)

	signature, err := key.Sign([]byte(publicKey.Metadata().ID()))
	require.NoError(t, err)

	err = accessValidator.ValidateAccess(ctx, publicKey.Metadata().ID(), base64.StdEncoding.EncodeToString(signature), "test-cluster")
	require.NoError(t, err)

	require.Len(t, roleProvider.clusterIDs, 1)
	require.Equal(t, "test-cluster", roleProvider.clusterIDs[0])

	// access should be denied if the role is less than Reader

	roleProvider.role = role.None

	err = accessValidator.ValidateAccess(ctx, publicKey.Metadata().ID(), base64.StdEncoding.EncodeToString(signature), "test-cluster")
	require.Error(t, err)
}

// TestAccessValidatorUnconfirmedKey checks that a key nobody confirmed does not grant access.
func TestAccessValidatorUnconfirmedKey(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	accessValidator, err := workloadproxy.NewSignatureAccessValidator(authenticator.New(st, false), &mockRoleProvider{role: role.Admin}, zaptest.NewLogger(t))
	require.NoError(t, err)

	key, err := pgp.GenerateKey("test", "", "owner@example.com", 8*time.Hour)
	require.NoError(t, err)

	armored, err := key.ArmorPublic()
	require.NoError(t, err)

	user := auth.NewUser("owner-user-id")
	user.TypedSpec().Value.Role = string(role.Admin)

	require.NoError(t, st.Create(ctx, user))

	publicKey := auth.NewPublicKey("unconfirmed-key-id")
	publicKey.Metadata().Labels().Set(auth.LabelPublicKeyUserID, user.Metadata().ID())

	publicKey.TypedSpec().Value.PublicKey = []byte(armored)
	publicKey.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(8 * time.Hour))
	publicKey.TypedSpec().Value.Confirmed = false //nolint:staticcheck
	publicKey.TypedSpec().Value.Identity = &specs.Identity{Email: "owner@example.com"}

	require.NoError(t, st.Create(ctx, publicKey))

	signature, err := key.Sign([]byte(publicKey.Metadata().ID()))
	require.NoError(t, err)

	err = accessValidator.ValidateAccess(ctx, publicKey.Metadata().ID(), base64.StdEncoding.EncodeToString(signature), "test-cluster")
	require.Error(t, err)
}
