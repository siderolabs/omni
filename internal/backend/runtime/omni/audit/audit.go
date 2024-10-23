// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package audit provides a state wrapper that logs audit events.
package audit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// NewLog creates a new audit logger.
func NewLog(auditLogDir string, logger *zap.Logger) (*Log, error) {
	err := os.MkdirAll(auditLogDir, 0o755)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	return &Log{
		logFile:                  NewLogFile(auditLogDir),
		logger:                   logger,
		mu:                       sync.RWMutex{},
		createHooks:              map[resource.Type]CreateHook{},
		updateHooks:              map[resource.Type]UpdateHook{},
		destroyHooks:             map[resource.Type]DestroyHook{},
		updateWithConflictsHooks: map[resource.Type]UpdateWithConflictsHook{},
	}, nil
}

// Log logs audit events.
//
//nolint:govet
type Log struct {
	logFile *LogFile
	logger  *zap.Logger

	mu                       sync.RWMutex
	createHooks              map[resource.Type]CreateHook
	updateHooks              map[resource.Type]UpdateHook
	destroyHooks             map[resource.Type]DestroyHook
	updateWithConflictsHooks map[resource.Type]UpdateWithConflictsHook
}

// ReadAuditLog reads the audit log file by file, oldest to newest within the given time range. The time range
// is inclusive, and truncated to the day.
func (l *Log) ReadAuditLog(start, end time.Time) (io.ReadCloser, error) {
	return l.logFile.ReadAuditLog(start, end)
}

// LogCreate logs the resource creation if there is a hook for this type.
func (l *Log) LogCreate(r resource.Resource) CreateHook {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.createHooks[r.Metadata().Type()]
}

// LogUpdate logs the resource update if there is a hook for this type.
func (l *Log) LogUpdate(newRes resource.Resource) UpdateHook {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.updateHooks[newRes.Metadata().Type()]
}

// LogDestroy logs the resource destruction if there is a hook for this type.
func (l *Log) LogDestroy(ptr resource.Pointer) DestroyHook {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.destroyHooks[ptr.Type()]
}

// LogUpdateWithConflicts logs the resource update with conflicts if there is a hook for this type.
func (l *Log) LogUpdateWithConflicts(ptr resource.Pointer) UpdateWithConflictsHook {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.updateWithConflictsHooks[ptr.Type()]
}

// AuditTalosAccess logs the talos access event.
func (l *Log) AuditTalosAccess(ctx context.Context, fullMethodName string, clusterID string, nodeID string) error {
	data := extractData(ctx, options{
		userAgent:     internalAgent,
		newDataIfNone: true,
	})
	if data == nil {
		return nil
	}

	if data.TalosAccess == nil {
		data.TalosAccess = &TalosAccess{}
	}

	data.TalosAccess.FullMethodName = fullMethodName
	data.TalosAccess.ClusterName = clusterID
	data.TalosAccess.MachineIP = nodeID

	return l.logFile.Dump(event{
		Type: "talos_access",
		Time: time.Now().UnixMilli(),
		Data: data,
	})
}

// Wrap wraps the http.Handler with audit logging.
func (l *Log) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == http.MethodOptions {
			next.ServeHTTP(w, req)

			return
		}

		data, ok := ctxstore.Value[*Data](req.Context())
		if !ok {
			next.ServeHTTP(w, req)

			return
		}

		if data.K8SAccess == nil {
			data.K8SAccess = &K8SAccess{}
		}

		err := l.logFile.Dump(event{
			Type: "k8s_access",
			Time: time.Now().UnixMilli(),
			Data: data,
		})
		if err != nil {
			l.logger.Error("failed to write audit log", zap.Error(err))
		}

		next.ServeHTTP(w, req)
	})
}

// RunCleanup runs [LogFile.RemoveFiles] once a minute, deleting all log files older than 30 days including
// current day.
func (l *Log) RunCleanup(ctx context.Context) error {
	for {
		if err := l.logFile.RemoveFiles(
			time.Unix(0, 0),
			time.Now().AddDate(0, 0, -30),
		); err != nil {
			l.logger.Warn("failed to cleanup old audit log files", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Minute):
		}
	}
}

