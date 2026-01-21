// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package blocks provides functions for streaming encryption and decryption of
// generated data blocks.
package blocks

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"filippo.io/age"
	"golang.org/x/crypto/nacl/secretbox"
)

// MakeEncrypter encrypts provided [io.Writer] data using age protocol.
func MakeEncrypter(dst io.Writer, key []byte) (io.WriteCloser, error) {
	nonce := make([]byte, nonceLen)

	_, err := io.ReadFull(NonceReader, nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	recipient, err := newNaclRecipient(nonce, key)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipient: %w", err)
	}

	encrypter, err := age.Encrypt(dst, recipient)
	if err != nil {
		return nil, fmt.Errorf("failed to create encrypter: %w", err)
	}

	return encrypter, nil
}

// I tried many different approaches to encrypt etcd backup in streaming mode, but each time I hit a wall with different
// problems. Creating tamper-proof encoded length of the block and file header were the most difficult parts. At the end
// I discovered that age tool from Filippo Valsorda can act as a library, and it has all the necessary functionality.
// Besides, I trust Filippo's crypto skills more than I trust mine.

// naclRecipient implements age.Recipient interface. Acts as actual header of the encrypted file.
// Interesting side effect - we can use standalone age tool to decrypt our backups.
type naclRecipient struct {
	encryptionKey [keyLen]byte
	nonce         [nonceLen]byte
}

func newNaclRecipient(nonce []byte, encryptionKey []byte) (*naclRecipient, error) {
	switch {
	case len(encryptionKey) != keyLen:
		return nil, errors.New("encryption key must be 32 bytes")
	case len(nonce) != nonceLen:
		return nil, errors.New("nonce must be 24 bytes")
	}

	return &naclRecipient{nonce: [nonceLen]byte(nonce), encryptionKey: [keyLen]byte(encryptionKey)}, nil
}

func (n *naclRecipient) Wrap(fileKey []byte) ([]*age.Stanza, error) {
	sealed := secretbox.Seal(nil, fileKey, &n.nonce, &n.encryptionKey)

	stanza := age.Stanza{
		Type: typeString,
		Args: []string{hex.EncodeToString(n.nonce[:])},
		Body: sealed,
	}

	return []*age.Stanza{&stanza}, nil
}

// NonceReader is a nonce generator. It is a variable for testing purposes.
var NonceReader = rand.Reader
