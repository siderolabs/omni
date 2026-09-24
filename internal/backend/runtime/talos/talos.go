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
	goruntime "runtime"

	cosiresource "github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/common"
	"github.com/siderolabs/omni/client/pkg/access/role"
	"github.com/siderolabs/omni/client/pkg/cosi/labels"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	pkgruntime "github.com/siderolabs/omni/client/pkg/runtime"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/cosi"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
)

// Name talos runtime string id.
var Name = common.Runtime_Talos.String()

// Runtime implements runtime.Runtime for Talos resources.
type Runtime struct {
	clientFactory *ClientFactory
	logger        *zap.Logger
	accountName   string
	apiURL        string
}

// New creates a new Talos runtime.
func New(clientFactory *ClientFactory, logger *zap.Logger, accountName string, apiURL string) *Runtime {
	return &Runtime{
		clientFactory: clientFactory,
		logger:        logger.With(logging.Component("talos_runtime")),
		accountName:   accountName,
		apiURL:        apiURL,
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

	var (
		st      state.State
		release func()
		err     error
	)

	switch len(opts.Machines) {
	case 0:
		st, release, err = r.callerState(ctx, opts.Context, "", opts.Resource)
	case 1:
		st, release, err = r.callerState(ctx, "", opts.Machines[0], opts.Resource)
	default:
		return errors.New("multiple machines are not supported for Watch")
	}

	if err != nil {
		return err
	}

	defer release()

	var queries []cosiresource.LabelQuery

	if len(opts.LabelSelectors) > 0 {
		queries, err = labels.ParseSelectors(opts.LabelSelectors)
		if err != nil {
			return err
		}
	}

	return cosi.WatchLegacy(
		ctx,
		st,
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

	var (
		st      state.State
		release func()
		err     error
	)

	switch len(opts.Machines) {
	case 0:
		st, release, err = r.callerState(ctx, opts.Context, "", opts.Resource)
	case 1:
		st, release, err = r.callerState(ctx, "", opts.Machines[0], opts.Resource)
	default:
		return nil, errors.New("multiple machines are not supported for Get")
	}

	if err != nil {
		return nil, err
	}

	defer release()

	res, err := st.Get(ctx, cosiresource.NewMetadata(opts.Namespace, opts.Resource, opts.Name, cosiresource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	return runtime.NewResource(res)
}

// List implements runtime.Runtime.
func (r *Runtime) List(ctx context.Context, setters ...runtime.QueryOption) (runtime.ListResult, error) {
	opts := runtime.NewQueryOptions(setters...)

	if len(opts.Machines) == 0 {
		opts.Machines = []string{""}
	}

	var res []pkgruntime.ListItem

	for _, machine := range opts.Machines {
		items, err := r.list(ctx, machine, opts)
		if err != nil {
			return runtime.ListResult{}, err
		}

		res = append(res, items...)
	}

	return runtime.ListResult{
		Items: res,
		Total: len(res),
	}, nil
}

// list lists the resources on a single machine, or on the cluster if the machine is empty.
func (r *Runtime) list(ctx context.Context, machine string, opts *runtime.QueryOptions) ([]pkgruntime.ListItem, error) {
	var (
		st      state.State
		release func()
		err     error
	)

	if machine == "" {
		st, release, err = r.callerState(ctx, opts.Context, "", opts.Resource)
	} else {
		st, release, err = r.callerState(ctx, "", machine, opts.Resource)
	}

	if err != nil {
		return nil, err
	}

	defer release()

	items, err := st.List(ctx, cosiresource.NewMetadata(opts.Namespace, opts.Resource, "", cosiresource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	var res []pkgruntime.ListItem

	for _, item := range items.Items {
		resource, err := runtime.NewResource(item)
		if err != nil {
			return nil, err
		}

		res = append(res, newItem(resource))
	}

	return res, nil
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

	contextName := r.accountName
	apiURL := r.apiURL

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

// callerState returns the resource state of the target for a request made on behalf of the caller in the context:
// either the cluster, or the machine when machineID is set.
func (r *Runtime) callerState(ctx context.Context, clusterID, machineID, resourceType string) (state.State, func(), error) {
	if machineID != "" {
		machineStatus, err := safe.StateGet[*omni.MachineStatus](ctx, r.clientFactory.omniState, omni.NewMachineStatus(machineID).Metadata())
		if err != nil {
			return nil, nil, err
		}

		clusterID = machineStatus.TypedSpec().Value.Cluster
	} else if clusterID == "" {
		return nil, nil, status.Error(codes.InvalidArgument, "either a cluster or a machine is required")
	}

	ctx, err := accesspolicy.ApplyClusterAccessPolicy(ctx, clusterID, r.clientFactory.omniState)
	if err != nil {
		return nil, nil, err
	}

	requiredRole := role.Reader
	if clusterID == "" {
		requiredRole = role.Operator
	}

	if _, err = auth.CheckGRPC(ctx, auth.WithRole(requiredRole)); err != nil {
		return nil, nil, err
	}

	var c *Client

	if machineID != "" {
		c, err = r.clientFactory.GetReaderForMachine(ctx, machineID)
	} else {
		c, err = r.clientFactory.GetReaderForCluster(ctx, clusterID)
	}

	if err != nil {
		return nil, nil, err
	}

	// machine might have moved to another cluster since it was authorized
	if c.ClusterID() != "" && c.ClusterID() != clusterID {
		return nil, nil, status.Errorf(codes.Unavailable, "machine %q changed its cluster", machineID)
	}

	if c, err = r.checkConnected(ctx, c); err != nil {
		return nil, nil, err
	}

	if err = CheckSensitivity(ctx, c.COSI, resourceType); err != nil {
		return nil, nil, err
	}

	// the cached client closes its connection when garbage collected, keep it until the caller is done with the state
	return c.COSI, func() { goruntime.KeepAlive(c) }, nil
}

// checkConnected returns the client if its cluster or machine is reachable.
func (r *Runtime) checkConnected(ctx context.Context, c *Client) (*Client, error) {
	connected, err := c.Connected(ctx, r.clientFactory.omniState)
	if err != nil {
		return nil, err
	}

	if !connected {
		if c.clusterID == "" {
			return nil, fmt.Errorf("the machine %s is not reachable", c.machineID)
		}

		return nil, fmt.Errorf("the cluster %s is not reachable", c.clusterID)
	}

	return c, nil
}

// GetClientForCluster returns talos client for the cluster name.
func (r *Runtime) GetClientForCluster(ctx context.Context, clusterName string) (*Client, error) {
	c, err := r.clientFactory.GetForCluster(ctx, clusterName)
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

// GetClientForMachine returns a Talos client connected directly to the given machine's SideroLink endpoint.
//
// Cluster membership is determined automatically from the machine's state.
func (r *Runtime) GetClientForMachine(ctx context.Context, machineID string) (*Client, error) {
	c, err := r.clientFactory.GetForMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}

	connected, err := c.Connected(ctx, r.clientFactory.omniState)
	if err != nil {
		return nil, err
	}

	if !connected {
		return nil, fmt.Errorf("the cluster %s is not reachable", c.clusterID)
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
