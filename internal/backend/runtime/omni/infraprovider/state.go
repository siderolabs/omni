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
	persistentState state.CoreState
	ephemeralState  state.CoreState
	logger          *zap.Logger
}

// NewState creates a new State.
func NewState(persistentState, ephemeralState state.CoreState, logger *zap.Logger) *State {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &State{
		persistentState: validated.NewState(persistentState, validationOptions()...),
		ephemeralState:  ephemeralState,
		logger:          logger,
	}
}

// Get implements state.CoreState interface.
func (st *State) Get(ctx context.Context, pointer resource.Pointer, option ...state.GetOption) (resource.Resource, error) {
	access, err := st.checkAccess(ctx, pointer.Namespace(), pointer.Type(), true)
	if err != nil {
		return nil, err
	}

	res, err := access.targetState.Get(ctx, pointer, option...)
	if err != nil {
		return nil, err
	}

	if access.infraProviderID == "" {
		return res, nil
	}

	resInfraProviderID, ok := res.Metadata().Labels().Get(omni.LabelInfraProviderID)
	if ok && access.infraProviderID == resInfraProviderID {
		return res, nil
	}

	return nil, status.Errorf(codes.NotFound, "not found")
}

// List implements state.CoreState interface.
func (st *State) List(ctx context.Context, kind resource.Kind, option ...state.ListOption) (resource.List, error) {
	access, err := st.checkAccess(ctx, kind.Namespace(), kind.Type(), true)
	if err != nil {
		return resource.List{}, err
	}

	list, err := access.targetState.List(ctx, kind, option...)
	if err != nil {
		return resource.List{}, err
	}

	if access.infraProviderID == "" {
		return list, nil
	}

	filteredList := make([]resource.Resource, 0, len(list.Items))

	for _, item := range list.Items {
		resInfraProviderID, ok := item.Metadata().Labels().Get(omni.LabelInfraProviderID)
		if ok && access.infraProviderID == resInfraProviderID {
			filteredList = append(filteredList, item)
		}
	}

	return resource.List{Items: filteredList}, nil
}

// Create implements state.CoreState interface.
func (st *State) Create(ctx context.Context, resource resource.Resource, option ...state.CreateOption) error {
	access, err := st.checkAccess(ctx, resource.Metadata().Namespace(), resource.Metadata().Type(), false)
	if err != nil {
		return err
	}

	if access.infraProviderID == "" {
		return access.targetState.Create(ctx, resource, option...)
	}

	if access.config.readOnlyForProviders {
		return status.Errorf(codes.PermissionDenied, "infra providers are not allowed to create %q resources", resource.Metadata().Type())
	}

	if access.config.checkID {
		if resource.Metadata().ID() != access.infraProviderID {
			return status.Errorf(codes.InvalidArgument, "resource ID must match the infra provider ID %q", access.infraProviderID)
		}
	}

	resource.Metadata().Labels().Set(omni.LabelInfraProviderID, access.infraProviderID)

	return access.targetState.Create(ctx, resource, option...)
}

// Update implements state.CoreState interface.
func (st *State) Update(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	access, err := st.checkAccess(ctx, newResource.Metadata().Namespace(), newResource.Metadata().Type(), false)
	if err != nil {
		return err
	}

	oldResource, err := access.targetState.Get(ctx, newResource.Metadata())
	if err != nil {
		return err
	}

	if access.infraProviderID == "" {
		return access.targetState.Update(ctx, newResource, opts...)
	}

	if access.config.readOnlyForProviders {
		if !st.resourcesAreEqual(oldResource, newResource) {
			return status.Errorf(codes.PermissionDenied, "infra providers are not allowed to update %q resources other than setting finalizers", newResource.Metadata().Type())
		}
	}

	oldResInfraProviderID, ok := oldResource.Metadata().Labels().Get(omni.LabelInfraProviderID)
	if ok && oldResInfraProviderID == access.infraProviderID {
		newResource.Metadata().Labels().Set(omni.LabelInfraProviderID, access.infraProviderID)

		return access.targetState.Update(ctx, newResource, opts...)
	}

	return status.Errorf(codes.NotFound, "not found")
}

