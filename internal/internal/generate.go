// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package internal contains the command that generates the TS code.
package internal

//go:generate go run -tags=tools github.com/siderolabs/omni/internal/internal/tools/tsgen -out ../../frontend/src/api/resources.ts ../../,../../client/
