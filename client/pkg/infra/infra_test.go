// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/api/omni/specs"
	cloudspecs "github.com/siderolabs/omni/client/api/omni/specs/cloud"
	"github.com/siderolabs/omni/client/pkg/infra"
	"github.com/siderolabs/omni/client/pkg/infra/provision"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/cloud"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

type ms struct {
	uuid string
	id   string
}

type provisioner struct {
	ch         <-chan struct{}
	machines   map[resource.ID]ms
	machinesMu sync.Mutex
}

// Provision implements provision.Provisioner interface.
func (p *provisioner) Provision(ctx context.Context, _ *zap.Logger, state *TestResource, request *cloud.MachineRequest, _ *siderolink.ConnectionParams) (provision.Result, error) {
	p.machinesMu.Lock()
	defer p.machinesMu.Unlock()

	if p.machines == nil {
		p.machines = map[string]ms{}
	}

	if state.TypedSpec().Value == nil {
		state.TypedSpec().Value = &specs.MachineSpec{}
	}

	if state.TypedSpec().Value.Connected {
		m := p.machines[request.Metadata().ID()]

		return provision.Result{
			UUID:      m.uuid,
			MachineID: m.id,
		}, nil
	}

	p.machines[request.Metadata().ID()] = ms{
		uuid: uuid.New().String(),
		id:   fmt.Sprintf("machine%d", len(p.machines)),
	}

	state.TypedSpec().Value.Connected = true

	select {
	case <-p.ch:
	case <-ctx.Done():
		return provision.Result{}, ctx.Err()
	}

	return provision.Result{}, nil
}

// Deprovision implements provision.Provisioner interface.
func (p *provisioner) Deprovision(_ context.Context, _ *zap.Logger, _ *TestResource, request *cloud.MachineRequest) error {
	p.machinesMu.Lock()
	defer p.machinesMu.Unlock()

	_, ok := p.machines[request.Metadata().ID()]
	if !ok {
		return fmt.Errorf("failed to deprovision machine %s: doesn't exist", request.Metadata().ID())
	}

	delete(p.machines, request.Metadata().ID())

	return nil
}

func (p *provisioner) getMachine(id string) *ms {
	p.machinesMu.Lock()
	defer p.machinesMu.Unlock()

	ms, ok := p.machines[id]
	if !ok {
		return nil
	}

	return &ms
}

func TestInfra(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	t.Cleanup(cancel)

	provisionChannel := make(chan struct{}, 1)

	p := &provisioner{
		ch: provisionChannel,
	}

	state := setupInfra(ctx, t, p)

	customLabel := "custom"
	customValue := "hello"

	machineRequest := cloud.NewMachineRequest("test1")
	machineRequest.Metadata().Labels().Set(omni.LabelCloudProviderID, providerID)
	machineRequest.Metadata().Labels().Set(customLabel, customValue)

	require.NoError(t, state.Create(ctx, machineRequest))

	connectionParams := siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)

	require.NoError(t, state.Create(ctx, connectionParams))

	rtestutils.AssertResources(ctx, t, state, []string{machineRequest.Metadata().ID()}, func(machineRequestStatus *cloud.MachineRequestStatus, assert *assert.Assertions) {
		val, ok := machineRequestStatus.Metadata().Labels().Get(omni.LabelCloudProviderID)

		assert.True(ok)
		assert.Equal(providerID, val)

		val, ok = machineRequestStatus.Metadata().Labels().Get(customLabel)
		assert.True(ok)
		assert.Equal(customValue, val)

		assert.Equal(cloudspecs.MachineRequestStatusSpec_PROVISIONING, machineRequestStatus.TypedSpec().Value.Stage)
	})

	require.True(t, channel.SendWithContext(ctx, provisionChannel, struct{}{}))

	rtestutils.AssertResources(ctx, t, state, []string{machineRequest.Metadata().ID()}, func(machineRequestStatus *cloud.MachineRequestStatus, assert *assert.Assertions) {
		assert.Equal(cloudspecs.MachineRequestStatusSpec_PROVISIONED, machineRequestStatus.TypedSpec().Value.Stage)
	})

	rtestutils.AssertResources(ctx, t, state, []string{machineRequest.Metadata().ID()}, func(testResource *TestResource, assert *assert.Assertions) {
		assert.True(testResource.TypedSpec().Value.Connected)
	})

	require.NotNil(t, p.getMachine(machineRequest.Metadata().ID()))

	rtestutils.Destroy[*cloud.MachineRequest](ctx, t, state, []string{machineRequest.Metadata().ID()})

	rtestutils.AssertNoResource[*cloud.MachineRequestStatus](ctx, t, state, machineRequest.Metadata().ID())
	rtestutils.AssertNoResource[*TestResource](ctx, t, state, machineRequest.Metadata().ID())

	require.Nil(t, p.getMachine(machineRequest.Metadata().ID()))
}

func setupInfra(ctx context.Context, t *testing.T, p *provisioner) state.State {
	state := state.WrapCore(namespaced.NewState(inmem.Build))

	logger := zaptest.NewLogger(t)

	provider, err := infra.NewProvider(providerID, p)
	require.NoError(t, err)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return provider.Run(ctx, logger, infra.WithState(state))
	})

	t.Cleanup(func() {
		require.NoError(t, eg.Wait())
	})

	return state
}
