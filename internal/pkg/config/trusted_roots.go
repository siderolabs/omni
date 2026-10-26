// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// ReadTrustedRoots returns the certificates of the trusted roots file as PEM, or "" when none is configured.
// Every PEM block in the file must be a certificate. The certificates are re-encoded, so text around them and
// formatting differences never reach the machines, and only a change of the certificates themselves changes what
// every machine config carries.
func (p *Params) ReadTrustedRoots() (string, error) {
	path := p.GetTrustedRootsFile()
	if path == "" {
		return "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read the trusted roots file: %w", err)
	}

	var certs bytes.Buffer

	rest := data

	for {
		var block *pem.Block

		if block, rest = pem.Decode(rest); block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			return "", fmt.Errorf("the trusted roots file %q holds a %q block, only certificates are allowed", path, block.Type)
		}

		if _, err = x509.ParseCertificate(block.Bytes); err != nil {
			return "", fmt.Errorf("the trusted roots file %q holds an invalid certificate: %w", path, err)
		}

		certs.Write(pem.EncodeToMemory(&pem.Block{Type: block.Type, Bytes: block.Bytes}))
	}

	if certs.Len() == 0 {
		return "", fmt.Errorf("the trusted roots file %q holds no PEM certificates", path)
	}

	return certs.String(), nil
}
