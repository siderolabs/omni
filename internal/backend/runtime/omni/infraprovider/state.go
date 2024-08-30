// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package infraprovider

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
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

// infraProviderResourceSuffix is the suffix of the infra provider specific resources.
//
// They must follow the pattern: <resource-type>.<infra-provider-id>.infraprovider.sidero.dev.
const infraProviderResourceSuffix = ".infraprovider.sidero.dev"

// State is a state implementation doing special handling of the infra-provider specific resources.
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
	infraProviderID, err := st.checkAuthorization(ctx, pointer.Namespace(), pointer.Type())
	if err != nil {
		return nil, err
	}

	res, err := st.innerState.Get(ctx, pointer, option...)
	if err != nil {
		return nil, err
	}

	if infraProviderID == "" {
		return res, nil
	}

	resInfraProviderID, ok := res.Metadata().Labels().Get(omni.LabelInfraProviderID)
	if ok && infraProviderID == resInfraProviderID {
		return res, nil
	}

	return nil, status.Errorf(codes.NotFound, "not found")
}

// List implements state.CoreState interface.
func (st *State) List(ctx context.Context, kind resource.Kind, option ...state.ListOption) (resource.List, error) {
	infraProviderID, err := st.checkAuthorization(ctx, kind.Namespace(), kind.Type())
	if err != nil {
		return resource.List{}, err
	}

	list, err := st.innerState.List(ctx, kind, option...)
	if err != nil {
		return resource.List{}, err
	}

	if infraProviderID == "" {
		return list, nil
	}

	filteredList := make([]resource.Resource, 0, len(list.Items))

	for _, item := range list.Items {
		resInfraProviderID, ok := item.Metadata().Labels().Get(omni.LabelInfraProviderID)
		if ok && infraProviderID == resInfraProviderID {
			filteredList = append(filteredList, item)
		}
	}

	return resource.List{Items: filteredList}, nil
}

// Create implements state.CoreState interface.
func (st *State) Create(ctx context.Context, resource resource.Resource, option ...state.CreateOption) error {
	infraProviderID, err := st.checkAuthorization(ctx, resource.Metadata().Namespace(), resource.Metadata().Type())
	if err != nil {
		return err
	}

	if infraProviderID != "" && resource.Metadata().Type() == infra.MachineRequestType {
		return status.Errorf(codes.PermissionDenied, "infra providers are not allowed to create machine requests")
	}

	if infraProviderID == "" {
		return st.innerState.Create(ctx, resource, option...)
	}

	resource.Metadata().Labels().Set(omni.LabelInfraProviderID, infraProviderID)

	return st.innerState.Create(ctx, resource, option...)
}

// Update implements state.CoreState interface.
func (st *State) Update(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	infraProviderID, err := st.checkAuthorization(ctx, newResource.Metadata().Namespace(), newResource.Metadata().Type())
	if err != nil {
		return err
	}

	oldResource, err := st.innerState.Get(ctx, newResource.Metadata())
	if err != nil {
		return err
	}

	if infraProviderID == "" {
		return st.innerState.Update(ctx, newResource, opts...)
	}

	if newResource.Metadata().Type() == infra.MachineRequestType {
		oldMd := oldResource.Metadata().Copy()
		oldMd.Finalizers().Set(resource.Finalizers{})

		newMd := newResource.Metadata().Copy()
		newMd.Finalizers().Set(resource.Finalizers{})

		if !oldMd.Equal(newMd) {
			return status.Errorf(codes.PermissionDenied, "infra providers are not allowed to update machine requests other than setting finalizers")
		}
	}

	oldResInfraProviderID, ok := oldResource.Metadata().Labels().Get(omni.LabelInfraProviderID)
	if ok && oldResInfraProviderID == infraProviderID {
		newResource.Metadata().Labels().Set(omni.LabelInfraProviderID, infraProviderID)

		return st.innerState.Update(ctx, newResource, opts...)
	}

	return status.Errorf(codes.NotFound, "not found")
}

// Destroy implements state.CoreState interface.
func (st *State) Destroy(ctx context.Context, pointer resource.Pointer, option ...state.DestroyOption) error {
	infraProviderID, err := st.checkAuthorization(ctx, pointer.Namespace(), pointer.Type())
	if err != nil {
		return err
	}

	res, err := st.innerState.Get(ctx, pointer)
	if err != nil {
		return err
	}

	if infraProviderID == "" {
		return st.innerState.Destroy(ctx, pointer, option...)
	}

	resInfraProviderID, ok := res.Metadata().Labels().Get(omni.LabelInfraProviderID)
	if ok && infraProviderID == resInfraProviderID {
		return st.innerState.Destroy(ctx, pointer, option...)
	}

	return status.Errorf(codes.NotFound, "not found")
}

