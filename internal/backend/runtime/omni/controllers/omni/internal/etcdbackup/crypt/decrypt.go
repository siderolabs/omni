// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package crypt

import (
	"encoding/binary"
	"fmt"
	"io"
	"unsafe"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/blocks"
)

// DecryptedData contains data required to restore etcd backup.
type DecryptedData struct {
	AESCBCEncryptionSecret    string
	SecretboxEncryptionSecret string
}

// Decrypt decrypts etcd backup using age protocol.
func Decrypt(r io.Reader, key []byte) (DecryptedData, io.Reader, error) {
	decrypter, err := blocks.MakeDecrypter(r, key)
	if err != nil {
		return DecryptedData{}, nil, fmt.Errorf("failed to decrypt backup: %w", err)
	}

	header, err := readWithLen(decrypter)
	if err != nil {
		return DecryptedData{}, nil, fmt.Errorf("failed to read header: %w", err)
	}

	var hdr specs.EtcdBackupHeader

	err = hdr.UnmarshalVT(header)
	if err != nil {
		return DecryptedData{}, nil, fmt.Errorf("failed to unmarshal header: %w", err)
	}

	if hdr.Version != currentVersion {
		return DecryptedData{}, nil, fmt.Errorf("unsupported version: %d", hdr.Version)
	}

	var decryptedData DecryptedData

	raw, err := readWithLen(decrypter)
	if err != nil {
		return DecryptedData{}, nil, fmt.Errorf("failed to read aes-cbc: %w", err)
	}

	decryptedData.AESCBCEncryptionSecret = string(raw)

	raw, err = readWithLen(decrypter)
	if err != nil {
		return DecryptedData{}, nil, fmt.Errorf("failed to read secretbox: %w", err)
	}

	decryptedData.SecretboxEncryptionSecret = string(raw)

	return decryptedData, decrypter, nil
}

func readWithLen(decrypter io.Reader) ([]byte, error) {
	var encodedLenRaw [encodedLen]byte

	_, err := io.ReadFull(decrypter, encodedLenRaw[:])
	if err != nil {
		return nil, fmt.Errorf("failed to read len: %w", err)
	}

	encodedLen := binary.BigEndian.Uint64(encodedLenRaw[:])
	data := make([]byte, encodedLen)

	_, err = io.ReadFull(decrypter, data)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	return data, nil
}

const (
	encodedLen     = unsafe.Sizeof(uint64(0))
	currentVersion = 1
)
