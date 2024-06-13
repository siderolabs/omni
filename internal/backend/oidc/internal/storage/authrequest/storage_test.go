// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authrequest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/siderolabs/omni/internal/backend/oidc/internal/storage/authrequest"
)

func TestStorage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s := authrequest.NewStorage()

	// create
	req, err := s.CreateAuthRequest(ctx,
		&oidc.AuthRequest{
			ClientID: "test",
		}, "")
	require.NoError(t, err)

	assert.Equal(t, "test", req.GetClientID())
	assert.False(t, req.Done())

	// get by ID
	req2, err := s.AuthRequestByID(ctx, req.GetID())
	require.NoError(t, err)

	assert.Equal(t, req, req2)

	_, err = s.AuthRequestByID(ctx, "invalid")
	assert.Error(t, err)

	// attach code, get by code
	err = s.SaveAuthCode(ctx, req.GetID(), "code")
	require.NoError(t, err)

	req3, err := s.AuthRequestByCode(ctx, "code")
	require.NoError(t, err)

	assert.Equal(t, req, req3)

	_, err = s.AuthRequestByCode(ctx, "invalid")
	assert.Error(t, err)

	// authenticate request
	err = s.AuthenticateRequest(req.GetID(), "test@example.com")
	require.NoError(t, err)

	req, err = s.AuthRequestByID(ctx, req.GetID())
	require.NoError(t, err)

	assert.True(t, req.Done())
	assert.Equal(t, "test@example.com", req.GetSubject())

	// delete
	err = s.DeleteAuthRequest(ctx, req.GetID())
	require.NoError(t, err)

	_, err = s.AuthRequestByID(ctx, req.GetID())
	assert.Error(t, err)

	_, err = s.AuthRequestByCode(ctx, "code")
	assert.Error(t, err)
}
