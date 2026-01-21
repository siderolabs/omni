// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package blocks_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/siderolabs/gen/xtesting/check"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/blocks"
)

const (
	encryptionKey = "12345678901234567890123456789012"
)

func TestEncryptDecrypt(t *testing.T) {
	//nolint:govet
	type args struct {
		key []byte
		rdr io.Reader
	}

	tests := map[string]struct {
		args    args
		wantDst string
	}{
		"normal": {
			args: args{
				key: []byte(encryptionKey),
				rdr: strings.NewReader("Sidero"),
			},
			wantDst: "Sidero",
		},
		"normal double eof": {
			args: args{
				key: []byte(encryptionKey),
				rdr: makeReaderChain(
					func(p []byte) (n int, err error) { return copy(p, "Sidero"), io.EOF },
					func([]byte) (n int, err error) { return 0, io.EOF },
				),
			},
			wantDst: "Sidero",
		},
		"normal big": {
			args: args{
				key: []byte(encryptionKey),
				rdr: strings.NewReader(strings.Repeat("Sidero", 6_000_000)),
			},
			wantDst: strings.Repeat("Sidero", 6_000_000),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var encrypted bytes.Buffer

			writer := must.Value(blocks.MakeEncrypter(&encrypted, tt.args.key))(t)

			must.Value(io.Copy(writer, tt.args.rdr))(t)
			require.NoError(t, writer.Close())

			rdr := must.Value(blocks.MakeDecrypter(&encrypted, tt.args.key))(t)

			require.Equal(t, tt.wantDst, string(must.Value(io.ReadAll(rdr))(t)))
		})
	}
}

func TestEncrypt_Errors(t *testing.T) {
	old := blocks.NonceReader

	//nolint:govet
	type args struct {
		key []byte
		rdr io.Reader
	}

	signal := false

	tests := map[string]struct {
		args      args
		writer    io.Writer
		firstErr  check.Check
		secondErr check.Check
		before    func()
	}{
		"no nonce": {
			args: args{
				key: []byte(encryptionKey),
				rdr: strings.NewReader("Sidero"),
			},
			writer:   io.Discard,
			firstErr: check.ErrorContains("failed to generate nonce"),
			before:   func() { blocks.NonceReader = makeReaderChain(func([]byte) (int, error) { return 0, io.EOF }) },
		},
		"wrong nonce": {
			args: args{
				key: []byte(encryptionKey),
				rdr: strings.NewReader("Sidero"),
			},
			writer:   io.Discard,
			firstErr: check.ErrorContains("failed to generate nonce"),
			before: func() {
				blocks.NonceReader = makeReaderChain(
					func([]byte) (int, error) { return 15, nil },
					func([]byte) (int, error) { return 0, io.EOF },
				)
			},
		},
		"wrong key": {
			args: args{
				key: []byte("1234567890123456789012345678901"),
				rdr: strings.NewReader("Sidero"),
			},
			writer:   io.Discard,
			firstErr: check.ErrorContains("encryption key must be 32 bytes"),
		},
		"generator with error": {
			args: args{
				key: []byte(encryptionKey),
				rdr: makeReaderChain(
					func(p []byte) (n int, err error) { return copy(p, "Sidero"), nil },
					func([]byte) (n int, err error) { return 0, errors.New("test error") },
				),
			},
			writer:    io.Discard,
			firstErr:  check.NoError(),
			secondErr: check.ErrorContains("test error"),
		},
		"writter with error": {
			args: args{
				key: []byte(encryptionKey),
				rdr: makeReaderChain(
					func(p []byte) (n int, err error) { return copy(p, "Sidero"), nil },
					func([]byte) (n int, err error) { return 0, io.EOF },
				),
			},
			writer:   makeWriterChain(func([]byte) (n int, err error) { return 0, errors.New("writer error") }),
			firstErr: check.ErrorContains("failed to create encrypter: failed to write header"),
		},
		"writter with second error": {
			args: args{
				key: []byte(encryptionKey),
				rdr: makeReaderChain(
					func(p []byte) (n int, err error) { signal = true; return copy(p, "Sidero"), nil }, //nolint:nlreturn
					func([]byte) (n int, err error) { return 0, io.EOF },
				),
			},
			writer: makeWriterChain(
				func(p []byte) (n int, err error) {
					if signal {
						return 0, errors.New("writer error")
					}

					return len(p), nil
				},
			),
			firstErr:  check.NoError(),
			secondErr: check.ErrorContains("writer error"),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() { blocks.NonceReader = old }()

			if tt.before != nil {
				tt.before()
			}

			writer, err := blocks.MakeEncrypter(tt.writer, tt.args.key)
			tt.firstErr(t, err)

			if writer != nil {
				tt.secondErr(t, copyNClose(writer, tt.args.rdr))
			}
		})
	}
}

func copyNClose(closer io.WriteCloser, rdr io.Reader) error {
	_, err := io.Copy(closer, rdr)
	if err != nil {
		return err
	}

	return closer.Close()
}

func makeReaderChain(fn ...func(p []byte) (n int, err error)) io.Reader {
	return &reader{fn: func(p []byte) (n int, err error) { return pop(&fn)(p) }}
}

type reader struct {
	fn func(p []byte) (n int, err error)
}

func (r *reader) Read(p []byte) (n int, err error) { return r.fn(p) }

func makeWriterChain(fn ...func(p []byte) (n int, err error)) io.Writer {
	return &writer{fn: func(p []byte) (n int, err error) { return pop(&fn)(p) }}
}

type writer struct {
	fn func(p []byte) (n int, err error)
}

func (w *writer) Write(p []byte) (n int, err error) { return w.fn(p) }

// pop pops first element from slice and returns it. If it is the last element, it returns it without changing slice.
func pop[T any](elems *[]T) T {
	if len(*elems) == 1 {
		return (*elems)[0]
	}

	elem := (*elems)[0]
	*elems = (*elems)[1:]

	return elem
}
