// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package cloudprovider_test

import (
	"context"
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

	cloudspecs "github.com/siderolabs/omni/client/api/omni/specs/cloud"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/cloud"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/cloudprovider"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/validated"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

const (
	cloudProviderID           = "qemu-1"
	talosVersion              = "v1.2.3"
	schematicID               = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	cloudProviderResNamespace = resources.CloudProviderSpecificNamespacePrefix + cloudProviderID
)

func TestCloudProviderAccess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	ctx = prepareCloudProviderServiceAccount(ctx)

	logger := zaptest.NewLogger(t)
	innerSt := namespaced.NewState(inmem.Build)
	st := state.WrapCore(cloudprovider.NewState(innerSt, logger))

	// MachineRequest

	mr := cloud.NewMachineRequest("test-mr")

	// create
	err := st.Create(ctx, mr)
	assert.ErrorContains(t, err, "cloud providers are not allowed to create machine requests")

	// prepare for update
	mr.Metadata().Labels().Set(omni.LabelCloudProviderID, cloudProviderID)

	mr.TypedSpec().Value.TalosVersion = talosVersion
	mr.TypedSpec().Value.SchematicId = schematicID

	require.NoError(t, innerSt.Create(ctx, mr))

	// update spec
	_, err = safe.StateUpdateWithConflicts(ctx, st, mr.Metadata(), func(res *cloud.MachineRequest) error {
		res.TypedSpec().Value.TalosVersion = "v1.2.4"

		return nil
	})
	assert.True(t, validated.IsValidationError(err))
	assert.ErrorContains(t, err, "machine request spec is immutable")

	// update metadata labels
	_, err = safe.StateUpdateWithConflicts(ctx, st, mr.Metadata(), func(res *cloud.MachineRequest) error {
		res.Metadata().Labels().Set("foo", "bar")

		return nil
	})
	assert.ErrorContains(t, err, "cloud providers are not allowed to update machine requests other than setting finalizers")

	// update metadata - add finalizer
	_, err = safe.StateUpdateWithConflicts(ctx, st, mr.Metadata(), func(res *cloud.MachineRequest) error {
		res.Metadata().Finalizers().Add("foobar")

		return nil
	})
	assert.NoError(t, err)

	// MachineRequestStatus

	mrs := cloud.NewMachineRequestStatus("test-mrs")

	// create
	assert.NoError(t, st.Create(ctx, mrs))

	// assert that the label is set
	res, err := innerSt.Get(ctx, mrs.Metadata())
	require.NoError(t, err)

	cpID, _ := res.Metadata().Labels().Get(omni.LabelCloudProviderID)
	assert.Equal(t, cloudProviderID, cpID)

	// update
	_, err = safe.StateUpdateWithConflicts(ctx, st, mrs.Metadata(), func(res *cloud.MachineRequestStatus) error {
		res.Metadata().Labels().Set("foo", "bar")

		res.TypedSpec().Value.Id = "12345"
		res.TypedSpec().Value.Stage = cloudspecs.MachineRequestStatusSpec_PROVISIONING

		return nil
	})
	assert.NoError(t, err)
}

func TestInternalAccess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	ctx = actor.MarkContextAsInternalActor(ctx)

	logger := zaptest.NewLogger(t)

	innerSt := namespaced.NewState(inmem.Build)
	st := state.WrapCore(cloudprovider.NewState(innerSt, logger))
	mr := cloud.NewMachineRequest("test-mr")

	err := st.Create(ctx, mr)
	assert.True(t, validated.IsValidationError(err))
	assert.ErrorContains(t, err, "invalid talos version format")
	assert.ErrorContains(t, err, "invalid schematic ID format")

	mr.TypedSpec().Value.TalosVersion = talosVersion
	mr.TypedSpec().Value.SchematicId = schematicID

	err = st.Create(ctx, mr)
	assert.NoError(t, err)
}

