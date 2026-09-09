// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security

import "context"

// SeverityCounts is exported for testing. Its fields are already exported, so a type alias
// exposes them as-is - no constructor/getter scaffolding needed.
type SeverityCounts = severityCounts

// CountSeverities is exported for testing.
var CountSeverities = countSeverities

// OutputFlag is exported for testing. Its one field is unexported, so a constructor stands in for
// the struct literal, and Validate/Write wrap the unexported methods a type alias can't reach.
type OutputFlag = outputFlag

// NewOutputFlag is exported for testing.
func NewOutputFlag(format string) OutputFlag {
	return OutputFlag{format: format}
}

// Validate is exported for testing.
func (f outputFlag) Validate(allowed ...string) error {
	return f.validate(allowed...)
}

// Write is exported for testing.
func (f outputFlag) Write(v any) error {
	return f.write(v)
}

// RawJSON, WriteJSON, ParseReportFormat, ValidateScanOutputFormat and ShortSchematic are exported
// for testing. All are plain functions, so a var alias exposes each as-is.
var (
	RawJSON                  = rawJSON
	WriteJSON                = writeJSON
	ParseReportFormat        = parseReportFormat
	ValidateScanOutputFormat = validateScanOutputFormat
	ShortSchematic           = shortSchematic
)

// ScanResult is exported for testing. Its fields are already exported, so a type alias exposes
// them as-is.
type ScanResult = scanResult

// FetchConcurrency is exported for testing.
const FetchConcurrency = fetchConcurrency

// ErrNoArtifacts is exported for testing.
var ErrNoArtifacts = errNoArtifacts

// FetchConcurrently, SkipMissing and RequireArtifacts are exported for testing. All are generic,
// so each needs a wrapper function with its own type parameters - a var alias can't carry generics.

// FetchConcurrently is exported for testing.
func FetchConcurrently[T, R any](ctx context.Context, items []T, fetch func(context.Context, T) (R, error)) ([]R, error) {
	return fetchConcurrently(ctx, items, fetch)
}

// SkipMissing is exported for testing.
func SkipMissing[T, R any](fetch func(context.Context, T) (R, error)) func(context.Context, T) (R, error) {
	return skipMissing(fetch)
}

// RequireArtifacts is exported for testing.
func RequireArtifacts[R any](results []R, missing func(R) bool) ([]R, error) {
	return requireArtifacts(results, missing)
}
