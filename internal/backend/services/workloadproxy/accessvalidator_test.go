// Copyright (c) 2025 Sidero Labs, Inc.
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

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/internal/backend/services/workloadproxy"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
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

	accessValidator, err := workloadproxy.NewSignatureAccessValidator(st, roleProvider, zaptest.NewLogger(t))
	require.NoError(t, err)

	key, err := pgp.GenerateKey("test", "", "test@example.com", 8*time.Hour)
	require.NoError(t, err)

	publicKey := auth.NewPublicKey(resources.DefaultNamespace, "test-public-key-id")

	armored, err := key.ArmorPublic()
	require.NoError(t, err)

	publicKey.TypedSpec().Value.PublicKey = []byte(armored)
	publicKey.TypedSpec().Value.Expiration = timestamppb.New(time.Now().Add(8 * time.Hour))

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