func TestCloudProviderSpecificNamespace(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	ctx = prepareCloudProviderServiceAccount(ctx)

	logger := zaptest.NewLogger(t)
	innerSt := namespaced.NewState(inmem.Build)
	st := state.WrapCore(cloudprovider.NewState(innerSt, logger))

	// try to create and update a resource in the cloud-provider specific namespace, i.e., "cloud-provider:qemu-1", assert that it is allowed

	res1 := newTestRes(cloudProviderResNamespace, "test-res-1", testResSpec{str: "foo"})

	require.NoError(t, st.Create(ctx, res1))

	_, err := safe.StateUpdateWithConflicts(ctx, st, res1.Metadata(), func(res *testRes) error {
		res.TypedSpec().str = "bar"

		return nil
	})
	assert.NoError(t, err)

	assert.NoError(t, st.Destroy(ctx, res1.Metadata()))

	// try to create a resource in the cloud-provider specific namespace of a different cloud provider, i.e., "cloud-provider:qemu-2", assert that it is not allowed

	res2 := newTestRes(resources.CloudProviderSpecificNamespacePrefix+"qemu-2", "test-res-2", testResSpec{str: "foo"})

	err = st.Create(ctx, res2)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.ErrorContains(t, err, "namespace not allowed, must be one of")

	// try to create a resource with omni-internal type, i.e., "ExposedServices.omni.sidero.dev" in the cloud-provider specific namespace - assert that it is not allowed

	omniRes := omni.NewExposedService(cloudProviderResNamespace, "test-res-3")

	err = st.Create(ctx, omniRes)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.ErrorContains(t, err, `resources in namespace "cloud-provider:qemu-1" must have a type suffix ".qemu-1.cloudprovider.sidero.dev"`)
}

func TestCloudProviderIDChecks(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	ctx = prepareCloudProviderServiceAccount(ctx)

	logger := zaptest.NewLogger(t)
	innerSt := namespaced.NewState(inmem.Build)
	st := state.WrapCore(cloudprovider.NewState(innerSt, logger))

	prepareResources(ctx, t, innerSt)

	// Get - assert that it is checked against cloud provider id

	_, err := st.Get(ctx, cloud.NewMachineRequest("mr-1").Metadata())
	assert.NoError(t, err)

	_, err = st.Get(ctx, cloud.NewMachineRequest("mr-2").Metadata())
	assert.Equal(t, codes.NotFound, status.Code(err))

	// List - assert that it is filtered by cloud provider id

	list, err := st.List(ctx, cloud.NewMachineRequest("").Metadata())
	assert.NoError(t, err)

	if assert.Len(t, list.Items, 1) {
		assert.Equal(t, "mr-1", list.Items[0].Metadata().ID())
	}

	// Watch - assert that it is filtered by cloud provider id

	watchCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	t.Cleanup(cancel)

	eventCh := make(chan state.Event)

	err = st.Watch(watchCtx, cloud.NewMachineRequest("mr-1").Metadata(), eventCh)
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

	err = st.Watch(watchCtx, cloud.NewMachineRequest("mr-2").Metadata(), eventCh)
	require.NoError(t, err)

	assertEvents(watchCtx, t, eventCh, nil)

	cancel()

	// WatchKind - assert that it is filtered by cloud provider id

	watchCtx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
	t.Cleanup(cancel)

	eventCh = make(chan state.Event)

	err = st.WatchKind(watchCtx, cloud.NewMachineRequest("").Metadata(), eventCh, state.WithBootstrapContents(true))
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

	// Destroy - assert that it is checked against cloud provider id

	err = st.Destroy(ctx, cloud.NewMachineRequest("mr-1").Metadata())
	assert.NoError(t, err)

	err = st.Destroy(ctx, cloud.NewMachineRequest("mr-2").Metadata())
	assert.Equal(t, codes.NotFound, status.Code(err))
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

func prepareResources(ctx context.Context, t *testing.T, innerSt state.CoreState) {
	mr1 := cloud.NewMachineRequest("mr-1")
	mr1.TypedSpec().Value.TalosVersion = talosVersion
	mr1.TypedSpec().Value.SchematicId = schematicID

	mr1.Metadata().Labels().Set(omni.LabelCloudProviderID, cloudProviderID)

	mr2 := cloud.NewMachineRequest("mr-2")
	mr2.TypedSpec().Value.TalosVersion = "v1.2.4"
	mr2.TypedSpec().Value.SchematicId = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	mr2.Metadata().Labels().Set(omni.LabelCloudProviderID, "aws-2")

	require.NoError(t, innerSt.Create(ctx, mr1))
	require.NoError(t, innerSt.Create(ctx, mr2))
}

func prepareCloudProviderServiceAccount(ctx context.Context) context.Context {
	fullID := cloudProviderID + "@cloud-provider.serviceaccount.omni.sidero.dev"

	ctx = ctxstore.WithValue(ctx, auth.EnabledAuthContextKey{Enabled: true})
	ctx = ctxstore.WithValue(ctx, auth.IdentityContextKey{Identity: fullID})
	ctx = ctxstore.WithValue(ctx, auth.VerifiedEmailContextKey{Email: fullID})
	ctx = ctxstore.WithValue(ctx, auth.RoleContextKey{Role: role.CloudProvider})

	return ctx
}

// testResType is the type of testRes.
const testResType = resource.Type("TestRess." + cloudProviderID + ".cloudprovider.sidero.dev")

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
		DefaultNamespace: cloudProviderResNamespace,
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
