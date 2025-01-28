// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package handler_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

type mockSignerVerifier struct {
	appendSignature []byte
}

func (mock mockSignerVerifier) Fingerprint() string {
	return "mock-fingerprint"
}

func (mock mockSignerVerifier) Sign(data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)

	return []byte(hex.EncodeToString(append(hash[:], mock.appendSignature...))), nil
}

func (mock mockSignerVerifier) Verify(data, signature []byte) error {
	expected, _ := mock.Sign(data) //nolint:errcheck

	if !bytes.Equal(signature, expected) {
		return errors.New("invalid signature")
	}

	return nil
}
