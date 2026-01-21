// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package s3store_test

import (
	"io"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/etcdbackup/s3store"
)

func TestReaderLimiter(t *testing.T) {
	const str = "Hello Siderolabs!!"

	rdr := strings.NewReader(str)
	r := rate.NewLimiter(rate.Limit(6), 6)

	rdrLimiter := s3store.NewReaderLimiter(io.NopCloser(rdr), r)

	defer rdrLimiter.Close() //nolint:errcheck

	var builder strings.Builder

	now := time.Now()

	n, err := io.Copy(&builder, rdrLimiter)
	if err != nil {
		t.Fatalf("failed to copy: %v", err)
	} else if n != int64(len(str)) {
		t.Fatalf("expected %d bytes, got %d", len(str), n)
	}

	if builder.String() != str {
		t.Fatalf("expected %q, got %q", str, builder.String())
	}

	if e := time.Since(now); e < time.Second*2 {
		t.Fatalf("expected at least 2 seconds to pass, got %v", e)
	}
}
