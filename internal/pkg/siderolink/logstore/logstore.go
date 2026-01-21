// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package logstore

import (
	"context"
	"io"
)

// LogStore is an interface for writing logs and getting readers to read them.
type LogStore interface {
	WriteLine(ctx context.Context, message []byte) error

	// Reader returns a reader starting from N lines before the end.
	//
	// If nLines < 0, it reads from the very beginning.
	//
	// If follow is true, ReadLine() blocks instead of returning EOF when catching up.
	Reader(ctx context.Context, nLines int, follow bool) (LineReader, error)

	io.Closer
}

// LineReader is an interface for reading lines from a log store.
type LineReader interface {
	io.Closer

	// ReadLine reads the next message.
	//
	// Returns io.EOF if follow=false and end is reached.
	//
	// Blocks if follow=true and end is reached, until a new line is available or context is done.
	ReadLine(ctx context.Context) ([]byte, error)
}
