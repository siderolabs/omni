// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package grpczap provides zap-based logging interceptors for gRPC servers.
//
// It is a trimmed-down version of the "logging/zap" package from
// github.com/grpc-ecosystem/go-grpc-middleware v1, which was
// removed in v2.
package grpczap

import (
	"context"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var defaultOptions = &options{
	levelFunc:       DefaultCodeToLevel,
	shouldLog:       func(string, error) bool { return true },
	codeFunc:        status.Code,
	durationFunc:    DefaultDurationToField,
	messageFunc:     DefaultMessageProducer,
	timestampFormat: time.RFC3339,
}

type options struct {
	levelFunc       CodeToLevel
	shouldLog       func(fullMethod string, err error) bool
	codeFunc        func(err error) codes.Code
	durationFunc    DurationToField
	messageFunc     MessageProducer
	timestampFormat string
}

func evaluateServerOpt(opts []Option) *options {
	optCopy := &options{}
	*optCopy = *defaultOptions

	for _, opt := range opts {
		opt(optCopy)
	}

	return optCopy
}

// Option customizes zap gRPC logging.
type Option func(*options)

// CodeToLevel maps gRPC codes to zap log levels.
type CodeToLevel func(code codes.Code) zapcore.Level

// DurationToField maps a request duration to a zap field.
type DurationToField func(duration time.Duration) zapcore.Field

// MessageProducer emits the final gRPC log entry.
type MessageProducer func(ctx context.Context, msg string, level zapcore.Level, code codes.Code, err error, duration zapcore.Field)

// WithLevels customizes the code-to-level mapping.
func WithLevels(f CodeToLevel) Option {
	return func(o *options) {
		o.levelFunc = f
	}
}

// WithCodes customizes the error-to-code mapping.
func WithCodes(f func(err error) codes.Code) Option {
	return func(o *options) {
		o.codeFunc = f
	}
}

// WithDurationField customizes the duration field.
func WithDurationField(f DurationToField) Option {
	return func(o *options) {
		o.durationFunc = f
	}
}

// WithMessageProducer customizes final log emission.
func WithMessageProducer(f MessageProducer) Option {
	return func(o *options) {
		o.messageFunc = f
	}
}

// WithTimestampFormat customizes grpc.start_time formatting.
func WithTimestampFormat(format string) Option {
	return func(o *options) {
		o.timestampFormat = format
	}
}

// DefaultCodeToLevel maps server-side gRPC return codes to zap levels.
func DefaultCodeToLevel(code codes.Code) zapcore.Level {
	switch code {
	case codes.OK, codes.Canceled, codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.Unauthenticated:
		return zap.InfoLevel
	case codes.DeadlineExceeded, codes.PermissionDenied, codes.ResourceExhausted, codes.FailedPrecondition, codes.Aborted, codes.OutOfRange,
		codes.Unavailable:
		return zap.WarnLevel
	case codes.Unknown, codes.Unimplemented, codes.Internal, codes.DataLoss:
		return zap.ErrorLevel
	default:
		return zap.ErrorLevel
	}
}

// DefaultDurationToField is the default duration field implementation.
var DefaultDurationToField = DurationToTimeMillisField

// DurationToTimeMillisField converts duration to milliseconds.
func DurationToTimeMillisField(duration time.Duration) zapcore.Field {
	return zap.Float32("grpc.time_ms", float32(duration.Nanoseconds()/1000)/1000)
}
