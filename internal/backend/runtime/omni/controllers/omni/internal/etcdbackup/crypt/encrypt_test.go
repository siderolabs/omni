// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package crypt_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/siderolabs/gen/xtesting/check"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/etcdbackup/crypt"
)

const (
	encryptionKey  = "01234567890123456789012345678901"
	aesSecret      = "some aes secret"
	secreboxSecret = "some secretbox secret"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestEncrypt(t *testing.T) {
	//nolint:govet
	type args struct {
		encryptionKey []byte
		secret        string
		aesKey        string
		data          string
	}

	tests := map[string]struct {
		args args
	}{
		"no error": {
			args: args{
				encryptionKey: []byte(encryptionKey),
				aesKey:        aesSecret,
				secret:        secreboxSecret,
				data:          "Hello World",
			},
		},
		"empty aes key and secretbox keys": {
			args: args{
				encryptionKey: []byte(encryptionKey),
				aesKey:        "",
				secret:        "",
				data:          "Hello World",
			},
		},
		"long sequence": {
			args: args{
				encryptionKey: []byte(encryptionKey),
				aesKey:        aesSecret,
				secret:        secreboxSecret,
				data:          strings.Repeat("SIDERO", 1_000_000),
			},
		},
	}

	for name, test := range tests {
		if !t.Run(name, func(t *testing.T) {
			var encryptionTarget bytes.Buffer

			err := crypt.Encrypt(&encryptionTarget, etcdbackup.EncryptionData{
				EncryptionKey:             test.args.encryptionKey,
				AESCBCEncryptionSecret:    test.args.aesKey,
				SecretboxEncryptionSecret: test.args.secret,
			}, strings.NewReader(test.args.data))

			require.NoError(t, err)

			decryptedHeader, decrypter := must.Values(crypt.Decrypt(&encryptionTarget, test.args.encryptionKey))(t)

			require.Equal(t, test.args.aesKey, decryptedHeader.AESCBCEncryptionSecret)
			require.Equal(t, test.args.secret, decryptedHeader.SecretboxEncryptionSecret)
			require.Equal(t, test.args.data, string(must.Value(io.ReadAll(decrypter))(t)))
		}) {
			return
		}
	}
}

func TestEncrypt_Errors(t *testing.T) {
	//nolint:govet
	type args struct {
		encryptionKey []byte
		rdr           io.Reader
	}

	tests := map[string]struct {
		args     args
		errCheck check.Check
	}{
		"no error": {
			args: args{
				encryptionKey: []byte(encryptionKey),
				rdr:           strings.NewReader("Hello World"),
			},
			errCheck: check.NoError(),
		},
		"invalid encryption key size": {
			args: args{
				encryptionKey: []byte(encryptionKey + "0"),
			},
			errCheck: check.ErrorContains("encryption key must be 32 bytes"),
		},
		"empty backup": {
			args: args{
				encryptionKey: []byte(encryptionKey),
				rdr:           strings.NewReader(""),
			},
			errCheck: check.ErrorContains("backup was empty"),
		},
	}

	for name, test := range tests {
		if !t.Run(name, func(t *testing.T) {
			var encryptionTarget bytes.Buffer

			err := crypt.Encrypt(&encryptionTarget, etcdbackup.EncryptionData{
				EncryptionKey:             test.args.encryptionKey,
				AESCBCEncryptionSecret:    aesSecret,
				SecretboxEncryptionSecret: secreboxSecret,
			}, test.args.rdr)

			test.errCheck(t, err)
		}) {
			return
		}
	}
}

func TestDecrypt_Errors(t *testing.T) {
	var encryptionTarget bytes.Buffer

	err := crypt.Encrypt(&encryptionTarget, etcdbackup.EncryptionData{
		EncryptionKey:             []byte(encryptionKey),
		AESCBCEncryptionSecret:    aesSecret,
		SecretboxEncryptionSecret: secreboxSecret,
	}, strings.NewReader("Hello"))

	require.NoError(t, err)

	encrypted := encryptionTarget.Bytes()

	type args struct {
		rdr           io.Reader
		encryptionKey []byte
	}

	tests := []struct {
		name     string
		errCheck check.Check
		args     args
	}{
		{
			name: "no error",
			args: args{
				encryptionKey: []byte(encryptionKey),
				rdr:           bytes.NewReader(encrypted),
			},
			errCheck: check.NoError(),
		},
		{
			name: "invalid encryption key size",
			args: args{
				encryptionKey: []byte(encryptionKey + "0"),
				rdr:           bytes.NewReader(encrypted),
			},
			errCheck: check.ErrorContains("encryption key must be 32 bytes"),
		},
		{
			name: "wrong encryption key",
			args: args{
				encryptionKey: []byte("01234567890123456789012345678902"),
				rdr:           bytes.NewReader(encrypted),
			},
			errCheck: check.ErrorContains("failed to decrypt file key"),
		},
		{
			name: "empty backup",
			args: args{
				encryptionKey: []byte(encryptionKey),
				rdr:           strings.NewReader(""),
			},
			errCheck: check.ErrorContains("failed to read intro"),
		},
		{
			name: "partial backup",
			args: args{
				encryptionKey: []byte(encryptionKey),
				rdr:           bytes.NewReader(encrypted[:len(encrypted)-1]),
			},
			errCheck: check.ErrorContains("failed to decrypt and authenticate payload chunk"),
		},
		{
			name: "failed to read len",
			args: args{
				encryptionKey: []byte(encryptionKey),
				rdr:           bytes.NewReader(encrypted[:190]),
			},
			errCheck: check.ErrorContains("failed to read len"),
		},
		// TODO: I suppose there is case where backup was truncated at exactly block boundary
		// and we will fail to decrypt it, but I don't know how to test it.
	}

	for _, tt := range tests {
		if !t.Run(tt.name, func(t *testing.T) {
			_, rdr, err := crypt.Decrypt(tt.args.rdr, tt.args.encryptionKey)
			tt.errCheck(t, err)

			if rdr != nil {
				_, err := io.ReadAll(rdr)
				tt.errCheck(t, err)
			}
		}) {
			return
		}
	}
}

func Benchmark_Encrypt(b *testing.B) {
	b.StopTimer()

	key := []byte(encryptionKey)

	b.ResetTimer()
	b.ReportAllocs()
	b.StartTimer()

	data := &repeatReader{data: []byte("SIDERO")}

	for b.Loop() {
		rdr := io.LimitReader(data, 10_000_000)

		err := crypt.Encrypt(io.Discard, etcdbackup.EncryptionData{
			EncryptionKey:             key,
			AESCBCEncryptionSecret:    aesSecret,
			SecretboxEncryptionSecret: secreboxSecret,
		}, rdr)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// repeatReader is used to generate large amount of data.
// It is not a real reader, it just copies the same data over and over again.
type repeatReader struct {
	data []byte
}

func (r *repeatReader) Read(p []byte) (n int, err error) {
	total := 0

	for {
		if len(p) == 0 {
			return total, nil
		}

		n = copy(p, r.data)
		p = p[n:]

		total += n
	}
}

func TestCrypt_Upload_Blocked(t *testing.T) {
	//nolint:govet
	tests := []struct {
		name       string
		blocked    bool
		errorCheck check.Check
	}{
		{
			name:       "not blocked",
			blocked:    false,
			errorCheck: check.NoError(),
		},
		{
			name:       "blocked",
			blocked:    true,
			errorCheck: check.ErrorContains("failed in wrapped uploader"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encryptor := crypt.NewStore(&backupBlocker{block: tt.blocked})

			ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()

			err := encryptor.Upload(ctx, etcdbackup.Description{
				EncryptionData: etcdbackup.EncryptionData{
					AESCBCEncryptionSecret:    aesSecret,
					SecretboxEncryptionSecret: secreboxSecret,
					EncryptionKey:             []byte(encryptionKey),
				},
			}, strings.NewReader("Hello World"))

			tt.errorCheck(t, err)
		})
	}
}

type backupBlocker struct {
	etcdbackup.Store
	block bool
}

func (e *backupBlocker) Upload(ctx context.Context, _ etcdbackup.Description, r io.Reader) error {
	if e.block {
		<-ctx.Done()
	}

	_, err := io.Copy(io.Discard, r)

	return err
}

func (e *backupBlocker) Download(ctx context.Context, _ []byte, _, _ string) (etcdbackup.BackupData, io.ReadCloser, error) {
	if e.block {
		<-ctx.Done()
	}

	return etcdbackup.BackupData{}, io.NopCloser(bytes.NewReader(nil)), nil
}