func (st *State) resourcesAreEqual(res1, res2 resource.Resource) bool {
	var ignoreVersion resource.Version

	md1Copy := res1.Metadata().Copy()
	md2Copy := res2.Metadata().Copy()

	md1Copy.SetVersion(ignoreVersion)
	md2Copy.SetVersion(ignoreVersion)
	md1Copy.Finalizers().Set(resource.Finalizers{})
	md2Copy.Finalizers().Set(resource.Finalizers{})

	if !md1Copy.Equal(md2Copy) {
		return false
	}

	type equaler interface {
		Equal(any) bool
	}

	spec1 := res1.Spec()
	spec2 := res2.Spec()

	if spec1 == spec2 {
		return true
	}

	if res1.Spec() == nil || res2.Spec() == nil {
		return false
	}

	s1, ok := res1.Spec().(equaler)
	if !ok {
		return false
	}

	return s1.Equal(res2.Spec())
}

// Destroy implements state.CoreState interface.
func (st *State) Destroy(ctx context.Context, pointer resource.Pointer, option ...state.DestroyOption) error {
	access, err := st.checkAccess(ctx, pointer.Namespace(), pointer.Type(), false)
	if err != nil {
		return err
	}

	res, err := access.targetState.Get(ctx, pointer)
	if err != nil {
		return err
	}

	if access.infraProviderID == "" {
		return access.targetState.Destroy(ctx, pointer, option...)
	}

	if access.config.readOnlyForProviders {
		return status.Errorf(codes.PermissionDenied, "infra providers are not allowed to destroy %q resources", pointer.Type())
	}

	resInfraProviderID, ok := res.Metadata().Labels().Get(omni.LabelInfraProviderID)
	if ok && access.infraProviderID == resInfraProviderID {
		return access.targetState.Destroy(ctx, pointer, option...)
	}

	return status.Errorf(codes.NotFound, "not found")
}

// Watch implements state.CoreState interface.
func (st *State) Watch(ctx context.Context, pointer resource.Pointer, eventCh chan<- state.Event, option ...state.WatchOption) error {
	access, err := st.checkAccess(ctx, pointer.Namespace(), pointer.Type(), true)
	if err != nil {
		return err
	}

	if access.infraProviderID == "" {
		return access.targetState.Watch(ctx, pointer, eventCh, option...)
	}

	innerEventCh := st.filterEvents(ctx, access.infraProviderID, eventCh)

	return access.targetState.Watch(ctx, pointer, innerEventCh, option...)
}

// WatchKind implements state.CoreState interface.
func (st *State) WatchKind(ctx context.Context, kind resource.Kind, eventCh chan<- state.Event, option ...state.WatchKindOption) error {
	access, err := st.checkAccess(ctx, kind.Namespace(), kind.Type(), true)
	if err != nil {
		return err
	}

	if access.infraProviderID == "" {
		return access.targetState.WatchKind(ctx, kind, eventCh, option...)
	}

	innerEventCh := st.filterEvents(ctx, access.infraProviderID, eventCh)

	return access.targetState.WatchKind(ctx, kind, innerEventCh, option...)
}

// WatchKindAggregated implements state.CoreState interface.
func (st *State) WatchKindAggregated(ctx context.Context, kind resource.Kind, eventsCh chan<- []state.Event, option ...state.WatchKindOption) error {
	access, err := st.checkAccess(ctx, kind.Namespace(), kind.Type(), true)
	if err != nil {
		return err
	}

	if access.infraProviderID == "" {
		return access.targetState.WatchKindAggregated(ctx, kind, eventsCh, option...)
	}

	innerEventsCh := st.filterEventsAggregated(ctx, access.infraProviderID, eventsCh)

	return access.targetState.WatchKindAggregated(ctx, kind, innerEventsCh, option...)
}

