// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package virtual provides a virtual state implementation.
package virtual

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/accesspolicy"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// State is a virtual state implementation which provides virtual resources.
// Virtual resources are not stored in the backend, but are generated on the fly.
// They also do not behave like regular resources; for example, their content might differ, according to the caller/context.
type State struct {
	PrimaryState state.State

	computed map[resource.Type]*Computed
}

// NewState creates new virtual state instance.
func NewState(state state.State) *State {
	return &State{
		PrimaryState: state,
		computed:     map[resource.Type]*Computed{},
	}
}

// RunComputed starts a new computed producer on the virtual state
// which runs until the context is canceled.
func (v *State) RunComputed(ctx context.Context, t resource.Type, factory ProducerFactory, resolveID ProducerIDTransformer, logger *zap.Logger) {
	st := NewComputed(t, factory, resolveID, time.Minute, logger)

	v.computed[t] = st

	st.Run(ctx)
}

// Get implements state.CoreState.
func (v *State) Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	if len(opts) > 0 {
		return nil, errUnsupported(errors.New("no get options are supported"))
	}

	if err := v.validateKind(ptr); err != nil {
		return nil, err
	}

	computed, ok := v.computed[ptr.Type()]
	if ok {
		return computed.Get(ctx, ptr, opts...)
	}

	switch ptr.Type() {
	case virtual.CurrentUserType:
		if ptr.ID() != virtual.CurrentUserID {
			return nil, errNotFound(ptr)
		}

		return v.currentUser(ctx)
	case virtual.PermissionsType:
		return v.permissions(ctx)
	case virtual.ClusterPermissionsType:
		return v.clusterPermissions(ctx, ptr)
	case virtual.LabelsCompletionType:
		return v.labelsCompletion(ctx, ptr)
	default:
		return nil, errUnsupported(fmt.Errorf("unsupported resource type for get %q", ptr.Type()))
	}
}

// List implements state.CoreState.
func (v *State) List(ctx context.Context, kind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	if len(opts) > 0 {
		return resource.List{}, errUnsupported(errors.New("no list options are supported"))
	}

	if err := v.validateKind(kind); err != nil {
		return resource.List{}, err
	}

	switch kind.Type() {
	case virtual.CurrentUserType:
		user, err := v.currentUser(ctx)
		if err != nil {
			return resource.List{}, err
		}

		return resource.List{Items: []resource.Resource{user}}, nil
	case virtual.PermissionsType:
		permissions, err := v.permissions(ctx)
		if err != nil {
			return resource.List{}, err
		}

		return resource.List{Items: []resource.Resource{permissions}}, nil
	default:
		return resource.List{}, errUnsupported(fmt.Errorf("unsupported resource type for list %q", kind.Type()))
	}
}

// Create implements state.CoreState.
func (v *State) Create(_ context.Context, _ resource.Resource, _ ...state.CreateOption) error {
	return errUnsupported(errors.New("create is not supported"))
}

// Update implements state.CoreState.
func (v *State) Update(_ context.Context, _ resource.Resource, _ ...state.UpdateOption) error {
	return errUnsupported(errors.New("update is not supported"))
}

// Destroy implements state.CoreState.
func (v *State) Destroy(_ context.Context, _ resource.Pointer, _ ...state.DestroyOption) error {
	return errUnsupported(errors.New("destroy is not supported"))
}

// Watch implements state.CoreState.
func (v *State) Watch(ctx context.Context, ptr resource.Pointer, c chan<- state.Event, opts ...state.WatchOption) error {
	computed, ok := v.computed[ptr.Type()]
	if ok {
		return computed.Watch(ctx, ptr, c, opts...)
	}

	return errUnsupported(fmt.Errorf("unsupported resource type for watch %q", ptr.Type()))
}

// WatchKind implements state.CoreState.
func (v *State) WatchKind(_ context.Context, ptr resource.Kind, _ chan<- state.Event, _ ...state.WatchKindOption) error {
	return errUnsupported(fmt.Errorf("unsupported resource type for watch kind %q", ptr.Type()))
}

// WatchKindAggregated implements state.CoreState.
func (v *State) WatchKindAggregated(_ context.Context, _ resource.Kind, _ chan<- []state.Event, _ ...state.WatchKindOption) error {
	return errUnsupported(errors.New("watch kind aggregated is not supported"))
}

func (v *State) validateKind(kind resource.Kind) error {
	if kind.Namespace() != resources.VirtualNamespace {
		return errUnsupported(fmt.Errorf("unsupported namespace: %s", kind.Namespace()))
	}

	return nil
}

