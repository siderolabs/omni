// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package infraprovider_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/infraprovider"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

const (
	infraProviderID           = "qemu-1"
	talosVersion              = "v1.2.3"
	infraProviderResNamespace = resources.InfraProviderSpecificNamespacePrefix + infraProviderID
)

func TestInfraProviderAccess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	ctx = prepareInfraProviderServiceAccount(ctx)

	logger := zaptest.NewLogger(t)
	persistentState := namespaced.NewState(inmem.Build)
	ephemeralState := namespaced.NewState(inmem.Build)
	st := state.WrapCore(infraprovider.NewState(persistentState, ephemeralState, logger))

	// MachineRequest

	mr := infra.NewMachineRequest("test-mr")

	testInfraProviderAccessInputResource(ctx, t, st, persistentState, mr, func(res *infra.MachineRequest) {
		res.TypedSpec().Value.TalosVersion = talosVersion
	}, func(res *infra.MachineRequest) error {
		res.TypedSpec().Value.TalosVersion = "v1.2.4"

		return nil
	}, func(t *testing.T, err error) {
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
		assert.ErrorContains(t, err, `infra providers are not allowed to update "MachineRequests.omni.sidero.dev" resources other than setting finalizers`)
	})

	// InfraMachine

	infraMachine := infra.NewMachine("test-im")

	testInfraProviderAccessInputResource(ctx, t, st, persistentState, infraMachine, func(res *infra.Machine) {
		res.TypedSpec().Value.PreferredPowerState = specs.InfraMachineSpec_POWER_STATE_ON
	}, func(res *infra.Machine) error {
		res.TypedSpec().Value.PreferredPowerState = specs.InfraMachineSpec_POWER_STATE_OFF

		return nil
	}, func(t *testing.T, err error) {
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
		assert.ErrorContains(t, err, `infra providers are not allowed to update "InfraMachines.omni.sidero.dev" resources other than setting finalizers`)
	})

	// MachineRequestStatus

	testInfraProviderAccessOutputResource(ctx, t, st, persistentState, infra.NewMachineRequestStatus("test-mrs"), func(res *infra.MachineRequestStatus) error {
		res.TypedSpec().Value.Id = "12345"
		res.TypedSpec().Value.Stage = specs.MachineRequestStatusSpec_PROVISIONING

		return nil
	})

	// InfraMachineStatus

	testInfraProviderAccessOutputResource(ctx, t, st, persistentState, infra.NewMachineStatus("test-ims"), func(res *infra.MachineStatus) error {
		res.TypedSpec().Value.PowerState = specs.InfraMachineStatusSpec_POWER_STATE_ON

		return nil
	})

	// InfraProviderStatus

	status := infra.NewProviderStatus("invalid-id")

	err := st.Create(ctx, status)
	assert.ErrorContains(t, err, fmt.Sprintf(`resource ID must match the infra provider ID "%s"`, infraProviderID))

	status = infra.NewProviderStatus(infraProviderID)

	// create
	assert.NoError(t, st.Create(ctx, status))

	status.TypedSpec().Value.Name = "aa"

	// update
	assert.NoError(t, st.Update(ctx, status))

	// ConfigPatchRequest

	cpr := infra.NewConfigPatchRequest("test-cpr")

	// create
	assert.NoError(t, st.Create(ctx, cpr))

	// assert that the label is set
	res, err := persistentState.Get(ctx, cpr.Metadata())
	require.NoError(t, err)

	cpID, _ := res.Metadata().Labels().Get(omni.LabelInfraProviderID)
	assert.Equal(t, infraProviderID, cpID)

	// update
	_, err = safe.StateUpdateWithConflicts(ctx, st, cpr.Metadata(), func(res *infra.ConfigPatchRequest) error {
		res.Metadata().Labels().Set("foo", "bar")

		return res.TypedSpec().Value.SetUncompressedData([]byte("{}"))
	})
	assert.NoError(t, err)
}

