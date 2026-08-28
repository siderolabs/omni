// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infra_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.yaml.in/yaml/v4"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/omni/client/api/omni/management"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/infra"
	"github.com/siderolabs/omni/client/pkg/infra/provision"
	"github.com/siderolabs/omni/client/pkg/jointoken"
	infrares "github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	resourceregistry "github.com/siderolabs/omni/client/pkg/omni/resources/registry"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
)

// The image factory these tests configure their fake Omni with. Deliberately not the public one, so a
// provider that falls back to the default factory URL shows up as a failure.
const (
	testFactoryURL      = "https://factory.example.org"
	testFactoryPXEURL   = "https://pxe.factory.example.org"
	testFactoryUsername = "user"
	testFactoryPassword = "pass"
)

// testMediaSpec is the medium the steps below ask for when only the schematic behind it matters.
var testMediaSpec = provision.MediaSpec{
	MediaSpec: imagefactory.MediaSpec{
		Kind:         imagefactory.InstallationMediaKindDisk,
		Platform:     "nocloud",
		Architecture: "amd64",
		Format:       "raw.xz",
	},
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

//nolint:gocyclo,cyclop,gocognit
func validateConnectionParams(_ context.Context, _ *zap.Logger, pctx provision.Context[*TestResource]) error {
	parts := pctx.ConnectionParams.KernelArgs
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

	token := url.Query().Get(siderolink.JoinTokenQueryParam)
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

	if pctx.ConnectionParams.CustomDataEncoded {
		value, ok = t.ExtraData[omni.LabelMachineRequest]
		if !ok {
			return errors.New("invalid connection params: missing machine ID in the extra data")
		}

		if value != pctx.GetRequestID() {
			return fmt.Errorf("expected machine request id %s got %s", providerID, value)
		}
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

// genSchematic pins the schematic the machine request generates, which EnsureInstallationMedia reports back as
// the ID of the schematic it ensured.
func genSchematic(ctx context.Context, logger *zap.Logger, pctx provision.Context[*TestResource]) error {
	if pctx.ConnectionParams.CustomDataEncoded {
		_, err := pctx.EnsureInstallationMedia(ctx, logger, testMediaSpec)
		if err == nil {
			return errors.New("generating schematics with the connection params must be not allowed")
		}
	} else {
		media, err := pctx.EnsureInstallationMedia(ctx, logger, testMediaSpec)
		if err != nil {
			return err
		}

		expectedSchematic := "4e2a2ec4368100c1b21d4fa7be47f3d38ddab9185f34fc187f82400b1e20da17"

		if media.SchematicID != expectedSchematic {
			return fmt.Errorf("expected schematic id to be %s got %s", expectedSchematic, media.SchematicID)
		}
	}

	media, err := pctx.EnsureInstallationMedia(ctx, logger, testMediaSpec, provision.WithoutConnectionParams())
	if err != nil {
		return err
	}

	expectedSchematic := "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

	if media.SchematicID != expectedSchematic {
		return fmt.Errorf("expected schematic id to be %s got %s", expectedSchematic, media.SchematicID)
	}

	embedded, err := pctx.EnsureInstallationMedia(
		ctx, logger, testMediaSpec,
		provision.WithoutConnectionParams(),
		provision.WithEmbeddedMachineConfig("version: v1alpha1\nmachine: {}\n"),
	)
	if err != nil {
		return err
	}

	if embedded.SchematicID == expectedSchematic {
		return errors.New("expected the embedded machine config to change the schematic id")
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
		provision.NewStep("patches", func(ctx context.Context, _ *zap.Logger, pctx provision.Context[*TestResource]) error {
			return pctx.CreateConfigPatch(ctx, pctx.GetRequestID(), []byte("machine: {}"))
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
	for _, tt := range []struct {
		name    string
		options []infra.Option
	}{
		{
			name: "no options",
		},
		{
			name:    "encode request IDs",
			options: []infra.Option{infra.WithEncodeRequestIDsIntoTokens()},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)

			t.Cleanup(cancel)

			provisionChannel := make(chan struct{}, 1)

			p := &provisioner{
				ch: provisionChannel,
			}

			st := setupInfra(ctx, t, p, tt.options...)

			createSiderolinkConfigs(ctx, t, st)

			customLabel := "custom"
			customValue := "hello"

			machineRequest := infrares.NewMachineRequest("test1")
			machineRequest.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)
			machineRequest.Metadata().Labels().Set(customLabel, customValue)

			patchID := machineRequest.Metadata().ID()

			require.NoError(t, st.Create(ctx, machineRequest))

			rtestutils.AssertResources(ctx, t, st, []string{machineRequest.Metadata().ID()}, func(machineRequestStatus *infrares.MachineRequestStatus, assert *assert.Assertions) {
				val, ok := machineRequestStatus.Metadata().Labels().Get(omni.LabelInfraProviderID)

				assert.True(ok)
				assert.Equal(providerID, val)

				val, ok = machineRequestStatus.Metadata().Labels().Get(customLabel)
				assert.True(ok)
				assert.Equal(customValue, val)

				assert.Equal(specs.MachineRequestStatusSpec_PROVISIONING, machineRequestStatus.TypedSpec().Value.Stage)
			})

			require.True(t, channel.SendWithContext(ctx, provisionChannel, struct{}{}))

			rtestutils.AssertResources(ctx, t, st, []string{machineRequest.Metadata().ID()}, func(machineRequestStatus *infrares.MachineRequestStatus, assert *assert.Assertions) {
				assert.Equal(specs.MachineRequestStatusSpec_PROVISIONED, machineRequestStatus.TypedSpec().Value.Stage)
			})

			rtestutils.AssertResources(ctx, t, st, []string{patchID}, func(r *infrares.ConfigPatchRequest, assert *assert.Assertions) {
				data, err := r.TypedSpec().Value.GetUncompressedData()

				assert.NoError(err)
				assert.EqualValues([]byte("machine: {}"), data.Data())
			})

			rtestutils.AssertResources(ctx, t, st, []string{machineRequest.Metadata().ID()}, func(testResource *TestResource, assert *assert.Assertions) {
				assert.True(testResource.TypedSpec().Value.Connected)
			})

			require.NotNil(t, p.getMachine(machineRequest.Metadata().ID()))

			rtestutils.Destroy[*infrares.MachineRequest](ctx, t, st, []string{machineRequest.Metadata().ID()})

			rtestutils.AssertNoResource[*infrares.MachineRequestStatus](ctx, t, st, machineRequest.Metadata().ID())
			rtestutils.AssertNoResource[*TestResource](ctx, t, st, machineRequest.Metadata().ID())

			require.Nil(t, p.getMachine(machineRequest.Metadata().ID()))

			rtestutils.AssertNoResource[*infrares.ConfigPatchRequest](ctx, t, st, patchID)
		})
	}
}

func setupInfra(ctx context.Context, t *testing.T, p provision.Provisioner[*TestResource], opts ...infra.Option) state.State {
	state := state.WrapCore(namespaced.NewState(inmem.Build))

	resourceRegistry := registry.NewResourceRegistry(state)

	require.NoError(t, resourceRegistry.RegisterDefault(ctx))

	// register Omni resources
	for _, r := range resourceregistry.Resources {
		require.NoError(t, resourceRegistry.Register(ctx, r))
	}

	logger := zaptest.NewLogger(t)

	features := omni.NewFeaturesConfig(omni.FeaturesConfigID)
	features.TypedSpec().Value.ImageFactoryBaseUrl = testFactoryURL
	features.TypedSpec().Value.ImageFactoryPxeBaseUrl = testFactoryPXEURL
	features.TypedSpec().Value.IsEnterpriseImageFactory = true

	require.NoError(t, state.Create(ctx, features))

	factoryAuth := omni.NewImageFactoryAuth(testFactoryURL)
	factoryAuth.TypedSpec().Value.Username = testFactoryUsername
	factoryAuth.TypedSpec().Value.Password = testFactoryPassword

	require.NoError(t, state.Create(ctx, factoryAuth))

	pc := infra.ProviderConfig{
		Name:        "Test Provider",
		Description: "This is the test provider",
		Icon:        "some svg here",
		Schema:      "hello",
	}

	provider, err := infra.NewProvider(providerID, p, pc)
	require.NoError(t, err)

	eg, ctx := errgroup.WithContext(ctx)

	// Stands in for Omni: ensuring the schematic is local to the test, and the medium is built from the
	// image factory configuration in the state, exactly as an older server's fallback would.
	opts = append(opts, infra.WithState(state), infra.WithInstallationMediaResolver(
		func(ctx context.Context, talosVersion string, schematic schematic.Schematic, spec provision.MediaSpec) (imagefactory.InstallationMedia, error) {
			schematicID, err := schematic.ID()
			if err != nil {
				return imagefactory.InstallationMedia{}, err
			}

			return imagefactory.ResolveInstallationMedia(ctx, state, talosVersion, imagefactory.MediaSpec{
				Kind:         spec.Kind,
				Platform:     spec.Platform,
				Architecture: spec.Architecture,
				Format:       spec.Format,
				SecureBoot:   spec.SecureBoot,
			}, schematicID, spec.StandaloneURL)
		},
	))

	eg.Go(func() error {
		return provider.Run(ctx, logger, opts...)
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

func createSiderolinkConfigs(ctx context.Context, t *testing.T, st state.State) {
	providerJoinConfig := siderolink.NewProviderJoinConfig(providerID)
	providerJoinConfig.TypedSpec().Value.JoinToken = "abcd"
	providerJoinConfig.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

	require.NoError(t, st.Create(ctx, providerJoinConfig))

	siderolinkAPIConfig := siderolink.NewAPIConfig()
	siderolinkAPIConfig.TypedSpec().Value.MachineApiAdvertisedUrl = "http://127.0.0.1:8099"
	siderolinkAPIConfig.TypedSpec().Value.LogsPort = 8092
	siderolinkAPIConfig.TypedSpec().Value.EventsPort = 8091

	require.NoError(t, st.Create(ctx, siderolinkAPIConfig))
}

// stepProvisioner is a Provisioner whose ProvisionSteps are configurable per test.
type stepProvisioner struct {
	steps []provision.Step[*TestResource]
}

func (p *stepProvisioner) ProvisionSteps() []provision.Step[*TestResource] {
	return p.steps
}

func (p *stepProvisioner) Deprovision(context.Context, *zap.Logger, *TestResource, *infrares.MachineRequest) error {
	return nil
}

func TestProvisionStepImageFactory(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	type installationMedia struct {
		disk imagefactory.InstallationMedia
		pxe  imagefactory.InstallationMedia
	}

	resolved := make(chan installationMedia, 1)

	p := &stepProvisioner{
		steps: []provision.Step[*TestResource]{
			provision.NewStep("resolveFactory", func(ctx context.Context, logger *zap.Logger, pctx provision.Context[*TestResource]) error {
				disk, err := pctx.EnsureInstallationMedia(ctx, logger, provision.MediaSpec{
					MediaSpec: imagefactory.MediaSpec{
						Kind:         imagefactory.InstallationMediaKindDisk,
						Platform:     "nocloud",
						Architecture: "amd64",
						Format:       "raw.xz",
					},
				})
				if err != nil {
					return err
				}

				pxe, err := pctx.EnsureInstallationMedia(ctx, logger, provision.MediaSpec{
					MediaSpec: imagefactory.MediaSpec{
						Kind:         imagefactory.InstallationMediaKindPXE,
						Platform:     "metal",
						Architecture: "amd64",
					},
				})
				if err != nil {
					return err
				}

				// The controller resumes at the recorded step, so this can run more than once for the same
				// request. Only the first resolution is asserted, and a re-run must not block on it.
				select {
				case resolved <- installationMedia{disk: disk, pxe: pxe}:
				default:
				}

				return nil
			}),
		},
	}

	st := setupInfra(ctx, t, p)
	createSiderolinkConfigs(ctx, t, st)

	machineRequest := infrares.NewMachineRequest("image-factory-test")
	machineRequest.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)
	machineRequest.TypedSpec().Value.TalosVersion = "v1.13.0"

	require.NoError(t, st.Create(ctx, machineRequest))

	rtestutils.AssertResources(ctx, t, st, []string{machineRequest.Metadata().ID()}, func(mrs *infrares.MachineRequestStatus, assert *assert.Assertions) {
		assert.Equal(specs.MachineRequestStatusSpec_PROVISIONED, mrs.TypedSpec().Value.Stage)
	})

	var media installationMedia

	select {
	case media = <-resolved:
	case <-ctx.Done():
		require.Fail(t, "the provision step did not report the installation media")
	}

	// The mock factory client returns the schematic's own ID, so the URL carries whatever the step's
	// machine request generated.
	require.Contains(t, media.disk.URL, "https://factory.example.org/image/")
	require.Contains(t, media.disk.URL, "/v1.13.0/nocloud-amd64.raw.xz")

	// A disk image is fetched by the provider itself, so the credentials come back as a header and the
	// URL stays clean.
	require.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte(testFactoryUsername+":"+testFactoryPassword)),
		media.disk.Headers.Get("Authorization"))
	require.NotContains(t, media.disk.URL, testFactoryPassword)

	// PXE firmware cannot send headers, so those credentials have to ride in the URL instead.
	require.Contains(t, media.pxe.URL,
		"https://"+testFactoryUsername+":"+testFactoryPassword+"@pxe.factory.example.org/pxe/")
	require.Contains(t, media.pxe.URL, "/v1.13.0/metal-amd64")
	require.Empty(t, media.pxe.Headers)
}

// TestProvisionContextWithoutResolver pins the error a hand-built Context reports, since provider step
// tests construct one without a way to reach Omni.
func TestProvisionContextWithoutResolver(t *testing.T) {
	t.Parallel()

	machineRequest := infrares.NewMachineRequest("no-state-test")
	machineRequest.TypedSpec().Value.TalosVersion = "v1.13.0"

	pctx := provision.NewContext(
		machineRequest,
		infrares.NewMachineRequestStatus("no-state-test"),
		NewTestResource("test-namespace", "r"),
		provision.ConnectionParams{},
		nil,
		nil,
	)

	_, err := pctx.EnsureInstallationMedia(t.Context(), zaptest.NewLogger(t), provision.MediaSpec{
		MediaSpec: imagefactory.MediaSpec{
			Kind:         imagefactory.InstallationMediaKindDisk,
			Platform:     "nocloud",
			Architecture: "amd64",
			Format:       "raw.xz",
		},
	})
	require.ErrorContains(t, err, "provision context has no installation media resolver")
}

func TestProvisionStepFailurePersistsError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	p := &stepProvisioner{
		steps: []provision.Step[*TestResource]{
			provision.NewStep("fail", func(context.Context, *zap.Logger, provision.Context[*TestResource]) error {
				return errors.New("permanent failure")
			}),
		},
	}

	st := setupInfra(ctx, t, p)
	createSiderolinkConfigs(ctx, t, st)

	machineRequest := infrares.NewMachineRequest("fail-test")
	machineRequest.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

	require.NoError(t, st.Create(ctx, machineRequest))

	rtestutils.AssertResources(ctx, t, st, []string{machineRequest.Metadata().ID()}, func(mrs *infrares.MachineRequestStatus, assert *assert.Assertions) {
		assert.Equal(specs.MachineRequestStatusSpec_FAILED, mrs.TypedSpec().Value.Stage)
		assert.Equal("permanent failure", mrs.TypedSpec().Value.Error)
	})
}

func TestProvisionStepRequeuePersistsError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	var allowSuccess atomic.Bool

	p := &stepProvisioner{
		steps: []provision.Step[*TestResource]{
			provision.NewStep("retry-then-succeed", func(context.Context, *zap.Logger, provision.Context[*TestResource]) error {
				if !allowSuccess.Load() {
					return provision.NewRetryErrorf(500*time.Millisecond, "transient failure")
				}

				return nil
			}),
		},
	}

	st := setupInfra(ctx, t, p)
	createSiderolinkConfigs(ctx, t, st)

	machineRequest := infrares.NewMachineRequest("requeue-test")
	machineRequest.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

	require.NoError(t, st.Create(ctx, machineRequest))

	rtestutils.AssertResources(ctx, t, st, []string{machineRequest.Metadata().ID()}, func(mrs *infrares.MachineRequestStatus, assert *assert.Assertions) {
		assert.Equal(specs.MachineRequestStatusSpec_PROVISIONING, mrs.TypedSpec().Value.Stage)
		assert.Equal("transient failure", mrs.TypedSpec().Value.Error)
	})

	allowSuccess.Store(true)

	rtestutils.AssertResources(ctx, t, st, []string{machineRequest.Metadata().ID()}, func(mrs *infrares.MachineRequestStatus, assert *assert.Assertions) {
		assert.Equal(specs.MachineRequestStatusSpec_PROVISIONED, mrs.TypedSpec().Value.Stage)
		assert.Empty(mrs.TypedSpec().Value.Error)
	})
}

func TestProvisionStepMutationsRestricted(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	const (
		allowedUUID    = "good-uuid"
		allowedInfraID = "good-infra-id"
		forbiddenLabel = "evil-label"
	)

	block := make(chan struct{})

	t.Cleanup(func() { close(block) })

	p := &stepProvisioner{
		steps: []provision.Step[*TestResource]{
			provision.NewStep("mutate", func(_ context.Context, _ *zap.Logger, pctx provision.Context[*TestResource]) error {
				pctx.SetMachineUUID(allowedUUID)
				pctx.SetMachineInfraID(allowedInfraID)

				// Direct mutations beyond the two helper methods must NOT propagate.
				pctx.MachineRequestStatus.TypedSpec().Value.Status = "evil status"
				pctx.MachineRequestStatus.TypedSpec().Value.Stage = specs.MachineRequestStatusSpec_FAILED
				pctx.MachineRequestStatus.Metadata().Labels().Set(forbiddenLabel, "yes")

				return nil
			}),
			provision.NewStep("block", func(ctx context.Context, _ *zap.Logger, _ provision.Context[*TestResource]) error {
				select {
				case <-block:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}),
		},
	}

	st := setupInfra(ctx, t, p)
	createSiderolinkConfigs(ctx, t, st)

	machineRequest := infrares.NewMachineRequest("mutate-test")
	machineRequest.Metadata().Labels().Set(omni.LabelInfraProviderID, providerID)

	require.NoError(t, st.Create(ctx, machineRequest))

	rtestutils.AssertResources(ctx, t, st, []string{machineRequest.Metadata().ID()}, func(mrs *infrares.MachineRequestStatus, assert *assert.Assertions) {
		assert.Equal(allowedUUID, mrs.TypedSpec().Value.Id)

		infraID, ok := mrs.Metadata().Labels().Get(omni.LabelMachineInfraID)
		assert.True(ok)
		assert.Equal(allowedInfraID, infraID)

		assert.NotEqual("evil status", mrs.TypedSpec().Value.Status)
		assert.Equal(specs.MachineRequestStatusSpec_PROVISIONING, mrs.TypedSpec().Value.Stage)

		_, hasForbidden := mrs.Metadata().Labels().Get(forbiddenLabel)
		assert.False(hasForbidden)
	})
}

// TestInstallationMediaWireConversion covers the two conversions between a provision step's spec and the
// management API, since a provider only ever gets what these two carry across.
func TestInstallationMediaWireConversion(t *testing.T) {
	t.Parallel()

	const schematicID = "376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba"

	diskSpec := func() provision.MediaSpec {
		return provision.MediaSpec{
			MediaSpec: imagefactory.MediaSpec{
				Kind:         imagefactory.InstallationMediaKindDisk,
				Platform:     "nocloud",
				Architecture: "amd64",
				Format:       "raw.xz",
			},
		}
	}

	t.Run("the request carries the lifetime the step asked for", func(t *testing.T) {
		t.Parallel()

		spec := diskSpec()
		spec.DownloadTokenTTL = 90 * time.Minute

		request, err := infra.InstallationMediaRequest("v1.13.0", schematicID, spec)
		require.NoError(t, err)

		require.Equal(t, 90*time.Minute, request.DownloadTokenTtl.AsDuration())
		require.Equal(t, management.InstallationMediaURLRequest_INSTALLATION_MEDIA_KIND_DISK, request.InstallationMediaKind)
		require.Equal(t, schematicID, request.SchematicId)
	})

	t.Run("a step that asks for no lifetime sends none", func(t *testing.T) {
		t.Parallel()

		request, err := infra.InstallationMediaRequest("v1.13.0", schematicID, diskSpec())
		require.NoError(t, err)

		// Unset rather than zero, so Omni can tell "no preference" from a lifetime and apply its default.
		require.Nil(t, request.DownloadTokenTtl)
	})

	t.Run("an unknown kind is refused", func(t *testing.T) {
		t.Parallel()

		_, err := infra.InstallationMediaRequest("v1.13.0", schematicID, provision.MediaSpec{})
		require.Error(t, err)
	})

	t.Run("the response carries the expiry back", func(t *testing.T) {
		t.Parallel()

		expiresAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)

		media := infra.InstallationMediaFromResponse(&management.InstallationMediaURLResponse{
			Url:              "https://factory.example.org/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz?token=abcd",
			ImageFactoryHost: "factory.example.org",
			StorageKey:       "key",
			ExpiresAt:        timestamppb.New(expiresAt),
		})

		require.Equal(t, expiresAt, media.ExpiresAt)
		require.Empty(t, media.Headers)
	})

	t.Run("a response without an expiry leaves it zero", func(t *testing.T) {
		t.Parallel()

		// An Omni that predates the field, or a factory authenticating with credentials, sends nothing.
		// Reading that as the Unix epoch would look like a URL that expired in 1970.
		media := infra.InstallationMediaFromResponse(&management.InstallationMediaURLResponse{
			Url:     "https://factory.example.org/image/" + schematicID + "/v1.13.0/nocloud-amd64.raw.xz",
			Headers: map[string]string{"Authorization": "Basic dXNlcjpwYXNz"},
		})

		require.Zero(t, media.ExpiresAt)
		require.Equal(t, "Basic dXNlcjpwYXNz", media.Headers.Get("Authorization"))
	})
}
