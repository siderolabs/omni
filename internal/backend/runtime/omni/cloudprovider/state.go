// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cloudprovider

import (
	"context"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/cloud"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// cloudProviderResourceSuffix is the suffix of the cloud provider specific resources.
//
// They must follow the pattern: <resource-type>.<cloud-provider-id>.cloudprovider.sidero.dev.
const cloudProviderResourceSuffix = ".cloudprovider.sidero.dev"

// State is a state implementation doing special handling of the cloud-provider specific resources.
type State struct {
	innerState state.CoreState
	logger     *zap.Logger
}

// NewState creates a new State.
func NewState(innerState state.CoreState, logger *zap.Logger) *State {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &State{
		innerState: validated.NewState(innerState, validationOptions()...),
		logger:     logger,
	}
}

// Get implements state.CoreState interface.
func (st *State) Get(ctx context.Context, pointer resource.Pointer, option ...state.GetOption) (resource.Resource, error) {
	cloudProviderID, err := st.checkAuthorization(ctx, pointer.Namespace(), pointer.Type())
	if err != nil {
		return nil, err
	}

	res, err := st.innerState.Get(ctx, pointer, option...)
	if err != nil {
		return nil, err
	}

	if cloudProviderID == "" {
		return res, nil
	}

	resCloudProviderID, ok := res.Metadata().Labels().Get(omni.LabelCloudProviderID)
	if ok && cloudProviderID == resCloudProviderID {
		return res, nil
	}

	return nil, status.Errorf(codes.NotFound, "not found")
}

// List implements state.CoreState interface.
func (st *State) List(ctx context.Context, kind resource.Kind, option ...state.ListOption) (resource.List, error) {
	cloudProviderID, err := st.checkAuthorization(ctx, kind.Namespace(), kind.Type())
	if err != nil {
		return resource.List{}, err
	}

	list, err := st.innerState.List(ctx, kind, option...)
	if err != nil {
		return resource.List{}, err
	}

	if cloudProviderID == "" {
		return list, nil
	}

	filteredList := make([]resource.Resource, 0, len(list.Items))

	for _, item := range list.Items {
		resCloudProviderID, ok := item.Metadata().Labels().Get(omni.LabelCloudProviderID)
		if ok && cloudProviderID == resCloudProviderID {
			filteredList = append(filteredList, item)
		}
	}

	return resource.List{Items: filteredList}, nil
}

// Create implements state.CoreState interface.
func (st *State) Create(ctx context.Context, resource resource.Resource, option ...state.CreateOption) error {
	cloudProviderID, err := st.checkAuthorization(ctx, resource.Metadata().Namespace(), resource.Metadata().Type())
	if err != nil {
		return err
	}

	if cloudProviderID != "" && resource.Metadata().Type() == cloud.MachineRequestType {
		return status.Errorf(codes.PermissionDenied, "cloud providers are not allowed to create machine requests")
	}

	if cloudProviderID == "" {
		return st.innerState.Create(ctx, resource, option...)
	}

	resource.Metadata().Labels().Set(omni.LabelCloudProviderID, cloudProviderID)

	return st.innerState.Create(ctx, resource, option...)
}

// Update implements state.CoreState interface.
func (st *State) Update(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	cloudProviderID, err := st.checkAuthorization(ctx, newResource.Metadata().Namespace(), newResource.Metadata().Type())
	if err != nil {
		return err
	}

	oldResource, err := st.innerState.Get(ctx, newResource.Metadata())
	if err != nil {
		return err
	}

	if cloudProviderID == "" {
		return st.innerState.Update(ctx, newResource, opts...)
	}

	if newResource.Metadata().Type() == cloud.MachineRequestType {
		oldMd := oldResource.Metadata().Copy()
		oldMd.Finalizers().Set(resource.Finalizers{})

		newMd := newResource.Metadata().Copy()
		newMd.Finalizers().Set(resource.Finalizers{})

		if !oldMd.Equal(newMd) {
			return status.Errorf(codes.PermissionDenied, "cloud providers are not allowed to update machine requests other than setting finalizers")
		}
	}

	oldResCloudProviderID, ok := oldResource.Metadata().Labels().Get(omni.LabelCloudProviderID)
	if ok && oldResCloudProviderID == cloudProviderID {
		newResource.Metadata().Labels().Set(omni.LabelCloudProviderID, cloudProviderID)

		return st.innerState.Update(ctx, newResource, opts...)
	}

	return status.Errorf(codes.NotFound, "not found")
}

// Destroy implements state.CoreState interface.
func (st *State) Destroy(ctx context.Context, pointer resource.Pointer, option ...state.DestroyOption) error {
	cloudProviderID, err := st.checkAuthorization(ctx, pointer.Namespace(), pointer.Type())
	if err != nil {
		return err
	}

	res, err := st.innerState.Get(ctx, pointer)
	if err != nil {
		return err
	}

	if cloudProviderID == "" {
		return st.innerState.Destroy(ctx, pointer, option...)
	}

	resCloudProviderID, ok := res.Metadata().Labels().Get(omni.LabelCloudProviderID)
	if ok && cloudProviderID == resCloudProviderID {
		return st.innerState.Destroy(ctx, pointer, option...)
	}

	return status.Errorf(codes.NotFound, "not found")
}

// Watch implements state.CoreState interface.
func (st *State) Watch(ctx context.Context, pointer resource.Pointer, eventCh chan<- state.Event, option ...state.WatchOption) error {
	cloudProviderID, err := st.checkAuthorization(ctx, pointer.Namespace(), pointer.Type())
	if err != nil {
		return err
	}

	if cloudProviderID == "" {
		return st.innerState.Watch(ctx, pointer, eventCh, option...)
	}

	innerEventCh := st.filterEvents(ctx, cloudProviderID, eventCh)

	return st.innerState.Watch(ctx, pointer, innerEventCh, option...)
}

// WatchKind implements state.CoreState interface.
func (st *State) WatchKind(ctx context.Context, kind resource.Kind, eventCh chan<- state.Event, option ...state.WatchKindOption) error {
	cloudProviderID, err := st.checkAuthorization(ctx, kind.Namespace(), kind.Type())
	if err != nil {
		return err
	}

	if cloudProviderID == "" {
		return st.innerState.WatchKind(ctx, kind, eventCh, option...)
	}

	innerEventCh := st.filterEvents(ctx, cloudProviderID, eventCh)

	return st.innerState.WatchKind(ctx, kind, innerEventCh, option...)
}

// WatchKindAggregated implements state.CoreState interface.
func (st *State) WatchKindAggregated(ctx context.Context, kind resource.Kind, eventsCh chan<- []state.Event, option ...state.WatchKindOption) error {
	cloudProviderID, err := st.checkAuthorization(ctx, kind.Namespace(), kind.Type())
	if err != nil {
		return err
	}

	if cloudProviderID == "" {
		return st.innerState.WatchKindAggregated(ctx, kind, eventsCh, option...)
	}

	innerEventsCh := st.filterEventsAggregated(ctx, cloudProviderID, eventsCh)

	return st.innerState.WatchKindAggregated(ctx, kind, innerEventsCh, option...)
}

func (st *State) checkAuthorization(ctx context.Context, ns resource.Namespace, resType resource.Type) (cloudProviderID string, err error) {
	if actor.ContextIsInternalActor(ctx) {
		return "", nil
	}

	checkResult, err := auth.CheckGRPC(ctx, auth.WithRole(role.CloudProvider))
	if err != nil {
		return "", err
	}

	// if the role is exactly CloudProvider, additionally, check for the label match
	if checkResult.Role == role.CloudProvider {
		var checkLabel bool

		checkLabel, err = st.checkNamespaceAndType(ns, checkResult.CloudProviderID, resType)
		if err != nil {
			return "", err
		}

		// return the cloud provider ID only for the resource live in a shared namespace, i.e., "cloud-provider"
		// as their cloud provider ID label needs to be checked.
		if checkLabel {
			return checkResult.CloudProviderID, nil
		}
	}

	return "", nil
}

func (st *State) filterEvents(ctx context.Context, cloudProviderID string, eventCh chan<- state.Event) chan state.Event {
	innerEventCh := make(chan state.Event)

	panichandler.Go(func() {
		defer close(eventCh)

		for {
			select {
			case <-ctx.Done():
				return

			case event, ok := <-innerEventCh:
				if !ok {
					return
				}

				if event.Type == state.Bootstrapped || event.Type == state.Errored {
					channel.SendWithContext(ctx, eventCh, event)

					continue
				}

				if event.Resource != nil {
					resCloudProviderID, cpOk := event.Resource.Metadata().Labels().Get(omni.LabelCloudProviderID)
					if !cpOk || cloudProviderID != resCloudProviderID {
						continue // discard
					}
				}

				channel.SendWithContext(ctx, eventCh, event)
			}
		}
	}, st.logger)

	return innerEventCh
}

func (st *State) filterEventsAggregated(ctx context.Context, cloudProviderID string, eventsCh chan<- []state.Event) chan []state.Event {
	innerEventsCh := make(chan []state.Event)

	panichandler.Go(func() {
		defer close(eventsCh)

		for {
			select {
			case <-ctx.Done():
				return

			case events, ok := <-innerEventsCh:
				if !ok {
					return
				}

				filteredEvents := make([]state.Event, 0, len(events))

				for _, event := range events {
					if event.Resource != nil {
						resCloudProviderID, cpOk := event.Resource.Metadata().Labels().Get(omni.LabelCloudProviderID)
						if !cpOk || cloudProviderID != resCloudProviderID {
							continue // discard
						}
					}

					filteredEvents = append(filteredEvents, event)
				}

				channel.SendWithContext(ctx, eventsCh, filteredEvents)
			}
		}
	}, st.logger)

	return innerEventsCh
}

func (st *State) checkNamespaceAndType(ns resource.Namespace, cloudProviderID string, resType resource.Type) (checkLabel bool, err error) {
	if ns == resources.CloudProviderNamespace {
		return true, nil
	}

	cloudProviderSpecificNamespace := resources.CloudProviderSpecificNamespacePrefix + cloudProviderID
	if ns == cloudProviderSpecificNamespace {
		resTypeSuffix := "." + cloudProviderID + cloudProviderResourceSuffix

		if !strings.HasSuffix(resType, resTypeSuffix) {
			return false, status.Errorf(codes.InvalidArgument, "resources in namespace %q must have a type suffix %q", ns, resTypeSuffix)
		}

		return false, nil
	}

	return false, status.Errorf(codes.PermissionDenied, "namespace not allowed, must be one of %s or %s",
		resources.CloudProviderNamespace, cloudProviderSpecificNamespace)
}
