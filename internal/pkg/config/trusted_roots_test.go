// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/config"
)

func TestReadTrustedRoots(t *testing.T) {
	t.Parallel()

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.ECDSA(true))
	require.NoError(t, err)

	key, err := x509.NewRSAKey()
	require.NoError(t, err)

	for _, tt := range []struct {
		name        string
		content     string
		expected    string
		expectError string
	}{
		{name: "one certificate", content: string(ca.CrtPEM), expected: string(ca.CrtPEM)},
		{name: "two certificates", content: string(ca.CrtPEM) + string(ca.CrtPEM), expected: string(ca.CrtPEM) + string(ca.CrtPEM)},
		{
			name:     "text, blank lines and CRLF around the certificates are dropped",
			content:  "# corp root\r\n\r\n" + strings.ReplaceAll(string(ca.CrtPEM), "\n", "\r\n") + "\n\n# end of bundle\n",
			expected: string(ca.CrtPEM),
		},
		{name: "empty", content: "", expectError: "holds no PEM certificates"},
		{name: "private key", content: string(key.KeyPEM), expectError: `holds a "RSA PRIVATE KEY" block`},
		{name: "broken certificate", content: "-----BEGIN CERTIFICATE-----\nYnJva2Vu\n-----END CERTIFICATE-----\n", expectError: "holds an invalid certificate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "ca.pem")
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o600))

			trustedRoots, readErr := (&config.Params{TrustedRootsFile: &path}).ReadTrustedRoots()
			if tt.expectError != "" {
				require.ErrorContains(t, readErr, tt.expectError)

				return
			}

			require.NoError(t, readErr)
			require.Equal(t, tt.expected, trustedRoots)
		})
	}

	trustedRoots, err := (&config.Params{}).ReadTrustedRoots()
	require.NoError(t, err)
	require.Empty(t, trustedRoots)
}
