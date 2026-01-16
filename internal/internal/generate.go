// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package internal contains the command that generates the TS code.
package internal

//go:generate go run -tags=tools github.com/siderolabs/omni/internal/internal/tools/tsgen -out ../../frontend/src/api/resources.ts ../../,../../client/

// Generate JSON schema.
//go:generate go tool go-jsonschema --only-models --struct-name-from-title --tags=json,yaml --package=config --extra-imports -o=../pkg/config/types.generated.go ../pkg/config/schema.json

// Generate nil-safe accessors for the config fields.
//go:generate go run -tags=tools github.com/siderolabs/omni/internal/internal/tools/accessorgen --source=../pkg/config/types.generated.go --output=../pkg/config/accessors.generated.go
