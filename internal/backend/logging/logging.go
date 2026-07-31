// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package logging contains zap logging helpers.
package logging

import (
	"slices"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Component returns the well-known "component" zap field.
func Component(name string) zap.Field {
	return zap.String("component", name)
}

// IncreaseLevel raises the logger's minimum level to lvl.
//
// Unlike zap.IncreaseLevel, it is a no-op when the underlying core is already
// at or above lvl, instead of failing and printing an error to stderr.
func IncreaseLevel(logger *zap.Logger, lvl zapcore.Level) *zap.Logger {
	if logger.Level() >= lvl {
		return logger
	}

	return logger.WithOptions(zap.IncreaseLevel(lvl))
}

// DropMessagesContaining discards entries whose message contains any of the given substrings.
func DropMessagesContaining(logger *zap.Logger, substrings ...string) *zap.Logger {
	if len(substrings) == 0 {
		return logger
	}

	return logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return &dropMessagesCore{Core: core, substrings: substrings}
	}))
}

type dropMessagesCore struct {
	zapcore.Core

	substrings []string
}

func (c *dropMessagesCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if slices.ContainsFunc(c.substrings, func(substring string) bool { return strings.Contains(ent.Message, substring) }) {
		return ce
	}

	return c.Core.Check(ent, ce)
}

// With has to rewrap, otherwise a logger.With call inside the library escapes the filter.
func (c *dropMessagesCore) With(fields []zapcore.Field) zapcore.Core {
	return &dropMessagesCore{Core: c.Core.With(fields), substrings: c.substrings}
}
