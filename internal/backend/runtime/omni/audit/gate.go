// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package audit

import (
	"context"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/pair"
)

// Check is a function that checks if the event is allowed.
type Check = func(ctx context.Context, eventType EventType, args ...any) bool

// Gate is a gate that checks if the event is allowed.
//
//nolint:govet
type Gate struct {
	mu  sync.RWMutex
	fns [10]map[resource.Type]Check
}

// Check checks if the event is allowed.
func (g *Gate) Check(ctx context.Context, eventType EventType, typ resource.Type, args ...any) bool {
	fn := g.check(eventType, typ)
	if fn == nil {
		return false
	}

	return fn(ctx, eventType, args...)
}

func (g *Gate) check(eventType EventType, typ resource.Type) Check {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.fns[0] == nil {
		return nil
	}

	for i, e := range allEvents {
		if eventType == e.typ {
			return g.fns[i][typ]
		}
	}

	return nil
}

// AddChecks adds checks for the event types. It's allowed to pass several at once using bitwise OR.
func (g *Gate) AddChecks(eventTypes EventType, pairs []pair.Pair[resource.Type, Check]) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.fns[0] == nil {
		for i := range g.fns {
			g.fns[i] = map[resource.Type]Check{}
		}
	}

	for _, p := range pairs {
		g.addCheck(eventTypes, p)
	}
}

func (g *Gate) addCheck(eventTypes EventType, p pair.Pair[resource.Type, Check]) {
	for i, e := range allEvents {
		if e.typ&eventTypes != 0 {
			if _, ok := g.fns[i][p.F1]; ok {
				panic("duplicate check")
			}

			g.fns[i][p.F1] = p.F2
		}
	}
}

// AllowAll is a check that allows all events for certain event type.
func AllowAll(context.Context, EventType, ...any) bool {
	return true
}

const (
	// EventGet is the get event type.
	EventGet EventType = 1 << iota
	// EventList is the list event type.
	EventList
	// EventCreate is the create event type.
	EventCreate
	// EventUpdate is the update event type.
	EventUpdate
	// EventDestroy is the destroy event type.
	EventDestroy
	// EventWatch is the watch event type.
	EventWatch
	// EventWatchKind is the watch kind event type.
	EventWatchKind
	// EventWatchKindAggregated is the watch kind aggregated event type.
	EventWatchKindAggregated
	// EventUpdateWithConflicts is the update with conflicts event type.
	EventUpdateWithConflicts
	// EventWatchFor is the watch for event type.
	EventWatchFor
)

// EventType represents the type of event.
type EventType int

// MarshalJSON marshals the event type to JSON.
func (e *EventType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + e.String() + `"`), nil
}

// String returns the string representation of the event type.
func (e *EventType) String() string {
	for _, ev := range allEvents {
		if *e == ev.typ {
			return ev.str
		}
	}

	return "<unknown>"
}

var allEvents = []struct {
	str string
	typ EventType
}{
	{"get", EventGet},
	{"list", EventList},
	{"create", EventCreate},
	{"update", EventUpdate},
	{"destroy", EventDestroy},
	{"watch", EventWatch},
	{"watch_kind", EventWatchKind},
	{"watch_kind_aggregated", EventWatchKindAggregated},
	{"update_with_conflicts", EventUpdateWithConflicts},
	{"watch_for", EventWatchFor},
}
