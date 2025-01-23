// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package crypt

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/blocks"
)

// Encrypt encrypts etcd backup using age protocol.
func Encrypt(dst io.Writer, descr etcdbackup.EncryptionData, r io.Reader) error {
	wrt, err := blocks.MakeEncrypter(dst, descr.EncryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt backup: %w", err)
	}

	defer wrt.Close() //nolint:errcheck

	for _, data := range [...][]byte{
		ensure.Value(pointer.To(specs.EtcdBackupHeader{Version: 1}).MarshalVT()),
		[]byte(descr.AESCBCEncryptionSecret),
		[]byte(descr.SecretboxEncryptionSecret),
	} {
		err = writeWithSize(wrt, data)
		if err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	n, err := io.Copy(wrt, r)
	if err != nil {
		return fmt.Errorf("failed to copy backup: %w", err)
	}

	if n == 0 {
		return errors.New("backup was empty")
	}

	err = wrt.Close()
	if err != nil {
		return fmt.Errorf("failed to close enrypter: %w", err)
	}

	return nil
}

func writeWithSize(dst io.Writer, buf []byte) error {
	var encodedLen [encodedLen]byte

	binary.BigEndian.PutUint64(encodedLen[:], uint64(len(buf)))

	_, err := dst.Write(encodedLen[:])
	if err != nil {
		return fmt.Errorf("failed to write len: %w", err)
	}

	_, err = dst.Write(buf)
	if err != nil {
		return fmt.Errorf("failed to write buf: %w", err)
	}

	return nil
}
