// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type MachineRequestSetStatusSuite struct {
	OmniSuite
}

type testInfraProvider = qtransform.QController[*infra.MachineRequest, *infra.MachineRequestStatus]

func newTestInfraProvider() *testInfraProvider {
	return qtransform.NewQController(
		qtransform.Settings[*infra.MachineRequest, *infra.MachineRequestStatus]{
			Name: "testInfraProvider",
			MapMetadataFunc: func(request *infra.MachineRequest) *infra.MachineRequestStatus {
				return infra.NewMachineRequestStatus(request.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *infra.MachineRequestStatus) *infra.MachineRequest {
				return infra.NewMachineRequest(status.Metadata().ID())
			},
			TransformExtraOutputFunc: func(_ context.Context, _ controller.ReaderWriter, _ *zap.Logger, machineRequest *infra.MachineRequest,
				machineRequestStatus *infra.MachineRequestStatus,
			) error {
				if machineRequestStatus.TypedSpec().Value.Id == "" {
					machineRequestStatus.TypedSpec().Value.Id = uuid.New().String()
				}

				machineRequestStatus.TypedSpec().Value.Stage = specs.MachineRequestStatusSpec_PROVISIONED

				helpers.CopyAllLabels(machineRequest, machineRequestStatus)

				return nil
			},
		},
	)
}

func (suite *MachineRequestSetStatusSuite) TestReconcile() {
	require := suite.Require()

	ctx, cancel := context.WithTimeout(suite.ctx, time.Second*10)
	defer cancel()

	suite.startRuntime()

	var eg errgroup.Group

	reconcilerCtx, reconcilerCancel := context.WithCancel(ctx)

	defer func() {
		reconcilerCancel()

		suite.Require().NoError(eg.Wait())
	}()

	eg.Go(func() error {
		if err := suite.reconcileLabels(reconcilerCtx); err != nil {
			suite.T().Logf("machine status labels reconciler crashed with error %s", err)

			return err
		}

		return nil
	})

	require.NoError(suite.runtime.RegisterQController(omnictrl.NewMachineRequestSetStatusController()))
	require.NoError(suite.runtime.RegisterQController(newTestInfraProvider()))

	machineRequestSet := omni.NewMachineRequestSet(resources.DefaultNamespace, "test")

	machineRequestSet.TypedSpec().Value.ProviderId = "test"
	machineRequestSet.TypedSpec().Value.TalosVersion = "v1.7.5"

	suite.Require().NoError(suite.state.Create(ctx, machineRequestSet))

	var err error

	// scale up
	machineRequestSet, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineRequestSet.Metadata(), func(r *omni.MachineRequestSet) error {
		r.TypedSpec().Value.MachineCount = 4

		return nil
	})

	suite.Require().NoError(err)

	var ids []resource.ID

	err = retry.Constant(time.Second*5).RetryWithContext(ctx, func(ctx context.Context) error {
		ids = rtestutils.ResourceIDs[*infra.MachineRequest](ctx, suite.T(), suite.state, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineRequestSet, machineRequestSet.Metadata().ID())))

		if len(ids) != int(machineRequestSet.TypedSpec().Value.MachineCount) {
			return retry.ExpectedErrorf("expected %d requests got %d", machineRequestSet.TypedSpec().Value.MachineCount, len(ids))
		}

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, ids, func(r *infra.MachineRequest, assert *assert.Assertions) {
		l, ok := r.Metadata().Labels().Get(omni.LabelMachineRequestSet)

		assert.True(ok)
		assert.Equal(l, machineRequestSet.Metadata().ID())

		l, ok = r.Metadata().Labels().Get(omni.LabelInfraProviderID)

		assert.True(ok)
		assert.Equal(l, machineRequestSet.TypedSpec().Value.ProviderId)

		assert.Equal(machineRequestSet.TypedSpec().Value.TalosVersion, r.TypedSpec().Value.TalosVersion)
	})

	rtestutils.AssertResources(ctx, suite.T(), suite.state, ids, func(*infra.MachineRequestStatus, *assert.Assertions) {})

	requestStatuses, err := safe.ReaderListAll[*infra.MachineRequestStatus](ctx, suite.state,
		state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineRequestSet, machineRequestSet.Metadata().ID())),
	)

	suite.Require().NoError(err)

	machineIDs := []string{requestStatuses.Get(0).TypedSpec().Value.Id}

	// remove the first request link
	rtestutils.AssertResources(ctx, suite.T(), suite.state, machineIDs, func(r *system.ResourceLabels[*omni.MachineStatus], assertion *assert.Assertions) {
		assertion.True(r.Metadata().Finalizers().Has(omnictrl.MachineRequestSetStatusControllerName), r.Metadata().ID())
	})

	rtestutils.Destroy[*system.ResourceLabels[*omni.MachineStatus]](ctx, suite.T(), suite.state, machineIDs)

	rtestutils.AssertNoResource[*infra.MachineRequest](ctx, suite.T(), suite.state, ids[0])

	err = retry.Constant(time.Second*5).RetryWithContext(ctx, func(ctx context.Context) error {
		ids = rtestutils.ResourceIDs[*infra.MachineRequest](ctx, suite.T(), suite.state, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineRequestSet, machineRequestSet.Metadata().ID())))

		if len(ids) != int(machineRequestSet.TypedSpec().Value.MachineCount) {
			return retry.ExpectedErrorf("expected %d requests got %d", machineRequestSet.TypedSpec().Value.MachineCount, len(ids))
		}

		return nil
	})

	suite.Require().NoError(err)

	rtestutils.AssertResources(ctx, suite.T(), suite.state, ids, func(r *infra.MachineRequest, assert *assert.Assertions) {
		l, ok := r.Metadata().Labels().Get(omni.LabelMachineRequestSet)

		assert.True(ok)
		assert.Equal(l, machineRequestSet.Metadata().ID())

		l, ok = r.Metadata().Labels().Get(omni.LabelInfraProviderID)

		assert.True(ok)
		assert.Equal(l, machineRequestSet.TypedSpec().Value.ProviderId)

		assert.Equal(machineRequestSet.TypedSpec().Value.TalosVersion, r.TypedSpec().Value.TalosVersion)
	})

	// scale down
	machineRequestSet, err = safe.StateUpdateWithConflicts(ctx, suite.state, machineRequestSet.Metadata(), func(r *omni.MachineRequestSet) error {
		r.TypedSpec().Value.MachineCount = 2

		return nil
	})

	suite.Require().NoError(err)

	err = retry.Constant(time.Second*5).RetryWithContext(ctx, func(ctx context.Context) error {
		ids = rtestutils.ResourceIDs[*infra.MachineRequest](ctx, suite.T(), suite.state, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachineRequestSet, machineRequestSet.Metadata().ID())))

		if len(ids) != int(machineRequestSet.TypedSpec().Value.MachineCount) {
			return retry.ExpectedErrorf("expected %d requests got %d", machineRequestSet.TypedSpec().Value.MachineCount, len(ids))
		}

		return nil
	})

	suite.Require().NoError(err)

	// remove the machine request set
	rtestutils.DestroyAll[*omni.MachineRequestSet](ctx, suite.T(), suite.state)

	requests, err := safe.ReaderListAll[*infra.MachineRequest](ctx, suite.state)
	suite.Require().NoError(err)

	suite.Require().True(requests.Len() == 0)
}