// Watch implements state.CoreState interface.
func (st *State) Watch(ctx context.Context, pointer resource.Pointer, eventCh chan<- state.Event, option ...state.WatchOption) error {
	infraProviderID, err := st.checkAuthorization(ctx, pointer.Namespace(), pointer.Type())
	if err != nil {
		return err
	}

	if infraProviderID == "" {
		return st.innerState.Watch(ctx, pointer, eventCh, option...)
	}

	innerEventCh := st.filterEvents(ctx, infraProviderID, eventCh)

	return st.innerState.Watch(ctx, pointer, innerEventCh, option...)
}

// WatchKind implements state.CoreState interface.
func (st *State) WatchKind(ctx context.Context, kind resource.Kind, eventCh chan<- state.Event, option ...state.WatchKindOption) error {
	infraProviderID, err := st.checkAuthorization(ctx, kind.Namespace(), kind.Type())
	if err != nil {
		return err
	}

	if infraProviderID == "" {
		return st.innerState.WatchKind(ctx, kind, eventCh, option...)
	}

	innerEventCh := st.filterEvents(ctx, infraProviderID, eventCh)

	return st.innerState.WatchKind(ctx, kind, innerEventCh, option...)
}

// WatchKindAggregated implements state.CoreState interface.
func (st *State) WatchKindAggregated(ctx context.Context, kind resource.Kind, eventsCh chan<- []state.Event, option ...state.WatchKindOption) error {
	infraProviderID, err := st.checkAuthorization(ctx, kind.Namespace(), kind.Type())
	if err != nil {
		return err
	}

	if infraProviderID == "" {
		return st.innerState.WatchKindAggregated(ctx, kind, eventsCh, option...)
	}

	innerEventsCh := st.filterEventsAggregated(ctx, infraProviderID, eventsCh)

	return st.innerState.WatchKindAggregated(ctx, kind, innerEventsCh, option...)
}

func (st *State) checkAuthorization(ctx context.Context, ns resource.Namespace, resType resource.Type) (infraProviderID string, err error) {
	if actor.ContextIsInternalActor(ctx) {
		return "", nil
	}

	checkResult, err := auth.CheckGRPC(ctx, auth.WithRole(role.InfraProvider))
	if err != nil {
		return "", err
	}

	// if the role is exactly InfraProvider, additionally, check for the label match
	if checkResult.Role == role.InfraProvider {
		var checkLabel bool

		checkLabel, err = st.checkNamespaceAndType(ns, checkResult.InfraProviderID, resType)
		if err != nil {
			return "", err
		}

		// return the infra provider ID only for the resource live in a shared namespace, i.e., "infra-provider"
		// as their infra provider ID label needs to be checked.
		if checkLabel {
			return checkResult.InfraProviderID, nil
		}
	}

	return "", nil
}

func (st *State) filterEvents(ctx context.Context, infraProviderID string, eventCh chan<- state.Event) chan state.Event {
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
					resInfraProviderID, cpOk := event.Resource.Metadata().Labels().Get(omni.LabelInfraProviderID)
					if !cpOk || infraProviderID != resInfraProviderID {
						continue // discard
					}
				}

				channel.SendWithContext(ctx, eventCh, event)
			}
		}
	}, st.logger)

	return innerEventCh
}

func (st *State) filterEventsAggregated(ctx context.Context, infraProviderID string, eventsCh chan<- []state.Event) chan []state.Event {
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
						resInfraProviderID, cpOk := event.Resource.Metadata().Labels().Get(omni.LabelInfraProviderID)
						if !cpOk || infraProviderID != resInfraProviderID {
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

func (st *State) checkNamespaceAndType(ns resource.Namespace, infraProviderID string, resType resource.Type) (checkLabel bool, err error) {
	if ns == resources.InfraProviderNamespace {
		return true, nil
	}

	infraProviderSpecificNamespace := resources.InfraProviderSpecificNamespacePrefix + infraProviderID
	if ns == infraProviderSpecificNamespace {
		resTypeSuffix := "." + infraProviderID + infraProviderResourceSuffix

		if !strings.HasSuffix(resType, resTypeSuffix) {
			return false, status.Errorf(codes.InvalidArgument, "resources in namespace %q must have a type suffix %q", ns, resTypeSuffix)
		}

		return false, nil
	}

	return false, status.Errorf(codes.PermissionDenied, "namespace not allowed, must be one of %s or %s",
		resources.InfraProviderNamespace, infraProviderSpecificNamespace)
}
