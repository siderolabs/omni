// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package external provides an external state implementation.
package external

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/pair/ordered"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
)

// State is an external state implementation which provides external resources.
// External resources are not stored in the backend, but are generated on the fly from the things we got
// from the external storage (S3 or filesystem).
type State struct {
	CoreState    state.CoreState
	StoreFactory store.Factory
}

// Get implements [state.CoreState] interface.
func (s *State) Get(ctx context.Context, pointer resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	if pointer.Type() != omni.EtcdBackupType || pointer.Namespace() != resources.ExternalNamespace {
		return nil, makeUnsupportedError(fmt.Errorf("unsupported resource type for get %q", pointer))
	}

	if len(opts) > 0 {
		return nil, makeUnsupportedError(errors.New("no get options are supported"))
	}

	id, err := extractClusterID(pointer.ID())
	if err != nil {
		return nil, makeValidationError(err)
	}

	list, err := s.List(ctx, pointer, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, id)))
	if err != nil {
		return nil, err
	}

	if len(list.Items) == 0 {
		return nil, makeNotFoundError(fmt.Errorf("etcd backup %q for cluster %q not found", pointer.ID(), id))
	}

	typedList := safe.NewList[*omni.EtcdBackup](list)

	res, ok := typedList.Find(func(backup *omni.EtcdBackup) bool { return backup.Metadata().ID() == pointer.ID() })
	if !ok {
		return nil, makeNotFoundError(fmt.Errorf("etcd backup %q for cluster %q not found", pointer.ID(), id))
	}

	return res, nil
}

func extractClusterID(id resource.ID) (string, error) {
	idx := strings.LastIndex(id, "-")
	if idx == -1 {
		return "", fmt.Errorf("invalid ID %q", id)
	}

	_, err := strconv.Atoi(id[idx+1:])
	if err != nil {
		return "", fmt.Errorf("invalid timestamp in ID %q: %w", id, err)
	}

	return id[:idx], nil
}

// List implements [state.CoreState] interface.
func (s *State) List(ctx context.Context, kind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	if kind.Type() != omni.EtcdBackupType || kind.Namespace() != resources.ExternalNamespace {
		return resource.List{}, makeUnsupportedError(fmt.Errorf("unsupported resource kind for list %q", kind))
	}

	parsed, err := parseListOptions(opts)
	if err != nil {
		return resource.List{}, makeValidationError(err)
	}

	clusterUUID, err := safe.StateGetByID[*omni.ClusterUUID](ctx, s.CoreState, parsed.ClusterID)
	if err != nil {
		return resource.List{}, fmt.Errorf("failed to get cluster UUID by ID: %w", err)
	}

	st, err := s.StoreFactory.GetStore()
	if err != nil {
		return resource.List{}, fmt.Errorf("failed to get store: %w", err)
	}

	iter, err := st.ListBackups(ctx, clusterUUID.TypedSpec().Value.Uuid)
	if err != nil {
		return resource.List{}, fmt.Errorf("failed to list backups for cluster %q: %w", parsed.ClusterID, err)
	}

	limit := 1000
	result := make([]resource.Resource, 0, limit)

	for range limit { // limit max number of iterations to 1000 for now
		info, more, err := iter()
		if err != nil {
			return resource.List{}, fmt.Errorf("failed to get the next backup: %w", err)
		}

		if !more {
			break
		}

		etcdBackup := omni.NewEtcdBackup(clusterUUID.Metadata().ID(), info.Timestamp)
		etcdBackup.TypedSpec().Value.Snapshot = info.Snapshot
		etcdBackup.TypedSpec().Value.CreatedAt = timestamppb.New(info.Timestamp)
		etcdBackup.TypedSpec().Value.Size = uint64(info.Size)
		etcdBackup.Metadata().Labels().Set(omni.LabelClusterUUID, clusterUUID.TypedSpec().Value.Uuid)
		etcdBackup.Metadata().Labels().Set(omni.LabelCluster, clusterUUID.Metadata().ID())

		result = append(result, etcdBackup)
	}

	return resource.List{Items: result}, nil
}

func parseListOptions(opts []state.ListOption) (parsedListOptions, error) {
	opt := fromOpts(opts...)

	var result parsedListOptions

	if opt.IDQuery.Regexp != nil {
		return result, errors.New("ID query is not supported")
	}

	for _, q := range opt.LabelQueries {
		for _, t := range q.Terms {
			keyOp := ordered.MakePair(t.Key, t.Op)

			switch keyOp {
			case ordered.MakePair(omni.LabelCluster, resource.LabelOpEqual):
				if len(t.Value) == 0 || t.Value[0] == "" {
					return result, fmt.Errorf("empty value for %q is not supported", t.Key)
				}

				result.ClusterID = t.Value[0]
			default:
				return result, fmt.Errorf("unsupported label query term %v", keyOp)
			}
		}
	}

	if result.ClusterID == "" {
		return result, errors.New("cluster ID must be specified in query")
	}

	return result, nil
}