type (
	// CreateHook is a hook for specific type resource creation.
	CreateHook = func(ctx context.Context, res resource.Resource, option ...state.CreateOption) error
	// UpdateHook is a hook for specific type resource update.
	UpdateHook = func(ctx context.Context, oldRes, newRes resource.Resource, opts ...state.UpdateOption) error
	// UpdateWithConflictsHook is a hook for specific type resource update with conflicts.
	UpdateWithConflictsHook = func(ctx context.Context, oldRes, newRes resource.Resource, option ...state.UpdateOption) error
	// DestroyHook is a hook for specific type resource destruction.
	DestroyHook = func(ctx context.Context, ptr resource.Pointer, option ...state.DestroyOption) error
)

//nolint:govet
type event struct {
	Type         string        `json:"event_type,omitempty"`
	ResourceType resource.Type `json:"resource_type,omitempty"`
	Time         int64         `json:"event_ts,omitempty"`
	Data         *Data         `json:"event_data,omitempty"`
}

// Res is a resource type constraint.
type Res interface {
	resource.Resource
	meta.ResourceDefinitionProvider
}

// ShouldLogCreate adds a creation hook to logger which informs what resource type should be logged and how to log it.
func ShouldLogCreate[T Res](l *Log, before func(context.Context, *Data, T, ...state.CreateOption) error, opts ...Option) {
	resType, o := resourceType[T](), toOptions(opts...)

	setHook(l, l.createHooks, resType, func(ctx context.Context, res resource.Resource, option ...state.CreateOption) error {
		data := extractData(ctx, o)
		if data == nil {
			return nil
		}

		typedRes, ok := res.(T)
		if !ok {
			return fmt.Errorf("resource type %T != expected type %T passed to create hook", res, typedRes)
		}

		if err := before(ctx, data, typedRes, option...); err != nil {
			if errors.Is(err, ErrNoLog) {
				return nil
			}

			return err
		}

		if err := l.logFile.Dump(makeEvent("create", resType, data)); err != nil {
			return fmt.Errorf("failed to write audit log for create event: %w", err)
		}

		return nil
	})
}

// ShouldLogUpdate adds an update hook to logger which informs what resource type should be logged and how to log it.
func ShouldLogUpdate[T Res](l *Log, before func(context.Context, *Data, T, T, ...state.UpdateOption) error, opts ...Option) {
	resType, o := resourceType[T](), toOptions(opts...)

	setHook(l, l.updateHooks, resType, func(ctx context.Context, oldRes, newRes resource.Resource, opts ...state.UpdateOption) error {
		oldTypedRes, ok := oldRes.(T)
		if !ok {
			return fmt.Errorf("old resource type %T != expected type %T passed to update hook", oldRes, oldTypedRes)
		}

		newTypesRed, ok := newRes.(T)
		if !ok {
			return fmt.Errorf("new resource type %T != expected type %T passed to update hook", newRes, newTypesRed)
		}

		if isEqualResource(oldTypedRes, newTypesRed) {
			return nil
		}

		data := extractData(ctx, o)
		if data == nil {
			return nil
		}

		eventType := "update"
		if newTypesRed.Metadata().Phase() == resource.PhaseTearingDown {
			eventType = "teardown"
		}

		if err := before(ctx, data, oldTypedRes, newTypesRed, opts...); err != nil {
			if errors.Is(err, ErrNoLog) {
				return nil
			}

			return err
		}

		if err := l.logFile.Dump(makeEvent(eventType, resType, data)); err != nil {
			return fmt.Errorf("failed to write audit log for update event: %w", err)
		}

		return nil
	})
}