type accessCheckResult struct {
	targetState     state.CoreState
	infraProviderID string
	config          resourceConfig
}

func (st *State) checkAccess(ctx context.Context, ns resource.Namespace, resType resource.Type, isReadAccess bool) (accessCheckResult, error) {
	config, ok := getResourceConfig(ns, resType)
	if !ok {
		return accessCheckResult{}, status.Errorf(codes.PermissionDenied, "internal error: resource %q is not an infra provider managed resource", resType)
	}

	targetState := st.persistentState
	if config.ephemeral {
		targetState = st.ephemeralState
	}

	if actor.ContextIsInternalActor(ctx) {
		return accessCheckResult{
			config:      config,
			targetState: targetState,
		}, nil
	}

	checkResult, err := auth.CheckGRPC(ctx, auth.WithRole(role.InfraProvider))
	if err != nil {
		return accessCheckResult{}, err
	}

	// if the role is exactly InfraProvider, additionally, check for the label match
	if checkResult.Role == role.InfraProvider {
		var checkLabel bool

		checkLabel, err = st.checkNamespaceAndType(ns, checkResult.InfraProviderID, resType)
		if err != nil {
			return accessCheckResult{}, err
		}

		// return the infra provider ID only for the resource live in a shared namespace, i.e., "infra-provider"
		// as their infra provider ID label needs to be checked.
		if checkLabel {
			return accessCheckResult{
				infraProviderID: checkResult.InfraProviderID,
				config:          config,
				targetState:     targetState,
			}, nil
		}
	} else if !isReadAccess { // not an infra provider, check for the read-only access
		return accessCheckResult{}, status.Errorf(codes.PermissionDenied, "only infra providers are allowed to modify %q resources", resType)
	}

	return accessCheckResult{
		config:      config,
		targetState: targetState,
	}, nil
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
	switch ns {
	case resources.InfraProviderNamespace, resources.InfraProviderEphemeralNamespace:
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

	return false, status.Errorf(codes.PermissionDenied, "namespace not allowed, must be one of: %s, %s, %s",
		resources.InfraProviderNamespace, resources.InfraProviderEphemeralNamespace, infraProviderSpecificNamespace)
}

// resourceConfig defines the resource-specific configuration to be validated by the state.
type resourceConfig struct {
	readOnlyForProviders bool
	ephemeral            bool
	checkID              bool
}

// IsInfraProviderResource returns true if the given resource type is an infra provider specific resource.
func IsInfraProviderResource(ns resource.Namespace, resType resource.Type) bool {
	_, isInfraProviderResource := getResourceConfig(ns, resType)

	return isInfraProviderResource
}

// getResourceConfig returns the configuration for the given resource type.
func getResourceConfig(ns resource.Namespace, resType resource.Type) (config resourceConfig, isInfraProviderResource bool) {
	if strings.HasPrefix(ns, resources.InfraProviderSpecificNamespacePrefix) {
		return resourceConfig{}, true
	}

	switch resType {
	case infra.MachineRequestType:
		return resourceConfig{
			readOnlyForProviders: true,
		}, true
	case infra.MachineRequestStatusType:
		return resourceConfig{}, true
	case infra.InfraMachineType:
		return resourceConfig{
			readOnlyForProviders: true,
		}, true
	case infra.InfraMachineStatusType:
		return resourceConfig{}, true
	case infra.InfraProviderStatusType:
		return resourceConfig{
			checkID: true,
		}, true
	case infra.InfraProviderHealthStatusType:
		return resourceConfig{
			ephemeral: ns == resources.InfraProviderEphemeralNamespace,
			checkID:   true,
		}, true
	case infra.ConfigPatchRequestType:
		return resourceConfig{}, true
	default:
		return resourceConfig{}, false
	}
}