type parsedListOptions struct {
	ClusterID string
}

// WatchKind implements [state.CoreState] interface.
func (s *State) WatchKind(ctx context.Context, kind resource.Kind, ch chan<- state.Event, opts ...state.WatchKindOption) error {
	if kind.Type() != omni.EtcdBackupType || kind.Namespace() != resources.ExternalNamespace {
		return makeUnsupportedError(fmt.Errorf("unsupported resource kind for watch %q", kind))
	}

	convertedOpts, err := convertOpts(opts)
	if err != nil {
		return makeUnsupportedError(err)
	}

	go func() {
		list, err := s.List(ctx, kind, convertedOpts...)
		if err != nil {
			channel.SendWithContext(ctx, ch, state.Event{
				Error: fmt.Errorf("list error: %w", err),
				Type:  state.Errored,
			})

			return
		}

		for _, item := range list.Items {
			if !channel.SendWithContext(ctx, ch, state.Event{
				Resource: item,
				Type:     state.Created,
			}) {
				return
			}
		}

		if !channel.SendWithContext(ctx, ch, state.Event{Type: state.Bootstrapped}) {
			return
		}
	}()

	return nil
}

func convertOpts(opts []state.WatchKindOption) ([]state.ListOption, error) {
	if len(opts) == 0 {
		return nil, nil
	}

	wo := fromOpts(opts...)

	switch {
	case !wo.BootstrapContents:
		return nil, errors.New("should pass WithBootstrapContents option")
	case wo.TailEvents != 0:
		return nil, errors.New("tail events is not supported")
	case wo.IDQuery.Regexp != nil:
		return nil, errors.New("ID query is not supported")
	case wo.UnmarshalOptions.SkipProtobufUnmarshal:
		return nil, errors.New("skip protobuf unmarshal is not supported")
	}

	var result []state.ListOption

	for _, q := range wo.LabelQueries {
		for _, term := range q.Terms {
			keyOp := ordered.MakePair(term.Key, term.Op)

			switch keyOp {
			case ordered.MakePair(omni.LabelCluster, resource.LabelOpEqual):
				if len(term.Value) == 0 || term.Value[0] == "" {
					return nil, fmt.Errorf("empty value for %q is not supported", term.Key)
				}

				result = append(result, state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, term.Value[0])))
			default:
				return nil, fmt.Errorf("unsupported label query term %v", keyOp)
			}
		}
	}

	if len(result) == 0 {
		return nil, errors.New("cluster ID must be specified in query")
	}

	return result, nil
}

// Watch implements [state.CoreState] interface.
func (*State) Watch(context.Context, resource.Pointer, chan<- state.Event, ...state.WatchOption) error {
	return makeUnsupportedError(errors.New("watch is not supported"))
}

// Create implements [state.CoreState] interface. It is not supported.
func (*State) Create(context.Context, resource.Resource, ...state.CreateOption) error {
	return makeUnsupportedError(errors.New("create is not supported"))
}

// Update implements [state.CoreState] interface. It is not supported.
func (*State) Update(context.Context, resource.Resource, ...state.UpdateOption) error {
	return makeUnsupportedError(errors.New("update is not supported"))
}

// Destroy implements [state.CoreState] interface. It is not supported.
func (*State) Destroy(context.Context, resource.Pointer, ...state.DestroyOption) error {
	return makeUnsupportedError(errors.New("destroy is not supported"))
}

// WatchKindAggregated implements [state.CoreState] interface. It is not supported.
func (*State) WatchKindAggregated(context.Context, resource.Kind, chan<- []state.Event, ...state.WatchKindOption) error {
	return makeUnsupportedError(errors.New("watch kind aggregated is not supported"))
}

type notFoundError struct{ err error }

func (e *notFoundError) Error() string {
	return fmt.Sprintf("resource doesn't exist: %v", e.err)
}

func (*notFoundError) NotFoundError() {}

func makeNotFoundError(err error) error {
	return &notFoundError{err}
}

type unsupportedError struct{ err error }

func (u *unsupportedError) Error() string {
	return fmt.Sprintf("unsupported resource type: %v", u.err)
}

func (*unsupportedError) UnsupportedError() {}

func makeUnsupportedError(err error) error {
	return &unsupportedError{
		err,
	}
}

type validationError struct{ err error }

func (e *validationError) Error() string {
	return fmt.Sprintf("failed to validate: %v", e.err)
}

func (*validationError) ValidationError() {}

func makeValidationError(err error) error {
	return &validationError{
		err,
	}
}

func fromOpts[T any, F ~func(*T)](opts ...F) T {
	var result T

	for _, opt := range opts {
		opt(&result)
	}

	return result
}
