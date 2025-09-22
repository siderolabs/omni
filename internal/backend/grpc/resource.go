// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	cosiruntime "github.com/cosi-project/runtime/pkg/controller/runtime"
	cosiresource "github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	gateway "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/siderolabs/go-api-signature/pkg/message"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
	"github.com/siderolabs/omni/internal/backend/runtime"
)

func newResourceServer(state state.State, runtime *cosiruntime.Runtime) *ResourceServer {
	return &ResourceServer{
		runtime: runtime,
		state:   state,
	}
}

// ResourceServer implements resources CRUD API.
type ResourceServer struct {
	resources.UnimplementedResourceServiceServer
	runtime *cosiruntime.Runtime
	state   state.State
}

func (s *ResourceServer) register(server grpc.ServiceRegistrar) {
	resources.RegisterResourceServiceServer(server, s)
}

func (s *ResourceServer) gateway(ctx context.Context, mux *gateway.ServeMux, address string, opts []grpc.DialOption) error {
	return resources.RegisterResourceServiceHandlerFromEndpoint(ctx, mux, address, opts)
}

// Get returns resource from cluster using Talos or Kubernetes.
func (s *ResourceServer) Get(ctx context.Context, in *resources.GetRequest) (*resources.GetResponse, error) {
	r, err := runtime.Get(getSource(ctx).String())
	if err != nil {
		return nil, err
	}

	opts := withContext(router.ExtractContext(ctx))

	opts = append(opts, withResource(in)...)

	if in.Id != "" {
		opts = append(opts, runtime.WithName(in.Id))
	}

	res := &resources.GetResponse{}

	md, _ := metadata.FromIncomingContext(ctx)

	if nodes := md.Get(router.ResolvedNodesHeaderKey); nodes != nil {
		opts = append(opts, runtime.WithNodes(nodes...))
	}

	result, err := r.Get(ctx, opts...)
	if err != nil {
		return nil, wrapError(err)
	}

	res.Body, err = runtime.MarshalJSON(result)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// List returns resources from cluster using Talos or Kubernetes.
func (s *ResourceServer) List(ctx context.Context, in *resources.ListRequest) (*resources.ListResponse, error) {
	r, err := runtime.Get(getSource(ctx).String())
	if err != nil {
		return nil, err
	}

	opts := withContext(router.ExtractContext(ctx))

	opts = append(opts, withResource(in)...)

	md, _ := metadata.FromIncomingContext(ctx)

	if s := md.Get(message.SelectorsHeaderKey); len(s) > 0 {
		opts = append(opts, runtime.WithLabelSelectors(s...))
	}

	if s := md.Get(message.FieldSelectorsHeaderKey); len(s) > 0 {
		opts = append(opts, runtime.WithFieldSelector(s[0]))
	}

	if nodes := md.Get(router.ResolvedNodesHeaderKey); nodes != nil {
		opts = append(opts, runtime.WithNodes(nodes...))
	}

	if in.Offset > 0 {
		opts = append(opts, runtime.WithOffset(int(in.Offset)))
	}

	if in.Limit > 0 {
		opts = append(opts, runtime.WithLimit(int(in.Limit)))
	}

	if len(in.SearchFor) > 0 {
		opts = append(opts, runtime.WithSearchFor(in.SearchFor))
	}

	opts = append(opts, runtime.WithSort(in.SortByField, in.SortDescending))

	items, err := r.List(ctx, opts...)
	if err != nil {
		return nil, wrapError(err)
	}

	result := make([]string, 0, len(items.Items))

	for _, item := range items.Items {
		data, err := runtime.MarshalJSON(item.Unwrap())
		if err != nil {
			return nil, err
		}

		result = append(result, data)
	}

	return &resources.ListResponse{
		Items: result,
		Total: int32(items.Total),
	}, nil
}

// Watch the resource.
func (s *ResourceServer) Watch(in *resources.WatchRequest, serv grpc.ServerStreamingServer[resources.WatchResponse]) error {
	ctx, cancel := context.WithCancel(serv.Context())
	defer cancel()

	r, err := runtime.Get(getSource(ctx).String())
	if err != nil {
		return err
	}

	opts := withContext(router.ExtractContext(ctx))

	opts = append(opts, withResource(in)...)

	md, _ := metadata.FromIncomingContext(ctx)

	if s := md.Get(message.SelectorsHeaderKey); len(s) > 0 {
		opts = append(opts, runtime.WithLabelSelectors(s...))
	}

	if s := md.Get(message.FieldSelectorsHeaderKey); len(s) > 0 {
		opts = append(opts, runtime.WithFieldSelector(s[0]))
	}

	if nodes := md.Get(router.ResolvedNodesHeaderKey); len(nodes) > 0 {
		opts = append(opts, runtime.WithNodes(nodes...))
	}

	if in.Id != "" {
		opts = append(opts, runtime.WithName(in.Id))
	}

	events := make(chan runtime.WatchResponse)
	eg, ctx := panichandler.ErrGroupWithContext(ctx)

	if in.TailEvents != 0 {
		opts = append(opts, runtime.WithTailEvents(int(in.TailEvents)))
	}

	if in.Offset != 0 {
		opts = append(opts, runtime.WithOffset(int(in.Offset)))
	}

	if in.Limit != 0 {
		opts = append(opts, runtime.WithLimit(int(in.Limit)))
	}

	if len(in.SearchFor) > 0 {
		opts = append(opts, runtime.WithSearchFor(in.SearchFor))
	}

	opts = append(opts, runtime.WithSort(in.SortByField, in.SortDescending))

	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case ev := <-events:
				if err = serv.Send(ev.Unwrap()); err != nil {
					return err
				}
			}
		}
	})

	eg.Go(func() error {
		defer cancel()

		return wrapError(r.Watch(ctx, events, opts...))
	})

	return eg.Wait()
}