func (v *State) currentUser(ctx context.Context) (*virtual.CurrentUser, error) {
	identityVal, _ := ctxstore.Value[auth.IdentityContextKey](ctx)

	userRole := role.None

	if val, ok := ctxstore.Value[auth.RoleContextKey](ctx); ok {
		userRole = val.Role
	}

	user := virtual.NewCurrentUser()

	user.TypedSpec().Value.Identity = identityVal.Identity
	user.TypedSpec().Value.Role = string(userRole)

	version, err := resource.ParseVersion("1")
	if err != nil {
		return nil, err
	}

	user.Metadata().SetVersion(version)

	return user, nil
}

func (v *State) permissions(ctx context.Context) (*virtual.Permissions, error) {
	userRole := role.None

	if val, ok := ctxstore.Value[auth.RoleContextKey](ctx); ok {
		userRole = val.Role
	}

	permissions := virtual.NewPermissions()

	if userRole.Check(role.Reader) == nil {
		permissions.TypedSpec().Value.CanReadMachineConfigPatches = true
		permissions.TypedSpec().Value.CanReadMachineLogs = true
		permissions.TypedSpec().Value.CanReadClusters = true
		permissions.TypedSpec().Value.CanReadMachines = true
	}

	if userRole.Check(role.Operator) == nil {
		permissions.TypedSpec().Value.CanRemoveMachines = true
		permissions.TypedSpec().Value.CanCreateClusters = true
		permissions.TypedSpec().Value.CanManageMachineConfigPatches = true
		permissions.TypedSpec().Value.CanAccessMaintenanceNodes = true
	}

	if userRole.Check(role.Admin) == nil {
		permissions.TypedSpec().Value.CanManageUsers = userRole.Check(role.Admin) == nil
		permissions.TypedSpec().Value.CanManageBackupStore = userRole.Check(role.Admin) == nil
	}

	if !permissions.TypedSpec().Value.CanCreateClusters {
		_, err := safe.StateGet[*authres.AccessPolicy](ctx, v.PrimaryState, authres.NewAccessPolicy().Metadata())
		if err == nil {
			// if there is an access policy, we assume user can create clusters - we do the actual check on creation time by id, selectors, etc.
			permissions.TypedSpec().Value.CanCreateClusters = true
		} else if !state.IsNotFoundError(err) {
			return nil, err
		}
	}

	version, err := resource.ParseVersion("1")
	if err != nil {
		return nil, err
	}

	permissions.Metadata().SetVersion(version)

	return permissions, nil
}

func (v *State) clusterPermissions(ctx context.Context, ptr resource.Pointer) (*virtual.ClusterPermissions, error) {
	userRole, _, err := accesspolicy.RoleForCluster(ctx, ptr.ID(), v.PrimaryState)
	if err != nil {
		return nil, err
	}

	clusterPermissions := virtual.NewClusterPermissions(ptr.ID())

	if userRole.Check(role.Reader) == nil {
		clusterPermissions.TypedSpec().Value.CanRebootMachines = true
		clusterPermissions.TypedSpec().Value.CanDownloadKubeconfig = true
		clusterPermissions.TypedSpec().Value.CanDownloadTalosconfig = true
		clusterPermissions.TypedSpec().Value.CanReadConfigPatches = true
	}

	if userRole.Check(role.Operator) == nil {
		clusterPermissions.TypedSpec().Value.CanAddMachines = true
		clusterPermissions.TypedSpec().Value.CanRemoveMachines = true
		clusterPermissions.TypedSpec().Value.CanUpdateKubernetes = true
		clusterPermissions.TypedSpec().Value.CanUpdateTalos = true
		clusterPermissions.TypedSpec().Value.CanManageConfigPatches = true
		clusterPermissions.TypedSpec().Value.CanSyncKubernetesManifests = true
		clusterPermissions.TypedSpec().Value.CanManageClusterFeatures = true
	}

	version, err := resource.ParseVersion("1")
	if err != nil {
		return nil, err
	}

	clusterPermissions.Metadata().SetVersion(version)

	return clusterPermissions, nil
}

func (v *State) labelsCompletion(ctx context.Context, ptr resource.Pointer) (*virtual.LabelsCompletion, error) {
	md := resource.NewMetadata(resources.DefaultNamespace, ptr.ID(), "", resource.VersionUndefined)

	list, err := v.PrimaryState.List(ctx, md)
	if err != nil {
		return nil, err
	}

	labels := map[string]*specs.LabelsCompletionSpec_Values{}

	for _, res := range list.Items {
		for label, value := range res.Metadata().Labels().Raw() {
			if _, ok := labels[label]; !ok {
				labels[label] = &specs.LabelsCompletionSpec_Values{}
			}

			if !slices.Contains(labels[label].Items, value) {
				labels[label].Items = append(labels[label].Items, value)
			}
		}
	}

	completion := virtual.NewLabelsCompletion(resources.VirtualNamespace, ptr.ID())
	completion.TypedSpec().Value.Items = labels

	return completion, nil
}
