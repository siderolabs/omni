// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package s3store

import (
	"io"

	"golang.org/x/time/rate"
)

type ReaderLimiter = readerLimiter

//nolint:revive
func NewReaderLimiter(rdr io.ReadCloser, l *rate.Limiter) *ReaderLimiter {
	return &readerLimiter{rdr: rdr, l: l}
}
