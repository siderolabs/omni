// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package fstore

import "io"

func AtomicFileWrite(dst string, r io.Reader) error { return atomicFileWrite(dst, r) }
