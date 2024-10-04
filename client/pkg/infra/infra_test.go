// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
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
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/infra"
	"github.com/siderolabs/omni/client/pkg/infra/provision"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	infrares "github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

type imageFactoryClientMock struct{}

func (i *imageFactoryClientMock) EnsureSchematic(_ context.Context, schematic schematic.Schematic) (string, error) {
	return schematic.ID()
}

type ms struct {
	uuid string
	id   string
}

type provisioner struct {
	ch         <-chan struct{}
	machines   map[resource.ID]ms
	machinesMu sync.Mutex
}

func validateConnectionParams(_ context.Context, _ *zap.Logger, pctx provision.Context[*TestResource]) error {
	parts := strings.Split(pctx.ConnectionParams.KernelArgs, " ")
	if len(parts) == 0 {
		return errors.New("invalid connection params")
	}

	_, u, ok := strings.Cut(parts[0], "=")
	if !ok {
		return errors.New("invalid connection params")
	}

	url, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("invalid connection params: %w", err)
	}

	token := url.Query().Get("jointoken")
	if token == "" {
		return errors.New("invalid connection params")
	}

	t, err := jointoken.Parse(token)
	if err != nil {
		return fmt.Errorf("invalid connection params: %w", err)
	}

	if t.ExtraData == nil {
		return errors.New("invalid connection params: no extra data")
	}

	value, ok := t.ExtraData[omni.LabelInfraProviderID]
	if !ok {
		return errors.New("invalid connection params: missing infra provider extra data")
	}

	if value != providerID {
		return fmt.Errorf("expected provider id %s got %s", providerID, value)
	}

	if pctx.ConnectionParams.JoinConfig == "" {
		return fmt.Errorf("join config is empty")
	}

	dec := yaml.NewDecoder(bytes.NewBufferString(pctx.ConnectionParams.JoinConfig))

	for {
		var d struct {
			APIVersion     string `yaml:"apiVersion"`
			Kind           string `yaml:"kind"`
			APIURL         string `yaml:"apiUrl"`
			EventsEndpoint string `yaml:"endpoint"`
			LogsURL        string `yaml:"url"`
		}

		if err = dec.Decode(&d); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		switch d.Kind {
		case "SideroLinkConfig":
			au, err := url.Parse(d.APIURL)
			if err != nil {
				return err
			}

			if au.String() != url.String() {
				return fmt.Errorf("join config token, expected %s, got %s", url.String(), au.String())
			}
		case "EventSinkConfig":
			if d.EventsEndpoint != "[fdae:41e4:649b:9303::1]:8091" {
				return fmt.Errorf("event sink config is invalid: %q", d.EventsEndpoint)
			}
		case "KmsgLogConfig":
			if d.LogsURL != "tcp://[fdae:41e4:649b:9303::1]:8092" {
				return fmt.Errorf("logs config is invalid: %q", d.LogsURL)
			}
		}
	}

	return nil
}

func genSchematic(ctx context.Context, logger *zap.Logger, pctx provision.Context[*TestResource]) error {
	schematic, err := pctx.GenerateSchematicID(ctx, logger)
	if err != nil {
		return err
	}

	expectedSchematic := "a35d01089c2122ee67ef6f9a0834f01a405d8d6eb70a99a5979c41eeda504720"

	if schematic != expectedSchematic {
		return fmt.Errorf("expected schematic id to be %s got %s", expectedSchematic, schematic)
	}

	schematic, err = pctx.GenerateSchematicID(ctx, logger, provision.WithoutConnectionParams())
	if err != nil {
		return err
	}

	expectedSchematic = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

	if schematic != expectedSchematic {
		return fmt.Errorf("expected schematic id to be %s got %s", expectedSchematic, schematic)
	}

	return nil
}

