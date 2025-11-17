// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/testutils"
)

type infraProviderCleanupTestHelper struct{}

func (c infraProviderCleanupTestHelper) prepareProvider(t *testing.T, ctx context.Context, st state.State, providerID, userID, controllerName string) {
	provider := infra.NewProvider(providerID)
	provider.Metadata().Finalizers().Add(controllerName)

	providerStatus := infra.NewProviderStatus(providerID)
	providerHealthStatus := infra.NewProviderHealthStatus(providerID)

	user := auth.NewUser(resources.DefaultNamespace, userID)
	user.TypedSpec().Value.Role = "Admin"
	serviceAccount := auth.NewIdentity(resources.DefaultNamespace, providerID+access.InfraProviderServiceAccountNameSuffix)
	serviceAccount.Metadata().Labels().Set(auth.LabelIdentityTypeServiceAccount, "")
	serviceAccount.Metadata().Labels().Set(auth.LabelIdentityUserID, userID)
	serviceAccount.TypedSpec().Value.UserId = userID
	publicKey := auth.NewPublicKey(resources.DefaultNamespace, "test-public-key")
	publicKey.Metadata().Labels().Set(auth.LabelIdentityUserID, userID)

	require.NoError(t, st.Create(ctx, provider))
	require.NoError(t, st.Create(ctx, providerStatus))
	require.NoError(t, st.Create(ctx, providerHealthStatus))
	require.NoError(t, st.Create(ctx, user))
	require.NoError(t, st.Create(ctx, serviceAccount))
	require.NoError(t, st.Create(ctx, publicKey))
}

func (c infraProviderCleanupTestHelper) assertProviderTeardown(t *testing.T, ctx context.Context, st state.State, userID, providerID string) {
	providerMD := infra.NewProvider(providerID).Metadata()
	_, err := st.Teardown(ctx, providerMD)
	require.NoError(t, err)

	rtestutils.AssertNoResource[*infra.ProviderStatus](ctx, t, st, providerID)
	rtestutils.AssertNoResource[*infra.ProviderHealthStatus](ctx, t, st, providerID)
	rtestutils.AssertNoResource[*auth.PublicKey](ctx, t, st, "test-public-key")
	rtestutils.AssertNoResource[*auth.User](ctx, t, st, userID)
	rtestutils.AssertNoResource[*auth.Identity](ctx, t, st, access.InfraProviderServiceAccountPrefix+providerID)
}

func TestStaticProviderCleanup(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	helper := infraProviderCleanupTestHelper{}

	providerID := "static-infra-provider"
	userID := uuid.New().String()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
		st := testContext.State

		controller := omnictrl.NewInfraProviderCleanupController()
		helper.prepareProvider(t, ctx, st, providerID, userID, controller.Name())

		require.NoError(t, testContext.Runtime.RegisterController(controller))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		st := testContext.State
		providerMD := infra.NewProvider(providerID).Metadata()

		link := siderolink.NewLink(resources.DefaultNamespace, "test-machine", nil)
		link.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)
		infraMachineStatus := infra.NewMachineStatus(link.Metadata().ID())
		infraMachineStatus.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

		require.NoError(t, st.Create(ctx, link))
		require.NoError(t, st.Create(ctx, infraMachineStatus))

		helper.assertProviderTeardown(t, ctx, st, userID, providerID)

		rtestutils.AssertResource[*siderolink.Link](ctx, t, st, link.Metadata().ID(), func(r *siderolink.Link, assertion *assert.Assertions) {
			assertion.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
		})
		rtestutils.AssertNoResource[*infra.MachineStatus](ctx, t, st, infraMachineStatus.Metadata().ID())

		rtestutils.AssertResource[*infra.Provider](ctx, t, st, providerID, func(r *infra.Provider, assertion *assert.Assertions) {
			assertion.Empty(r.Metadata().Finalizers())
		})

		require.NoError(t, st.Destroy(ctx, providerMD))

		rtestutils.AssertNoResource[*infra.Provider](ctx, t, st, providerID)
	})
}

func TestProvisioningProviderCleanup(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	helper := infraProviderCleanupTestHelper{}

	providerID := "provisioning-infra-provider"
	userID := uuid.New().String()

	testutils.WithRuntime(ctx, t, testutils.TestOptions{}, func(ctx context.Context, testContext testutils.TestContext) {
		st := testContext.State

		controller := omnictrl.NewInfraProviderCleanupController()
		helper.prepareProvider(t, ctx, st, providerID, userID, controller.Name())

		require.NoError(t, testContext.Runtime.RegisterController(controller))
	}, func(ctx context.Context, testContext testutils.TestContext) {
		st := testContext.State
		providerMD := infra.NewProvider(providerID).Metadata()

		machineRequestStatus := infra.NewMachineRequestStatus("test-machine-request")
		machineRequestStatus.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

		machineRequestSet := omni.NewMachineRequestSet(resources.DefaultNamespace, "test-machine-request-set")
		machineRequestSet.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

		omniOwnedMachineRequestSet := omni.NewMachineRequestSet(resources.DefaultNamespace, "test-machine-request-set-omni-owned")
		omniOwnedMachineRequestSet.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

		require.NoError(t, st.Create(ctx, machineRequestStatus))
		require.NoError(t, st.Create(ctx, machineRequestSet))
		require.NoError(t, st.Create(ctx, omniOwnedMachineRequestSet, state.WithCreateOwner(omnictrl.MachineProvisionControllerName)))

		helper.assertProviderTeardown(t, ctx, st, userID, providerID)

		rtestutils.AssertNoResource[*infra.MachineRequestStatus](ctx, t, st, machineRequestStatus.Metadata().ID())
		rtestutils.AssertNoResource[*omni.MachineRequestSet](ctx, t, st, machineRequestSet.Metadata().ID())
		rtestutils.AssertResource[*omni.MachineRequestSet](ctx, t, st, omniOwnedMachineRequestSet.Metadata().ID(), func(r *omni.MachineRequestSet, assertion *assert.Assertions) {
			assertion.Equal(resource.PhaseRunning, r.Metadata().Phase())
		})

		rtestutils.AssertResource[*infra.Provider](ctx, t, st, providerID, func(r *infra.Provider, assertion *assert.Assertions) {
			assertion.Empty(r.Metadata().Finalizers())
		})

		require.NoError(t, st.Destroy(ctx, providerMD))

		rtestutils.AssertNoResource[*infra.Provider](ctx, t, st, providerID)
	})
}
