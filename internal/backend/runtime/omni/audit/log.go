// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/pair"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// NewLogger creates a new audit logger.
func NewLogger(auditLogDir string, logger *zap.Logger) (*Logger, error) {
	err := os.MkdirAll(auditLogDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	return &Logger{
		logFile: NewLogFile(auditLogDir),
		logger:  logger,
	}, nil
}

// Logger logs audit events.
type Logger struct {
	gate    Gate
	logFile *LogFile
	logger  *zap.Logger
}

// LogEvent logs an audit event.
func (l *Logger) LogEvent(ctx context.Context, eventType EventType, resType resource.Type, args ...any) {
	if !l.gate.Check(ctx, eventType, resType, args...) {
		return
	}

	value, ok := ctxstore.Value[*Data](ctx)
	if !ok {
		return
	}

	err := l.logFile.Dump(&event{
		Type:         eventType,
		ResourceType: resType,
		Time:         time.Now().UnixMilli(),
		Data:         value,
	})
	if err == nil {
		return
	}

	l.logger.Error("failed to dump audit log", zap.Error(err))
}

// ShoudLog adds checks that allow event type to be logged.
func (l *Logger) ShoudLog(eventType EventType, p ...pair.Pair[resource.Type, Check]) {
	l.gate.AddChecks(eventType, p)
}

//nolint:govet
type event struct {
	Type         EventType     `json:"event_type,omitempty"`
	ResourceType resource.Type `json:"resource_type,omitempty"`
	Time         int64         `json:"event_ts,omitempty"`
	Data         *Data         `json:"event_data,omitempty"`
}