// Provision implements provision.Provisioner interface.
func (p *provisioner) ProvisionSteps() []provision.Step[*TestResource] {
	return []provision.Step[*TestResource]{
		provision.NewStep("init", func(context.Context, *zap.Logger, provision.Context[*TestResource]) error {
			p.machinesMu.Lock()
			defer p.machinesMu.Unlock()

			if p.machines == nil {
				p.machines = map[string]ms{}

				return provision.NewRetryErrorf(time.Second, "retry me after 1 second")
			}

			return nil
		}),
		provision.NewStep("schematic", genSchematic),
		provision.NewStep("validate", validateConnectionParams),
		provision.NewStep("provision", func(ctx context.Context, _ *zap.Logger, pctx provision.Context[*TestResource]) error {
			p.machinesMu.Lock()
			defer p.machinesMu.Unlock()

			if pctx.State.TypedSpec().Value.Connected {
				return nil
			}

			m, ok := p.machines[pctx.GetRequestID()]
			if !ok {
				m = ms{
					uuid: uuid.New().String(),
					id:   fmt.Sprintf("machine%d", len(p.machines)),
				}

				p.machines[pctx.GetRequestID()] = m
			}

			pctx.SetMachineUUID(m.uuid)
			pctx.SetMachineInfraID(m.id)

			pctx.State.TypedSpec().Value.Connected = true

			select {
			case <-p.ch:
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		}),
	}
}

// Deprovision implements provision.Provisioner interface.
func (p *provisioner) Deprovision(_ context.Context, _ *zap.Logger, _ *TestResource, request *infrares.MachineRequest) error {
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

	machineRequest := infrares.NewMachineRequest("test1")
	machineRequest.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)
	machineRequest.Metadata().Labels().Set(customLabel, customValue)

	require.NoError(t, state.Create(ctx, machineRequest))

	connectionParams := siderolink.NewConnectionParams(resources.DefaultNamespace, siderolink.ConfigID)
	connectionParams.TypedSpec().Value.JoinToken = "abcd"
	connectionParams.TypedSpec().Value.Args = constants.KernelParamSideroLink + "=http://127.0.0.1:8099?jointoken=abcd"
	connectionParams.TypedSpec().Value.ApiEndpoint = "http://127.0.0.1:8099"
	connectionParams.TypedSpec().Value.LogsPort = 8092
	connectionParams.TypedSpec().Value.EventsPort = 8091

	require.NoError(t, state.Create(ctx, connectionParams))

	rtestutils.AssertResources(ctx, t, state, []string{machineRequest.Metadata().ID()}, func(machineRequestStatus *infrares.MachineRequestStatus, assert *assert.Assertions) {
		val, ok := machineRequestStatus.Metadata().Labels().Get(omni.LabelInfraProviderID)

		assert.True(ok)
		assert.Equal(providerID, val)

		val, ok = machineRequestStatus.Metadata().Labels().Get(customLabel)
		assert.True(ok)
		assert.Equal(customValue, val)

		assert.Equal(specs.MachineRequestStatusSpec_PROVISIONING, machineRequestStatus.TypedSpec().Value.Stage)
	})

	require.True(t, channel.SendWithContext(ctx, provisionChannel, struct{}{}))

	rtestutils.AssertResources(ctx, t, state, []string{machineRequest.Metadata().ID()}, func(machineRequestStatus *infrares.MachineRequestStatus, assert *assert.Assertions) {
		assert.Equal(specs.MachineRequestStatusSpec_PROVISIONED, machineRequestStatus.TypedSpec().Value.Stage)
	})

	rtestutils.AssertResources(ctx, t, state, []string{machineRequest.Metadata().ID()}, func(testResource *TestResource, assert *assert.Assertions) {
		assert.True(testResource.TypedSpec().Value.Connected)
	})

	require.NotNil(t, p.getMachine(machineRequest.Metadata().ID()))

	rtestutils.Destroy[*infrares.MachineRequest](ctx, t, state, []string{machineRequest.Metadata().ID()})

	rtestutils.AssertNoResource[*infrares.MachineRequestStatus](ctx, t, state, machineRequest.Metadata().ID())
	rtestutils.AssertNoResource[*TestResource](ctx, t, state, machineRequest.Metadata().ID())

	require.Nil(t, p.getMachine(machineRequest.Metadata().ID()))
}

func setupInfra(ctx context.Context, t *testing.T, p *provisioner) state.State {
	state := state.WrapCore(namespaced.NewState(inmem.Build))

	logger := zaptest.NewLogger(t)

	pc := infra.ProviderConfig{
		Name:        "Test Provider",
		Description: "This is the test provider",
		Icon:        "some svg here",
		Schema:      "hello",
	}

	provider, err := infra.NewProvider(providerID, p, pc)
	require.NoError(t, err)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return provider.Run(ctx, logger, infra.WithState(state), infra.WithImageFactoryClient(&imageFactoryClientMock{}))
	})

	t.Cleanup(func() {
		require.NoError(t, eg.Wait())
	})

	rtestutils.AssertResources(ctx, t, state, []string{providerID}, func(res *infrares.ProviderStatus, assert *assert.Assertions) {
		assert.Equal(res.TypedSpec().Value.Schema, "hello")
		assert.Equal(res.TypedSpec().Value.Name, pc.Name)
		assert.Equal(res.TypedSpec().Value.Description, pc.Description)
		assert.Equal(res.TypedSpec().Value.Icon, pc.Icon)
	})

	return state
}
