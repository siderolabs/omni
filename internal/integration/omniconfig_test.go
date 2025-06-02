// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/client"
)

// AssertOmniconfigDownload verifies getting Omni client configuration (omniconfig).
func AssertOmniconfigDownload(testCtx context.Context, client *client.Client) TestFunc {
	return func(t *testing.T) {
		req := require.New(t)

		ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
		defer cancel()

		data, err := client.Management().Omniconfig(ctx)
		req.NoError(err)
		req.NotEmpty(data)
	}
}
