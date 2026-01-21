// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

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