// ShouldLogDestroy adds a destruction hook to logger which informs what resource type should be logged and how to log it.
func ShouldLogDestroy(l *Log, resType resource.Type, before func(context.Context, *Data, resource.Pointer, ...state.DestroyOption) error, opts ...Option) {
	o := toOptions(opts...)

	setHook(l, l.destroyHooks, resType, func(ctx context.Context, ptr resource.Pointer, option ...state.DestroyOption) error {
		data := extractData(ctx, o)
		if data == nil {
			return nil
		}

		if err := before(ctx, data, ptr, option...); err != nil {
			return err
		}

		if err := l.logFile.Dump(makeEvent("destroy", resType, data)); err != nil {
			if errors.Is(err, ErrNoLog) {
				return nil
			}

			return fmt.Errorf("failed to write audit log for destroy event: %w", err)
		}

		return nil
	})
}

// ShouldLogUpdateWithConflicts adds an update with conflicts hook to logger which informs what resource type should be logged and how to log it.
func ShouldLogUpdateWithConflicts[T Res](l *Log, before func(context.Context, *Data, T, T, ...state.UpdateOption) error, opts ...Option) {
	resType, o := resourceType[T](), toOptions(opts...)

	setHook(l, l.updateWithConflictsHooks, resType, func(ctx context.Context, oldRes, newRes resource.Resource, option ...state.UpdateOption) error {
		oldTypedRes, ok := oldRes.(T)
		if !ok {
			return fmt.Errorf("old resource type %T != expected type %T passed to update hook", oldRes, oldTypedRes)
		}

		newTypesRed, ok := newRes.(T)
		if !ok {
			return fmt.Errorf("new resource type %T != expected type %T passed to update hook", newRes, newTypesRed)
		}

		if isEqualResource(oldTypedRes, newTypesRed) {
			return nil
		}

		data := extractData(ctx, o)
		if data == nil {
			return nil
		}

		if err := before(ctx, data, oldTypedRes, newTypesRed, option...); err != nil {
			if errors.Is(err, ErrNoLog) {
				return nil
			}

			return err
		}

		if err := l.logFile.Dump(makeEvent("update_with_conflicts", resType, data)); err != nil {
			return fmt.Errorf("failed to write audit log for update with conflicts events: %w", err)
		}

		return nil
	})
}

func resourceType[T meta.ResourceDefinitionProvider]() resource.Type {
	var zero T

	return zero.ResourceDefinition().Type
}

func makeEvent(eventType string, resType resource.Type, data *Data) event {
	return event{
		Type:         eventType,
		ResourceType: resType,
		Time:         time.Now().UnixMilli(),
		Data:         data,
	}
}

func setHook[T any](l *Log, hooks map[resource.Type]T, resType resource.Type, hook T) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := hooks[resType]; ok {
		panic(fmt.Errorf("hook for type %s already exists", resType))
	}

	hooks[resType] = hook
}

func extractData(ctx context.Context, opts options) *Data {
	data, ok := ctxstore.Value[*Data](ctx)
	if ok {
		return data
	}

	if !opts.newDataIfNone {
		return nil
	}

	result := &Data{}

	if opts.userAgent != "" {
		result.Session.UserAgent = opts.userAgent
	}

	return result
}

func isEqualResource(oldRes, newRes resource.Resource) bool {
	if oldRes.Metadata().ID() != newRes.Metadata().ID() {
		return false
	}

	oldSpec, newSpec := oldRes.Spec(), newRes.Spec()

	if equality, ok := oldSpec.(interface{ Equal(any) bool }); ok {
		return equality.Equal(newSpec)
	}

	return reflect.DeepEqual(oldSpec, newSpec)
}

type options struct {
	userAgent     string
	newDataIfNone bool
}

const internalAgent = "Omni-Internal-Agent"

// WithInternalAgent informs hook that if [audit.Data] is missing in context it should create new one with internal agent.
func WithInternalAgent() Option {
	return func(o *options) {
		o.newDataIfNone = true
		o.userAgent = internalAgent
	}
}

// Option is a function that modifies options.
type Option func(*options)

func toOptions(o ...Option) options {
	var result options

	for _, v := range o {
		v(&result)
	}

	return result
}

// ErrNoLog is returned by hooks to indicate that the event should be ignored.
var ErrNoLog = errors.New("ignore this event")
