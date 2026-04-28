// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build sidero.tools

// Package gen renders the Omni Helm chart config values from the Omni config
// JSON schema.
//
// The schema provides field order, descriptions, and Omni's own defaults. The
// chart override file provides active chart defaults, omitted schema paths,
// commented examples, and chart-specific comment text. Generation replaces only
// the top-level config block in values.yaml so the rest of the chart values file
// remains hand-maintained.
package gen
