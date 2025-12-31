// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package certs_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/certs"
)

func TestIsTalosCertificateStale(t *testing.T) {
	bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	require.NoError(t, err)

	data, err := json.Marshal(bundle)
	require.NoError(t, err)

	secrets := omni.NewClusterSecrets("")
	secrets.TypedSpec().Value.Data = data

	cert, _, err := certs.TalosAPIClientCertificateFromSecrets(secrets, constants.CertificateValidityTime, role.All)
	require.NoError(t, err)

	// cert should be fresh with default validity time
	stale, err := certs.IsPEMEncodedCertificateStale(cert.Crt, constants.CertificateValidityTime)
	require.NoError(t, err)

	require.False(t, stale)

	// cert should be stale with double validity time
	stale, err = certs.IsPEMEncodedCertificateStale(cert.Crt, 2*constants.CertificateValidityTime)
	require.NoError(t, err)

	require.True(t, stale)
}
