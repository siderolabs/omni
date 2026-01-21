// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package runtime

import (
	"encoding/json"
	"fmt"

	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/omni/client/api/omni/resources"
)

// NewWatchResponseFromCOSIEvent creates new WatchResponse from COSI state.Event.
func NewWatchResponseFromCOSIEvent(response state.Event) (*resources.WatchResponse, error) {
	var (
		old, res any
		err      error
	)

	res, err = NewResource(response.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource %w", err)
	}

	if response.Old != nil {
		old, err = NewResource(response.Old)
		if err != nil {
			return nil, fmt.Errorf("failed to create resource %w", err)
		}
	}

	return NewWatchResponse(resources.EventType(response.Type+1), res, old)
}

// NewWatchResponse creates watch response from resources and event type.
func NewWatchResponse(eventType resources.EventType, res, old any) (*resources.WatchResponse, error) {
	var (
		resEncoded string
		oldEncoded string
		err        error
	)

	if old != nil {
		oldEncoded, err = MarshalJSON(old)
		if err != nil {
			return nil, err
		}
	}

	resEncoded, err = MarshalJSON(res)
	if err != nil {
		return nil, err
	}

	return &resources.WatchResponse{
		Event: &resources.Event{
			EventType: eventType,
			Resource:  resEncoded,
			Old:       oldEncoded,
		},
	}, nil
}

// MarshalJSON encodes resource as JSON using jsonpb marshaler for proto.Messages or a standard marshaler.
func MarshalJSON(res any) (string, error) {
	if marshaler, ok := res.(json.Marshaler); ok {
		marshaled, err := marshaler.MarshalJSON()
		if err != nil {
			return "", fmt.Errorf("failed to marshal resource: %w", err)
		}

		return string(marshaled), nil
	}

	if m, ok := res.(proto.Message); ok {
		opts := protojson.MarshalOptions{
			UseProtoNames:  true,
			UseEnumNumbers: true,
		}

		data, err := opts.Marshal(m)

		return string(data), err
	}

	data, err := json.Marshal(res)

	return string(data), err
}