func testInfraProviderAccessInputResource[T resource.Resource](ctx context.Context, t *testing.T, st state.State, persistentState state.CoreState,
	res T, prepareRes func(res T), updateRes func(res T) error, assertUpdateResult func(*testing.T, error),
) {
	// create
	err := st.Create(ctx, res)
	assert.ErrorContains(t, err, fmt.Sprintf("infra providers are not allowed to create %q resources", res.Metadata().Type()))

	// prepare for update
	res.Metadata().Labels().Set(omni.LabelInfraProviderID, infraProviderID)

	prepareRes(res)

	require.NoError(t, persistentState.Create(ctx, res))

	// update spec
	_, err = safe.StateUpdateWithConflicts(ctx, st, res.Metadata(), updateRes)

	assertUpdateResult(t, err)

	// update metadata labels
	_, err = st.UpdateWithConflicts(ctx, res.Metadata(), func(res resource.Resource) error {
		res.Metadata().Labels().Set("foo", "bar")

		return nil
	})
	assert.ErrorContains(t, err, fmt.Sprintf("infra providers are not allowed to update %q resources other than setting finalizers", res.Metadata().Type()))

	// update metadata - add finalizer
	_, err = st.UpdateWithConflicts(ctx, res.Metadata(), func(res resource.Resource) error {
		res.Metadata().Finalizers().Add("foobar")

		return nil
	})
	assert.NoError(t, err)
}

func testInfraProviderAccessOutputResource[T resource.Resource](ctx context.Context, t *testing.T, st state.State, persistentState state.CoreState, res T, updateRes func(res T) error) {
	// create
	assert.NoError(t, st.Create(ctx, res))

	// assert that the label is set
	resAfterCreate, err := persistentState.Get(ctx, res.Metadata())
	require.NoError(t, err)

	cpID, _ := resAfterCreate.Metadata().Labels().Get(omni.LabelInfraProviderID)
	assert.Equal(t, infraProviderID, cpID)

	// update
	_, err = safe.StateUpdateWithConflicts(ctx, st, res.Metadata(), updateRes)
	assert.NoError(t, err)
}

func TestInternalAccess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	ctx = actor.MarkContextAsInternalActor(ctx)

	logger := zaptest.NewLogger(t)

	persistentState := namespaced.NewState(inmem.Build)
	ephemeralState := namespaced.NewState(inmem.Build)
	st := state.WrapCore(infraprovider.NewState(persistentState, ephemeralState, logger))
	mr := infra.NewMachineRequest("test-mr")

	err := st.Create(ctx, mr)
	assert.True(t, validated.IsValidationError(err))
	assert.ErrorContains(t, err, "invalid talos version format")

	mr.TypedSpec().Value.TalosVersion = talosVersion

	err = st.Create(ctx, mr)
	assert.NoError(t, err)

	_, err = safe.StateUpdateWithConflicts(ctx, st, mr.Metadata(), func(res *infra.MachineRequest) error {
		res.TypedSpec().Value.TalosVersion = "v1.2.5"

		return nil
	})
	assert.True(t, validated.IsValidationError(err))
	assert.ErrorContains(t, err, "machine request spec is immutable")
}

func TestInfraProviderSpecificNamespace(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	ctx = prepareInfraProviderServiceAccount(ctx)

	logger := zaptest.NewLogger(t)
	persistentState := namespaced.NewState(inmem.Build)
	ephemeralState := namespaced.NewState(inmem.Build)
	st := state.WrapCore(infraprovider.NewState(persistentState, ephemeralState, logger))

	// try to create and update a resource in the infra-provider specific namespace, i.e., "infra-provider:qemu-1", assert that it is allowed

	res1 := newTestRes(infraProviderResNamespace, "test-res-1", testResSpec{str: "foo"})

	require.True(t, infraprovider.IsInfraProviderResource(infraProviderResNamespace, res1.Metadata().Type()))
	require.NoError(t, st.Create(ctx, res1))

	_, err := safe.StateUpdateWithConflicts(ctx, st, res1.Metadata(), func(res *testRes) error {
		res.TypedSpec().str = "bar"

		return nil
	})
	assert.NoError(t, err)

	assert.NoError(t, st.Destroy(ctx, res1.Metadata()))

	// try to create a resource in the infra-provider specific namespace of a different infra provider, i.e., "infra-provider:qemu-2", assert that it is not allowed

	res2 := newTestRes(resources.InfraProviderSpecificNamespacePrefix+"qemu-2", "test-res-2", testResSpec{str: "foo"})

	err = st.Create(ctx, res2)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.ErrorContains(t, err, "namespace not allowed, must be one of")

	// try to create a resource with omni-internal type, i.e., "ExposedServices.omni.sidero.dev" in the infra-provider specific namespace - assert that it is not allowed

	omniRes := omni.NewExposedService(infraProviderResNamespace, "test-res-3")

	err = st.Create(ctx, omniRes)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.ErrorContains(t, err, `resources in namespace "infra-provider:qemu-1" must have a type suffix ".qemu-1.infraprovider.sidero.dev"`)
}

