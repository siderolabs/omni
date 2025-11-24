// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package logstore

import (
	"context"
	"io"
)

type LogStore interface {
	WriteLine(ctx context.Context, p []byte) error

	// Reader returns a reader starting from N lines before the end.
	// If nLines <= 0, it reads from the very beginning.
	// If follow is true, ReadLine() blocks instead of returning EOF when catching up.
	Reader(nLines int, follow bool) (LineReader, error)

	io.Closer
}

type LineReader interface {
	// ReadLine reads the next message.
	//
	// Returns io.EOF if follow=false and end is reached.
	//
	// Blocks if follow=true and end is reached.
	ReadLine(ctx context.Context) ([]byte, error)

	io.Closer
}