// Create a new resource in Omni runtime or Kubernetes.
func (s *ResourceServer) Create(ctx context.Context, in *resources.CreateRequest) (*resources.CreateResponse, error) {
	r, err := runtime.Get(getSource(ctx).String())
	if err != nil {
		return nil, err
	}

	opts := withContext(router.ExtractContext(ctx))

	obj, err := CreateResource(in.Resource)
	if err != nil {
		return nil, err
	}

	if err = r.Create(ctx, obj, opts...); err != nil {
		return nil, wrapError(err)
	}

	return &resources.CreateResponse{}, nil
}

// Update a resource in Omni runtime or Kubernetes.
func (s *ResourceServer) Update(ctx context.Context, in *resources.UpdateRequest) (*resources.UpdateResponse, error) {
	r, err := runtime.Get(getSource(ctx).String())
	if err != nil {
		return nil, err
	}

	opts := withContext(router.ExtractContext(ctx))

	if in.CurrentVersion != "" {
		opts = append(opts, runtime.WithCurrentVersion(in.CurrentVersion))
	}

	obj, err := CreateResource(in.Resource)
	if err != nil {
		return nil, err
	}

	if err = r.Update(ctx, obj, opts...); err != nil {
		return nil, wrapError(err)
	}

	return &resources.UpdateResponse{}, nil
}

// Delete a resource in Omni runtime or Kubernetes.
func (s *ResourceServer) Delete(ctx context.Context, in *resources.DeleteRequest) (*resources.DeleteResponse, error) {
	r, err := runtime.Get(getSource(ctx).String())
	if err != nil {
		return nil, err
	}

	opts := withContext(router.ExtractContext(ctx))

	opts = append(opts,
		runtime.WithNamespace(in.Namespace),
		runtime.WithName(in.Id),
		runtime.WithResource(in.Type),
	)

	if err = r.Delete(ctx, opts...); err != nil {
		return nil, wrapError(err)
	}

	return &resources.DeleteResponse{}, nil
}

// Teardown a resource in Omni runtime.
func (s *ResourceServer) Teardown(ctx context.Context, in *resources.DeleteRequest) (*resources.DeleteResponse, error) {
	r, err := runtime.Get(getSource(ctx).String())
	if err != nil {
		return nil, err
	}

	opts := withContext(router.ExtractContext(ctx))

	opts = append(opts,
		runtime.WithNamespace(in.Namespace),
		runtime.WithName(in.Id),
		runtime.WithResource(in.Type),
		runtime.WithTeardownOnly(),
	)

	if err = r.Delete(ctx, opts...); err != nil {
		return nil, wrapError(err)
	}

	return &resources.DeleteResponse{}, nil
}

func getSource(ctx context.Context) common.Runtime {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		source := md.Get(message.RuntimeHeaderKey)
		if source != nil {
			if res, ok := common.Runtime_value[source[0]]; ok {
				return common.Runtime(res)
			}
		}
	}

	return common.Runtime_Kubernetes
}

type res interface {
	GetType() string
	GetNamespace() string
}

func withContext(ctx *common.Context) []runtime.QueryOption {
	if ctx == nil {
		return nil
	}

	return []runtime.QueryOption{runtime.WithContext(ctx.Name)}
}

func withResource(r res) []runtime.QueryOption {
	if r == nil {
		return nil
	}

	var opts []runtime.QueryOption

	if r.GetNamespace() != "" {
		opts = append(opts, runtime.WithNamespace(r.GetNamespace()))
	}

	if r.GetType() != "" {
		opts = append(opts, runtime.WithResource(r.GetType()))
	}

	return opts
}

// CreateResource creates a resource from a resource proto representation.
func CreateResource(resource *resources.Resource) (cosiresource.Resource, error) { //nolint:ireturn
	if resource == nil {
		return nil, errors.New("resource is nil")
	}

	if resource.Metadata == nil {
		return nil, errors.New("resource metadata is nil")
	}

	if resource.Metadata.Version == "" {
		resource.Metadata.Version = "1"
	}

	resource.Metadata.Phase = "running"

	obj, err := protobuf.CreateResource(resource.Metadata.Type)
	if err != nil {
		return nil, err
	}

	if resource.Spec != "" {
		err = json.Unmarshal([]byte(resource.Spec), obj.Spec())
		if err != nil {
			return nil, err
		}
	}

	if resource.Metadata.Created.AsTime().Equal(time.Unix(0, 0)) {
		resource.Metadata.Created = timestamppb.Now()
	}

	*obj.Metadata(), err = cosiresource.NewMetadataFromProto(resource.Metadata)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

type gRPCError interface {
	GRPCStatus() *status.Status
}

func wrapError(err error) error {
	var grpcErr gRPCError

	if errors.As(err, &grpcErr) { // avoid double wrapping
		return err
	}

	switch {
	case state.IsNotFoundError(err):
		return status.Error(codes.NotFound, err.Error())
	case state.IsOwnerConflictError(err):
		return status.Error(codes.PermissionDenied, err.Error())
	case state.IsPhaseConflictError(err):
		return status.Error(codes.InvalidArgument, err.Error())
	case state.IsConflictError(err):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	}

	return err
}