func TestInfraProviderIDChecks(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	ctx = prepareInfraProviderServiceAccount(ctx)

	logger := zaptest.NewLogger(t)
	persistentState := namespaced.NewState(inmem.Build)
	ephemeralState := namespaced.NewState(inmem.Build)
	st := state.WrapCore(infraprovider.NewState(persistentState, ephemeralState, logger))

	prepareResources(ctx, t, persistentState)

	// Get - assert that it is checked against infra provider id

	_, err := st.Get(ctx, infra.NewMachineRequest("mr-1").Metadata())
	assert.NoError(t, err)

	_, err = st.Get(ctx, infra.NewMachineRequest("mr-2").Metadata())
	assert.Equal(t, codes.NotFound, status.Code(err))

	// List - assert that it is filtered by infra provider id

	list, err := st.List(ctx, infra.NewMachineRequest("").Metadata())
	assert.NoError(t, err)

	if assert.Len(t, list.Items, 1) {
		assert.Equal(t, "mr-1", list.Items[0].Metadata().ID())
	}

	// Watch - assert that it is filtered by infra provider id

	watchCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	t.Cleanup(cancel)

	eventCh := make(chan state.Event)

	err = st.Watch(watchCtx, infra.NewMachineRequest("mr-1").Metadata(), eventCh)
	require.NoError(t, err)

	assertEvents(watchCtx, t, eventCh, []eventInfo{
		{
			ID:   "mr-1",
			Type: state.Created,
		},
	})

	cancel()

	watchCtx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
	t.Cleanup(cancel)

	eventCh = make(chan state.Event)

	err = st.Watch(watchCtx, infra.NewMachineRequest("mr-2").Metadata(), eventCh)
	require.NoError(t, err)

	assertEvents(watchCtx, t, eventCh, nil)

	cancel()

	// WatchKind - assert that it is filtered by infra provider id

	watchCtx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
	t.Cleanup(cancel)

	eventCh = make(chan state.Event)

	err = st.WatchKind(watchCtx, infra.NewMachineRequest("").Metadata(), eventCh, state.WithBootstrapContents(true))
	require.NoError(t, err)

	assertEvents(watchCtx, t, eventCh, []eventInfo{
		{
			ID:   "mr-1",
			Type: state.Created,
		},
		{
			Type: state.Bootstrapped,
		},
	})

	cancel()

	// Destroy - assert that it is checked against infra provider id.
	// We check them on MachineRequestStatus resources, as MachineRequest resources are read-only for infra providers.

	err = st.Destroy(ctx, infra.NewMachineRequestStatus("mrs-1").Metadata())
	assert.NoError(t, err)

	err = st.Destroy(ctx, infra.NewMachineRequestStatus("mrs-2").Metadata())
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestEphemeralState(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	ctx = prepareInfraProviderServiceAccount(ctx)

	logger := zaptest.NewLogger(t)
	persistentState := namespaced.NewState(inmem.Build)
	ephemeralState := namespaced.NewState(inmem.Build)
	st := state.WrapCore(infraprovider.NewState(persistentState, ephemeralState, logger))

	ephemeralResource := infra.NewProviderHealthStatus(infraProviderID)

	require.NoError(t, st.Create(ctx, ephemeralResource))

	_, err := persistentState.Get(ctx, ephemeralResource.Metadata())
	assert.True(t, state.IsNotFoundError(err), "ephemeral resource must not be stored in the persistent state")

	_, err = st.Get(ctx, ephemeralResource.Metadata())
	require.NoError(t, err)

	ephemeralResource.TypedSpec().Value.Error = "test"

	err = st.Update(ctx, ephemeralResource)
	require.NoError(t, err)

	err = st.Destroy(ctx, ephemeralResource.Metadata())
	require.NoError(t, err)
}

type eventInfo struct {
	ID   resource.ID
	Type state.EventType
}

func assertEvents(ctx context.Context, t *testing.T, eventCh chan state.Event, expectedEvents []eventInfo) {
	for {
		select {
		case <-ctx.Done():
			if len(expectedEvents) > 0 {
				t.Fatalf("expected %d more events", len(expectedEvents))
			}

			return
		case event := <-eventCh:
			if event.Error != nil {
				require.Fail(t, "unexpected error: %v", event.Error)
			}

			if len(expectedEvents) == 0 {
				require.Fail(t, "unexpected event")
			}

			expected := expectedEvents[0]
			expectedEvents = expectedEvents[1:]

			assert.Equal(t, expected.Type, event.Type)

			if expected.Type != state.Bootstrapped {
				assert.Equal(t, expected.ID, event.Resource.Metadata().ID())
			}
		}
	}
}

func prepareResources(ctx context.Context, t *testing.T, persistentState state.CoreState) {
	mr1 := infra.NewMachineRequest("mr-1")
	mr1.TypedSpec().Value.TalosVersion = talosVersion

	mr1.Metadata().Labels().Set(omni.LabelInfraProviderID, infraProviderID)

	mr2 := infra.NewMachineRequest("mr-2")
	mr2.TypedSpec().Value.TalosVersion = "v1.2.4"

	mr2.Metadata().Labels().Set(omni.LabelInfraProviderID, "aws-2")

	mrs1 := infra.NewMachineRequestStatus("mrs-1")
	mrs1.TypedSpec().Value.Id = "12345"

	mrs1.Metadata().Labels().Set(omni.LabelInfraProviderID, infraProviderID)

	mrs2 := infra.NewMachineRequestStatus("mrs-2")
	mrs2.TypedSpec().Value.Id = "67890"

	mrs2.Metadata().Labels().Set(omni.LabelInfraProviderID, "aws-2")

	require.NoError(t, persistentState.Create(ctx, mr1))
	require.NoError(t, persistentState.Create(ctx, mr2))
	require.NoError(t, persistentState.Create(ctx, mrs1))
	require.NoError(t, persistentState.Create(ctx, mrs2))
}

func prepareInfraProviderServiceAccount(ctx context.Context) context.Context {
	fullID := infraProviderID + "@infra-provider.serviceaccount.omni.sidero.dev"

	ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: true})
	ctx = ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: fullID})
	ctx = ctxstore.WithValue(ctx, auth.VerifiedEmailContextKey{Email: fullID})
	ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: role.InfraProvider})

	return ctx
}

// testResType is the type of testRes.
const testResType = resource.Type("TestRess." + infraProviderID + ".infraprovider.sidero.dev")

// testRes is a test resource.
type testRes = typed.Resource[testResSpec, testResExtension]

// NewA initializes a testRes resource.
func newTestRes(ns resource.Namespace, id resource.ID, spec testResSpec) *testRes {
	return typed.NewResource[testResSpec, testResExtension](
		resource.NewMetadata(ns, testResType, id, resource.VersionUndefined),
		spec,
	)
}

// testResExtension provides auxiliary methods for testRes.
type testResExtension struct{}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (testResExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             testResType,
		DefaultNamespace: infraProviderResNamespace,
	}
}

// testResSpec provides testRes definition.
type testResSpec struct {
	str string
}

// DeepCopy generates a deep copy of testResSpec.
func (t testResSpec) DeepCopy() testResSpec {
	return t
}
