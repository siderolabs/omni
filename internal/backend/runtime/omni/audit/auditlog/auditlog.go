// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package auditlog provides an interface for writing audit logs and getting readers to read them.
package auditlog

import (
	"io"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
)

type Reader interface {
	io.Closer
	Read() ([]byte, error)
}

// EventType represents the type of audit log event.
type EventType int

const (
	EventTypeUnspecified         EventType = iota
	EventTypeCreate                        // "create"
	EventTypeUpdate                        // "update"
	EventTypeUpdateWithConflicts           // "update_with_conflicts"
	EventTypeDestroy                       // "destroy"
	EventTypeTeardown                      // "teardown"
	EventTypeTalosAccess                   // "talos_access"
	EventTypeK8SAccess                     // "k8s_access"
)

// SQLString returns the string stored in the database for this event type.
func (e EventType) SQLString() string {
	switch e {
	case EventTypeUnspecified:
		return ""
	case EventTypeCreate:
		return "create"
	case EventTypeUpdate:
		return "update"
	case EventTypeUpdateWithConflicts:
		return "update_with_conflicts"
	case EventTypeDestroy:
		return "destroy"
	case EventTypeTeardown:
		return "teardown"
	case EventTypeTalosAccess:
		return "talos_access"
	case EventTypeK8SAccess:
		return "k8s_access"
	}

	return ""
}

// OrderByField represents the audit log field to sort by.
type OrderByField int

const (
	OrderByFieldUnspecified OrderByField = iota
	OrderByFieldDate
	OrderByFieldEventType
	OrderByFieldResourceType
	OrderByFieldResourceID
	OrderByFieldClusterID
	OrderByFieldActor
)

// OrderByDir represents the sort direction for audit log queries.
type OrderByDir int

const (
	OrderByDirUnspecified OrderByDir = iota
	OrderByDirASC
	OrderByDirDESC
)

// ReadFilters holds optional filters for reading audit log events.
type ReadFilters struct {
	Start        time.Time
	End          time.Time
	Search       string
	ResourceType string
	ResourceID   string
	ClusterID    string
	Actor        string
	EventType    EventType
	OrderByField OrderByField
	OrderByDir   OrderByDir
}

type Event struct {
	Data         *Data         `json:"event_data,omitempty"`
	Type         string        `json:"event_type,omitempty"`
	ResourceType resource.Type `json:"resource_type,omitempty"`
	ResourceID   string        `json:"resource_id,omitempty"`
	TimeMillis   int64         `json:"event_ts,omitempty"`
}

func MakeEvent(eventType string, resType resource.Type, resID resource.ID, data *Data) Event {
	return Event{
		Type:         eventType,
		ResourceType: resType,
		ResourceID:   resID,
		TimeMillis:   time.Now().UnixMilli(),
		Data:         data,
	}
}
