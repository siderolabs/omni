// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package cosi contains common code for handling COSI API proxying.
package cosi

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"

	"github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/internal/backend/runtime"
)

// WatchLegacy starts watch on top of COSI state.
//
// WatchLegacy performs some tricks to "emulate" Bootstrapped event.
// Recent COSI versions (in Omni itself, and Talos v1.3+) support native
// 'Bootstrapped' event, so this handler is only needed for Talos < 1.3.0.
// Once Omni deprecates Talos 1.2.x, this method can be safely removed.
//
//nolint:gocognit,gocyclo,cyclop
func WatchLegacy(ctx context.Context, st state.State, md resource.Metadata, out chan<- runtime.WatchResponse, tailEvents int, queries []resource.LabelQuery) error {
	var items []*runtime.Resource

	events := make(chan state.Event)

	var err error

	listMD := resource.NewMetadata(md.Namespace(), md.Type(), "", resource.VersionUndefined)

	var opts []state.WatchKindOption //nolint:prealloc

	for _, query := range queries {
		opts = append(opts, state.WatchWithLabelQuery(resource.RawLabelQuery(query)))
	}

	if md.ID() == "" {
		err = st.WatchKind(ctx, md, events, opts...)
	} else {
		var opts []state.WatchOption
		if tailEvents != 0 {
			opts = append(opts, state.WithTailEvents(tailEvents))
		}

		err = st.Watch(ctx, md, events, opts...)
	}

	if err != nil {
		return err
	}

	created := func(ctx context.Context, r *runtime.Resource) error {
		var ev *resources.WatchResponse

		ev, err = runtime.NewWatchResponse(resources.EventType_CREATED, r, nil)
		if err != nil {
			return err
		}

		items = append(items, r)

		channel.SendWithContext(ctx, out, responseFromResource(ev, r))

		return nil
	}

	var listOpts []state.ListOption //nolint:prealloc

	for _, query := range queries {
		listOpts = append(listOpts, state.WithLabelQuery(resource.RawLabelQuery(query)))
	}

	var listResponse resource.List

	if md.ID() == "" {
		listResponse, err = st.List(ctx, listMD, listOpts...)
		if err != nil {
			return err
		}
	} else {
		res, resErr := st.Get(ctx, md)
		if resErr != nil {
			if !state.IsNotFoundError(resErr) {
				return resErr
			}
		}

		if res != nil {
			listResponse.Items = append(listResponse.Items, res)
		}
	}

	for _, item := range listResponse.Items {
		r, rErr := runtime.NewResource(item)
		if rErr != nil {
			return rErr
		}

		if rErr = created(ctx, r); rErr != nil {
			return rErr
		}
	}

	channel.SendWithContext(ctx, out, newBooststrappedResponse())

	for {
		var ev *resources.WatchResponse

		select {
		case msg := <-events:
			if _, ok := msg.Resource.(*resource.Tombstone); ok {
				continue
			}

			var r *runtime.Resource

			if msg.Resource != nil {
				r, err = runtime.NewResource(msg.Resource)
				if err != nil {
					return fmt.Errorf("failed to create resource %w", err)
				}
			}

			switch msg.Type {
			case state.Created:
				if err = created(ctx, r); err != nil {
					return err
				}
			case state.Updated:
				index := itemIndex(items, r)
				if index < 0 {
					// when tail events is specified we should never get Created event
					// so the first update should be treated as updated
					if tailEvents == 0 {
						return errors.New("failed to find an old item in the items cache")
					} else if err = created(ctx, r); err != nil {
						return err
					}

					continue
				}

				items[index] = r

				ev, err = runtime.NewWatchResponseFromCOSIEvent(msg)
				if err != nil {
					return fmt.Errorf("failed to encode resource to send back to the client %w", err)
				}

				channel.SendWithContext(ctx, out, responseFromResource(ev, r))
			case state.Destroyed:
				index := itemIndex(items, r)

				if index < 0 {
					return errors.New("failed to find an old item in the items cache")
				}

				ev, err = runtime.NewWatchResponseFromCOSIEvent(msg)
				if err != nil {
					return fmt.Errorf("failed to encode resource to send back to the client %w", err)
				}

				if items != nil {
					items = slices.Delete(items, index, index+1)
				}

				channel.SendWithContext(ctx, out, responseFromResource(ev, r))
			case state.Bootstrapped:
				// ignored, see Watch() below
			case state.Errored:
				return fmt.Errorf("watch error: %w", msg.Error)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func itemIndex(items []*runtime.Resource, r *runtime.Resource) int {
	var (
		index int
		item  *runtime.Resource
	)

	for index, item = range items {
		if item.ID == r.ID {
			break
		}
	}

	return index
}

// Watch starts watch on top of COSI state.
//
// Watch converts COSI representation into resource API version.
func Watch(ctx context.Context, st state.State, md resource.Metadata, out chan<- runtime.WatchResponse, tailEvents int, queries []resource.LabelQuery) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	events := make(chan state.Event)

	var err error

	opts := []state.WatchKindOption{
		state.WithBootstrapContents(true),
	}

	for _, query := range queries {
		opts = append(opts, state.WatchWithLabelQuery(resource.RawLabelQuery(query)))
	}

	if md.ID() == "" {
		err = st.WatchKind(ctx, md, events, opts...)
	} else {
		var opts []state.WatchOption
		if tailEvents != 0 {
			opts = append(opts, state.WithTailEvents(tailEvents))
		}

		err = st.Watch(ctx, md, events, opts...)
	}

	if err != nil {
		return err
	}

	// single-resource Watch in COSI doesn't use Bootstrapped event, as it always
	// send a single event back anyways, so mock here Bootstrapped to be compatible with expected Resource service API
	// TODO: we should probably drop bootstrapped in the frontend for single-item watches?
	var bootstrapOnce sync.Once

	bootstrapEvent := func() {
		channel.SendWithContext(ctx, out, newBooststrappedResponse())
	}

	for {
		var msg state.Event

		select {
		case msg = <-events:
		case <-ctx.Done():
			return nil
		}

		if msg.Type == state.Destroyed {
			if _, ok := msg.Resource.(*resource.Tombstone); ok {
				if md.ID() != "" {
					bootstrapOnce.Do(bootstrapEvent)
				}

				continue
			}
		}

		switch msg.Type {
		case state.Created, state.Updated, state.Destroyed:
			ev, err := runtime.NewWatchResponseFromCOSIEvent(msg)
			if err != nil {
				return err
			}

			channel.SendWithContext(ctx, out, NewResponse(msg.Resource.Metadata().ID(), msg.Resource.Metadata().Namespace(), ev, msg.Resource))
		case state.Bootstrapped:
			bootstrapOnce.Do(bootstrapEvent)
		case state.Errored:
			return fmt.Errorf("watch error: %w", msg.Error)
		}

		if md.ID() != "" {
			bootstrapOnce.Do(bootstrapEvent)
		}
	}
}

func responseFromResource(resp *resources.WatchResponse, r *runtime.Resource) runtime.WatchResponse {
	return NewResponse(
		r.Metadata.ID,
		r.Metadata.Namespace,
		resp,
		r.Resource,
	)
}

func newBooststrappedResponse() runtime.WatchResponse {
	return NewResponse(
		"",
		"",
		&resources.WatchResponse{
			Event: &resources.Event{EventType: resources.EventType_BOOTSTRAPPED},
		},
		nil,
	)
}

// NewResponse creates new watch response.
func NewResponse(id, namespace string, resp *resources.WatchResponse, res resource.Resource) runtime.WatchResponse {
	return &watchResponse{BasicResponse: runtime.NewBasicResponse(id, namespace, resp), res: res}
}

type watchResponse struct {
	res resource.Resource
	runtime.BasicResponse
}

func (w *watchResponse) Field(name string) (string, bool) {
	val, ok := w.BasicResponse.Field(name)
	if ok {
		return val, true
	}

	val, ok = runtime.ResourceField(w.res, name)
	if ok {
		return val, true
	}

	return "", false
}

func (w *watchResponse) Match(searchFor string) bool {
	return w.BasicResponse.Match(searchFor) || runtime.MatchResource(w.res, searchFor)
}
