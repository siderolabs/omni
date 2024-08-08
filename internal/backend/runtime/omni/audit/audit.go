// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package audit provides a state wrapper that logs audit events.
package audit

import (
	"context"
	"fmt"
	"os"
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
		createHooks:              map[resource.Type]createHook{},
		updateHooks:              map[resource.Type]updateHook{},
		destroyHooks:             map[resource.Type]destroyHook{},
		updateWithConflictsHooks: map[resource.Type]updateWithConflictsHook{},
	}, nil
}

// Log logs audit events.
//
//nolint:govet
type Log struct {
	logFile *LogFile
	logger  *zap.Logger

	mu                       sync.RWMutex
	createHooks              map[resource.Type]createHook
	updateHooks              map[resource.Type]updateHook
	destroyHooks             map[resource.Type]destroyHook
	updateWithConflictsHooks map[resource.Type]updateWithConflictsHook
}

// LogCreate logs the resource creation if there is a hook for this type.
func (l *Log) LogCreate(ctx context.Context, r resource.Resource, option ...state.CreateOption) error {
	if hook := l.createHooks[r.Metadata().Type()]; hook != nil {
		return hook(ctx, r, option...)
	}

	return nil
}

// LogUpdate logs the resource update if there is a hook for this type.
func (l *Log) LogUpdate(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	if hook := l.updateHooks[newResource.Metadata().Type()]; hook != nil {
		return hook(ctx, newResource, opts...)
	}

	return nil
}

// LogDestroy logs the resource destruction if there is a hook for this type.
func (l *Log) LogDestroy(ctx context.Context, pointer resource.Pointer, option ...state.DestroyOption) error {
	if hook := l.destroyHooks[pointer.Type()]; hook != nil {
		return hook(ctx, pointer, option...)
	}

	return nil
}

// LogUpdateWithConflicts logs the resource update with conflicts if there is a hook for this type.
func (l *Log) LogUpdateWithConflicts(ctx context.Context, newRes resource.Resource, option ...state.UpdateOption) error {
	if hook := l.updateWithConflictsHooks[newRes.Metadata().Type()]; hook != nil {
		return hook(ctx, newRes, option...)
	}

	return nil
}

type (
	createHook              func(ctx context.Context, res resource.Resource, option ...state.CreateOption) error
	updateHook              func(ctx context.Context, newRes resource.Resource, opts ...state.UpdateOption) error
	updateWithConflictsHook func(ctx context.Context, newRes resource.Resource, option ...state.UpdateOption) error
	destroyHook             func(ctx context.Context, ptr resource.Pointer, option ...state.DestroyOption) error
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
			return err
		}

		if err := l.logFile.Dump(makeEvent("create", resType, data)); err != nil {
			return fmt.Errorf("failed to write audit log for create event: %w", err)
		}

		return nil
	})
}

// ShouldLogUpdate adds an update hook to logger which informs what resource type should be logged and how to log it.
func ShouldLogUpdate[T Res](l *Log, before func(context.Context, *Data, T, ...state.UpdateOption) error, opts ...Option) {
	resType, o := resourceType[T](), toOptions(opts...)

	setHook(l, l.updateHooks, resType, func(ctx context.Context, res resource.Resource, opts ...state.UpdateOption) error {
		data := extractData(ctx, o)
		if data == nil {
			return nil
		}

		typedRes, ok := res.(T)
		if !ok {
			return fmt.Errorf("resource type %T != expected type %T passed to update hook", res, typedRes)
		}

		if err := before(ctx, data, typedRes, opts...); err != nil {
			return err
		}

		if err := l.logFile.Dump(makeEvent("update", resType, data)); err != nil {
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
			return fmt.Errorf("failed to write audit log for destroy event: %w", err)
		}

		return nil
	})
}

// ShouldLogUpdateWithConflicts adds an update with conflicts hook to logger which informs what resource type should be logged and how to log it.
func ShouldLogUpdateWithConflicts[T Res](l *Log, before func(context.Context, *Data, T, ...state.UpdateOption) error, opts ...Option) {
	resType, o := resourceType[T](), toOptions(opts...)

	setHook(l, l.updateWithConflictsHooks, resType, func(ctx context.Context, res resource.Resource, option ...state.UpdateOption) error {
		data := extractData(ctx, o)
		if data == nil {
			return nil
		}

		typedRes, ok := res.(T)
		if !ok {
			return fmt.Errorf("resource type %T != expected type %T passed to update with conflicts hook", res, typedRes)
		}

		if err := before(ctx, data, typedRes, option...); err != nil {
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

type options struct {
	userAgent     string
	newDataIfNone bool
}

// WithInternalAgent informs hook that if [audit.Data] is missing in context it should create new one with internal agent.
func WithInternalAgent() Option {
	return func(o *options) {
		o.newDataIfNone = true
		o.userAgent = "Omni-Internal-Agent"
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
