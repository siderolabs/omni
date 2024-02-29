// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package blocks

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"filippo.io/age"
	"golang.org/x/crypto/nacl/secretbox"
)

// MakeDecrypter decrypts data from reader using the provided key.
func MakeDecrypter(r io.Reader, key []byte) (io.Reader, error) {
	identity, err := newNaclIdentity(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}

	decrypter, err := age.Decrypt(r, identity)
	if err != nil {
		return nil, fmt.Errorf("failed to create decrypter: %w", err)
	}

	return decrypter, nil
}

// naclIdentity implements age.Identity interface. Acts as actual header of the encrypted file.
type naclIdentity struct {
	encryptionKey [keyLen]byte
}

func newNaclIdentity(encryptionKey []byte) (*naclIdentity, error) {
	if len(encryptionKey) != keyLen {
		return nil, errors.New("encryption key must be 32 bytes")
	}

	return &naclIdentity{encryptionKey: [keyLen]byte(encryptionKey)}, nil
}

func (n *naclIdentity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	for _, stanza := range stanzas {
		if stanza.Type != typeString || len(stanza.Args) != 1 {
			continue
		}

		nonce, err := hex.DecodeString(stanza.Args[0])
		if err != nil {
			return nil, fmt.Errorf("failed to decode nonce: %w", err)
		}

		if len(nonce) != nonceLen {
			return nil, errors.New("nonce must be 24 bytes")
		}

		decrypted, ok := secretbox.Open(nil, stanza.Body, (*[nonceLen]byte)(nonce), &n.encryptionKey)
		if !ok {
			return nil, errors.New("failed to decrypt file key")
		}

		return decrypted, nil
	}

	return nil, errors.New("no matching stanza found")
}

const (
	keyLen     = 32
	nonceLen   = 24
	typeString = "nacl_v1"
)