//nolint:gocognit
func (suite *MachineRequestSetStatusSuite) reconcileLabels(ctx context.Context) error {
	ch := make(chan state.Event)

	err := suite.state.WatchKind(ctx, infra.NewMachineRequestStatus("").Metadata(), ch)
	if err != nil {
		return err
	}

	deleteLabels := func(id string) error {
		res := system.NewResourceLabels[*omni.MachineStatus](id)

		deleteCtx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()

		err = suite.state.TeardownAndDestroy(deleteCtx, res.Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-ch:
			switch event.Type {
			case state.Bootstrapped, state.Noop:
			case state.Errored:
				return event.Error
			case state.Destroyed:
				status := event.Resource.(*infra.MachineRequestStatus) //nolint:errcheck,forcetypeassert

				if err = deleteLabels(status.TypedSpec().Value.Id); err != nil {
					return err
				}
			case state.Created, state.Updated:
				status := event.Resource.(*infra.MachineRequestStatus) //nolint:errcheck,forcetypeassert

				res := system.NewResourceLabels[*omni.MachineStatus](status.TypedSpec().Value.Id)

				err = safe.StateModify(ctx, suite.state, res, func(r *system.ResourceLabels[*omni.MachineStatus]) error {
					if r.Metadata().Phase() == resource.PhaseTearingDown {
						return nil
					}

					res.Metadata().Labels().Set(omni.LabelMachineRequest, status.Metadata().ID())

					helpers.CopyAllLabels(status, r)

					return nil
				})
				if err != nil {
					return err
				}
			}
		}
	}
}

func TestMachineRequestSetStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(MachineRequestSetStatusSuite))
}
