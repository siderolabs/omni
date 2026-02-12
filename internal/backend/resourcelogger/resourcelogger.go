// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package resourcelogger provides a logger for resource updates.
package resourcelogger

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/omni/client/pkg/diff"
)

type eventHandler func(event state.Event) error

// Logger logs resource updates.
type Logger struct {
	state        state.State
	eventCh      chan state.Event
	eventHandler eventHandler
	types        map[resource.Type]*meta.ResourceDefinition
}

// New creates a new resource logger. It logs diffs of resource updates on the given types using the given logger with the given log level.
func New(ctx context.Context, st state.State, logger *zap.Logger, level string, typeNames ...string) (*Logger, error) {
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level: %w", err)
	}

	types, err := resolveResourceTypes(ctx, st, typeNames)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource types: %w", err)
	}

	return &Logger{
		types:        types,
		state:        st,
		eventCh:      make(chan state.Event),
		eventHandler: loggingEventHandler(logger, lvl, types),
	}, nil
}

func loggingEventHandler(logger *zap.Logger, lvl zapcore.Level, types map[resource.Type]*meta.ResourceDefinition) eventHandler {
	return func(event state.Event) error {
		switch event.Type { //nolint:exhaustive
		case state.Created, state.Updated, state.Destroyed:
		default:
			return nil
		}

		sensitivity := types[event.Resource.Metadata().Type()].TypedSpec().Sensitivity

		// log the diff only if this is an update event and the resource is not sensitive
		logDiff := event.Type == state.Updated && sensitivity != meta.Sensitive

		fields := []zap.Field{
			zap.String("resource", event.Resource.Metadata().String()),
		}

		if logDiff {
			oldYAML, err := resourceAsYAML(event.Old)
			if err != nil {
				return fmt.Errorf("failed to convert old resource to YAML: %w", err)
			}

			newYAML, err := resourceAsYAML(event.Resource)
			if err != nil {
				return fmt.Errorf("failed to convert new resource to YAML: %w", err)
			}

			diffStr, err := diff.Compute([]byte(oldYAML), []byte(newYAML))
			if err != nil {
				return fmt.Errorf("failed to compute diff: %w", err)
			}

			if diffStr != "" {
				resStr := resource.String(event.Old)
				diffStr = fmt.Sprintf("--- %s\n+++ %s\n%s", resStr, resStr, diffStr)
			}

			diffLines := strings.Split(diffStr, "\n")

			fields = append(fields, zap.Strings("diff", diffLines))
		}

		logger.Log(lvl, "resource "+strings.ToLower(event.Type.String()), fields...)

		return nil
	}
}

func resourceAsYAML(res resource.Resource) (string, error) {
	resYAML, err := resource.MarshalYAML(res)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resource to YAML: %w", err)
	}

	yamlBytes, err := yaml.Marshal(resYAML)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resource to YAML bytes: %w", err)
	}

	return string(yamlBytes), nil
}

// StartWatches starts the watches for all resource types and returns.
func (l *Logger) StartWatches(ctx context.Context) error {
	for _, resType := range l.types {
		md := resource.NewMetadata(resType.TypedSpec().DefaultNamespace, resType.TypedSpec().Type, "", resource.VersionUndefined)

		if err := l.state.WatchKind(ctx, md, l.eventCh); err != nil {
			return fmt.Errorf("failed to watch resource %q: %w", resType.Metadata(), err)
		}
	}

	return nil
}

// StartLogger starts the logger and blocks until the context is canceled.
func (l *Logger) StartLogger(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-l.eventCh:
			if event.Type == state.Errored {
				return fmt.Errorf("watch errored: %w", event.Error)
			}

			if err := l.eventHandler(event); err != nil {
				return err
			}
		}
	}
}

// Start starts the watches, then starts the logger and blocks until the context is canceled.
func (l *Logger) Start(ctx context.Context) error {
	if err := l.StartWatches(ctx); err != nil {
		return err
	}

	return l.StartLogger(ctx)
}

func resolveResourceTypes(ctx context.Context, st state.State, resourceTypes []string) (map[resource.Type]*meta.ResourceDefinition, error) {
	namesToDefinitions, err := resourceNamesToDefinitions(ctx, st)
	if err != nil {
		return nil, err
	}

	definitions := make(map[resource.Type]*meta.ResourceDefinition, len(resourceTypes))

	for _, resourceType := range resourceTypes {
		name := strings.ToLower(resourceType)

		resDefinitions, ok := namesToDefinitions[name]
		if !ok {
			return nil, fmt.Errorf("resource type %q is not registered", resourceType)
		}

		if len(resDefinitions) > 1 {
			return nil, fmt.Errorf("resource type %q is ambiguous: %q", resourceType, maps.Keys(resDefinitions))
		}

		for resourceType, definition := range resDefinitions {
			definitions[resourceType] = definition

			break
		}
	}

	return definitions, nil
}

func resourceNamesToDefinitions(ctx context.Context, st state.State) (map[string]map[resource.Type]*meta.ResourceDefinition, error) {
	rds, err := safe.StateListAll[*meta.ResourceDefinition](ctx, st)
	if err != nil {
		return nil, fmt.Errorf("failed to list resource definitions: %w", err)
	}

	nameToRDs := make(map[string]map[resource.ID]*meta.ResourceDefinition, rds.Len()*3)

	add := func(name string, rd *meta.ResourceDefinition) {
		name = strings.ToLower(name)

		if _, ok := nameToRDs[name]; !ok {
			nameToRDs[name] = make(map[resource.ID]*meta.ResourceDefinition)
		}

		nameToRDs[name][rd.TypedSpec().Type] = rd
	}

	for rd := range rds.All() {
		add(rd.Metadata().ID(), rd)

		for _, alias := range rd.TypedSpec().AllAliases {
			add(alias, rd)
		}
	}

	return nameToRDs, nil
}
