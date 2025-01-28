// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package logging contains zap logging helpers.
package logging

import (
	"go.uber.org/zap"
)

// Component returns the well-known "component" zap field.
func Component(name string) zap.Field {
	return zap.String("component", name)
}
