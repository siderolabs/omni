// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package talos implements the connector that can pull data from the Talos controller runtime.
package talos

import (
	"context"
	"errors"
	"fmt"

	cosiresource "github.com/cosi-project/runtime/pkg/resource"
	taloscommon "github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	talosrole "github.com/siderolabs/talos/pkg/machinery/role"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	pkgruntime "github.com/siderolabs/omni/client/pkg/runtime"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/cosi"
	"github.com/siderolabs/omni/internal/pkg/config"
)

// Name talos runtime string id.
var Name = common.Runtime_Talos.String()

// Runtime implements runtime.Runtime for Talos resources.
type Runtime struct {
	clientFactory *ClientFactory
	logger        *zap.Logger
}

// New creates a new Talos runtime.
func New(clientFactory *ClientFactory, logger *zap.Logger) *Runtime {
	return &Runtime{
		clientFactory: clientFactory,
		logger:        logger.With(logging.Component("talos_runtime")),
	}
}

// Watch implements runtime.Runtime.
func (r *Runtime) Watch(ctx context.Context, events chan<- runtime.WatchResponse, setters ...runtime.QueryOption) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("watch panic")

			r.logger.Error("watch panicked", zap.Stack("stack"), zap.Error(err))
		}
	}()

	return r.watch(ctx, events, setters...)
}

func (r *Runtime) watch(ctx context.Context, events chan<- runtime.WatchResponse, setters ...runtime.QueryOption) error {
	opts := runtime.NewQueryOptions(setters...)

	switch len(opts.Nodes) {
	case 0:
		// nothing
	case 1:
		ctx = client.WithNode(ctx, opts.Nodes[0])
	default:
		return errors.New("multiple nodes are not supported for Watch")
	}

	c, err := r.GetClient(ctx, opts.Context)
	if err != nil {
		return err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, constants.APIAuthzRoleMetadataKey, string(talosrole.Reader))

	var queries []cosiresource.LabelQuery

	if len(opts.LabelSelectors) > 0 {
		queries, err = labels.ParseSelectors(opts.LabelSelectors)
		if err != nil {
			return err
		}
	}

	return cosi.WatchLegacy(
		ctx,
		c.COSI,
		cosiresource.NewMetadata(
			opts.Namespace,
			opts.Resource,
			opts.Name,
			cosiresource.VersionUndefined,
		),
		events,
		opts.TailEvents,
		queries,
	)
}

// Get implements runtime.Runtime.
func (r *Runtime) Get(ctx context.Context, setters ...runtime.QueryOption) (any, error) {
	opts := runtime.NewQueryOptions(setters...)

	var responseMetadata *taloscommon.Metadata

	switch len(opts.Nodes) {
	case 0:
		// nothing
	case 1:
		ctx = client.WithNode(ctx, opts.Nodes[0])
		responseMetadata = &taloscommon.Metadata{
			Hostname: opts.Nodes[0],
		}
	default:
		return nil, errors.New("multiple nodes are not supported for Get")
	}

	c, err := r.GetClient(ctx, opts.Context)
	if err != nil {
		return nil, err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, constants.APIAuthzRoleMetadataKey, string(talosrole.Reader))

	res, err := c.COSI.Get(ctx, cosiresource.NewMetadata(opts.Namespace, opts.Resource, opts.Name, cosiresource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	return runtime.NewResource(res, runtime.WithMetadata(responseMetadata))
}

// List implements runtime.Runtime.
func (r *Runtime) List(ctx context.Context, setters ...runtime.QueryOption) (runtime.ListResult, error) {
	opts := runtime.NewQueryOptions(setters...)

	if len(opts.Nodes) == 0 {
		opts.Nodes = []string{""}
	}

	c, err := r.GetClient(ctx, opts.Context)
	if err != nil {
		return runtime.ListResult{}, err
	}

	var res []pkgruntime.ListItem

	for _, node := range opts.Nodes {
		var responseMetadata *taloscommon.Metadata

		nodeCtx := ctx

		if node != "" {
			nodeCtx = client.WithNode(ctx, node)
			responseMetadata = &taloscommon.Metadata{
				Hostname: node,
			}
		}

		nodeCtx = metadata.AppendToOutgoingContext(nodeCtx, constants.APIAuthzRoleMetadataKey, string(talosrole.Reader))

		items, err := c.COSI.List(nodeCtx, cosiresource.NewMetadata(opts.Namespace, opts.Resource, "", cosiresource.VersionUndefined))
		if err != nil {
			return runtime.ListResult{}, err
		}

		for _, item := range items.Items {
			resource, err := runtime.NewResource(item, runtime.WithMetadata(responseMetadata))
			if err != nil {
				return runtime.ListResult{}, err
			}

			res = append(res, newItem(resource))
		}
	}

	return runtime.ListResult{
		Items: res,
		Total: len(res),
	}, nil
}

// Create implements runtime.Runtime.
func (r *Runtime) Create(context.Context, cosiresource.Resource, ...runtime.QueryOption) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// Update implements runtime.Runtime.
func (r *Runtime) Update(context.Context, cosiresource.Resource, ...runtime.QueryOption) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// Delete implements runtime.Runtime.
func (r *Runtime) Delete(context.Context, ...runtime.QueryOption) error {
	return status.Error(codes.Unimplemented, "not implemented")
}

// GetTalosconfigRaw returns raw talosconfig for the cluster (or for whole instance if the cluster is not specified).
func (r *Runtime) GetTalosconfigRaw(context *common.Context, identity string) ([]byte, error) {
	auth := clientconfig.Auth{}

	auth.SideroV1 = &clientconfig.SideroV1{
		Identity: identity,
	}

	contextName := config.Config.Account.Name
	apiURL := config.Config.Services.API.URL()

	cluster := ""

	if context != nil {
		cluster = context.Name
	}

	if cluster != "" {
		contextName = contextName + "-" + cluster
	}

	talosconfig := clientconfig.Config{
		Context: contextName,
		Contexts: map[string]*clientconfig.Context{
			contextName: {
				Endpoints: []string{
					apiURL,
				},
				Auth:    auth,
				Cluster: cluster,
			},
		},
	}

	return talosconfig.Bytes()
}

// GetClient returns talos client for the cluster name.
func (r *Runtime) GetClient(ctx context.Context, clusterName string) (*Client, error) {
	c, err := r.clientFactory.Get(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	connected, err := c.Connected(ctx, r.clientFactory.omniState)
	if err != nil {
		return nil, err
	}

	if !connected {
		return nil, fmt.Errorf("the cluster %s is not reachable", clusterName)
	}

	return c, nil
}

type item struct {
	runtime.BasicItem[*runtime.Resource]
}

func (it *item) Field(name string) (string, bool) {
	val, ok := it.BasicItem.Field(name)
	if ok {
		return val, true
	}

	val, ok = runtime.ResourceField(it.BasicItem.Unwrap().Resource, name)
	if ok {
		return val, true
	}

	return "", false
}

func (it *item) Match(searchFor string) bool {
	return it.BasicItem.Match(searchFor) || runtime.MatchResource(it.BasicItem.Unwrap().Resource, searchFor)
}

func (it *item) Unwrap() any {
	return it.BasicItem.Unwrap()
}

func newItem(res *runtime.Resource) pkgruntime.ListItem {
	return &item{BasicItem: runtime.MakeBasicItem(res.Metadata.ID, res.Metadata.Namespace, res)}
}
