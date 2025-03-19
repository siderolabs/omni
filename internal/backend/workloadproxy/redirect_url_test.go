// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy_test

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/workloadproxy"
)

func TestRedirectURL(t *testing.T) {
	url := "http://some-cool-service:8099/"

	validKey := []byte("so valid")

	encoded := workloadproxy.EncodeRedirectURL(url, validKey)

	decoded, err := workloadproxy.DecodeRedirectURL(encoded, validKey)

	require.NoError(t, err)
	require.Equal(t, url, decoded)

	_, err = workloadproxy.DecodeRedirectURL(encoded, []byte("so invalid"))
	require.Error(t, err)

	_, err = workloadproxy.DecodeRedirectURL("what's that", validKey)
	require.Error(t, err)

	_, err = workloadproxy.DecodeRedirectURL(base64.StdEncoding.EncodeToString([]byte("what's that|??")), validKey)
	require.Error(t, err)
}
