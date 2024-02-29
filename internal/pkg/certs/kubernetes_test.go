// Copyright (c) 2024 Sidero Labs, Inc.
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
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/certs"
)

func TestIsKubernetesCertificateStale(t *testing.T) {
	bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	require.NoError(t, err)

	data, err := json.Marshal(bundle)
	require.NoError(t, err)

	secrets := omni.NewClusterSecrets("", "my-first-cluster")
	secrets.TypedSpec().Value.Data = data

	lbConfig := omni.NewLoadBalancerConfig("", "")
	lbConfig.TypedSpec().Value.SiderolinkEndpoint = "https://[2001:db8::1]:6443"

	kubeconfig, err := certs.GenerateKubeconfig(secrets, lbConfig, constants.CertificateValidityTime)
	require.NoError(t, err)

	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	require.NoError(t, err)

	// cert should be fresh with default validity time
	stale, err := certs.IsPEMEncodedCertificateStale(cfg.CertData, constants.CertificateValidityTime)
	require.NoError(t, err)

	require.False(t, stale)

	// cert should be stale with double validity time
	stale, err = certs.IsPEMEncodedCertificateStale(cfg.CertData, 2*constants.CertificateValidityTime)
	require.NoError(t, err)

	require.True(t, stale)
}
