// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package trustd_test

import (
	stdlibx509 "crypto/x509"
	"encoding/asn1"
	"net"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/siderolink/trustd"
)

func TestIssueServerCertificate(t *testing.T) {
	t.Parallel()

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
	require.NoError(t, err)

	serverAddr := net.ParseIP("fdae:41e4:649b:9303::1")

	cert, err := trustd.IssueServerCertificate(&x509.PEMEncodedCertificateAndKey{
		Crt: ca.CrtPEM,
		Key: ca.KeyPEM,
	}, serverAddr)
	require.NoError(t, err)

	require.NotEmpty(t, cert.Certificate)

	serverCert, err := stdlibx509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)

	assert.Equal(t, "omni", serverCert.Subject.CommonName)
	assert.True(t, serverCert.IPAddresses[0].Equal(serverAddr))
	assert.Equal(t, stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment, serverCert.KeyUsage)
	assert.Equal(t, []stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageServerAuth}, serverCert.ExtKeyUsage)
	assert.Equal(t, []asn1.ObjectIdentifier{constants.TrustdServerExtKeyUsage}, serverCert.UnknownExtKeyUsage)

	// the certificate should still pass the regular verification used by Talos versions which don't check the custom extended key usage
	roots := stdlibx509.NewCertPool()
	roots.AddCert(ca.Crt)

	_, err = serverCert.Verify(stdlibx509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageServerAuth},
	})
	require.NoError(t, err)
}
