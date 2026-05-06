// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	machine "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	siderolinkres "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	"github.com/siderolabs/omni/client/pkg/siderolink"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/imagefactoryauth"
	omnicfg "github.com/siderolabs/omni/internal/pkg/config"
)

// MaintenanceClientFactory creates a new MaintenanceClient.
type MaintenanceClientFactory = func(ctx context.Context, managementAddress string) (MaintenanceClient, error)

// MaintenanceClient is a client for interacting with Talos running in maintenance mode.
type MaintenanceClient interface {
	GetMachineConfig(ctx context.Context) (*configres.MachineConfig, error)
	ApplyConfiguration(ctx context.Context, req *machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error)
}

type maintenanceClient struct {
	talosClient *client.Client
}

func (c *maintenanceClient) GetMachineConfig(ctx context.Context) (*configres.MachineConfig, error) {
	machineConfig, err := safe.ReaderGetByID[*configres.MachineConfig](ctx, c.talosClient.COSI, configres.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("error getting machine config: %w", err)
	}

	return machineConfig, nil
}

func (c *maintenanceClient) ApplyConfiguration(ctx context.Context, req *machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
	return c.talosClient.ApplyConfiguration(ctx, req)
}

// MaintenanceConfigStatusController manages MaintenanceConfigStatus resource lifecycle.
//
// MaintenanceConfigStatusController generates cluster UUID for every cluster.
type MaintenanceConfigStatusController = qtransform.QController[*siderolinkres.Link, *omni.MaintenanceConfigStatus]

// NewMaintenanceConfigStatusController initializes MaintenanceConfigStatusController.
func NewMaintenanceConfigStatusController(maintenanceClientFactory MaintenanceClientFactory, eventSinkPort, logServerPort int, registries omnicfg.Registries) *MaintenanceConfigStatusController {
	helper := newMaintenanceConfigStatusControllerHelper(maintenanceClientFactory, eventSinkPort, logServerPort, registries)

	return qtransform.NewQController(
		qtransform.Settings[*siderolinkres.Link, *omni.MaintenanceConfigStatus]{
			Name: "MaintenanceConfigStatusController",
			MapMetadataFunc: func(link *siderolinkres.Link) *omni.MaintenanceConfigStatus {
				return omni.NewMaintenanceConfigStatus(link.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *omni.MaintenanceConfigStatus) *siderolinkres.Link {
				return siderolinkres.NewLink(status.Metadata().ID(), nil)
			},
			TransformFunc: helper.transform,
		},
		qtransform.WithExtraMappedInput[*omni.MachineStatus](
			qtransform.MapperSameID[*siderolinkres.Link](),
		),
		qtransform.WithConcurrency(32),
	)
}

type maintenanceConfigStatusControllerHelper struct {
	getMaintenanceConfigPatch func(talosVersion string) (configpatcher.Patch, error)
	maintenanceClientFactory  MaintenanceClientFactory
}

func newMaintenanceConfigStatusControllerHelper(maintenanceClientFactory MaintenanceClientFactory, eventSinkPort, logServerPort int, registries omnicfg.Registries,
) *maintenanceConfigStatusControllerHelper {
	if maintenanceClientFactory == nil {
		maintenanceClientFactory = func(ctx context.Context, managementAddress string) (MaintenanceClient, error) {
			talosClient, err := client.New(ctx, client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}), client.WithEndpoints(managementAddress))
			if err != nil {
				return nil, fmt.Errorf("error creating maintenance client: %w", err)
			}

			return &maintenanceClient{
				talosClient: talosClient,
			}, nil
		}
	}

	buildPatch := func(extraDocs ...talosconfig.Document) (configpatcher.Patch, error) {
		cfg, err := siderolink.NewJoinOptions(
			siderolink.WithoutMachineAPIURL(),
			siderolink.WithEventSinkPort(eventSinkPort),
			siderolink.WithLogServerPort(logServerPort),
		)
		if err != nil {
			return nil, err
		}

		configBytes, err := cfg.RenderJoinConfig(extraDocs...)
		if err != nil {
			return nil, err
		}

		patch, err := configpatcher.LoadPatch(configBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to load patch: %w", err)
		}

		return patch, nil
	}

	basePatch := sync.OnceValues(func() (configpatcher.Patch, error) {
		return buildPatch()
	})

	patchWithRegistryAuth := sync.OnceValues(func() (configpatcher.Patch, error) {
		authDoc, err := imagefactoryauth.BuildDoc(registries)
		if err != nil {
			return nil, fmt.Errorf("error building registry auth doc: %w", err)
		}

		if authDoc == nil {
			return basePatch()
		}

		return buildPatch(authDoc)
	})

	return &maintenanceConfigStatusControllerHelper{
		maintenanceClientFactory: maintenanceClientFactory,
		getMaintenanceConfigPatch: func(talosVersion string) (configpatcher.Patch, error) {
			vc, err := config.ParseContractFromVersion(talosVersion)
			if err != nil {
				return nil, fmt.Errorf("failed to parse contract from version: %w", err)
			}

			if vc.MultidocNetworkConfigSupported() {
				return patchWithRegistryAuth()
			}

			return basePatch()
		},
	}
}

func (helper *maintenanceConfigStatusControllerHelper) transform(ctx context.Context, r controller.Reader, logger *zap.Logger, link *siderolinkres.Link, status *omni.MaintenanceConfigStatus) error {
	if link.TypedSpec().Value.NodePublicKey == status.TypedSpec().Value.PublicKeyAtLastApply {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("public key has not changed (not rebooted/reconnected), skip")
	}

	if !link.TypedSpec().Value.Connected {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is not connected")
	}

	machineStatus, err := safe.ReaderGetByID[*omni.MachineStatus](ctx, r, link.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return xerrors.NewTagged[qtransform.SkipReconcileTag](err)
		}

		return err
	}

	if !machineStatus.TypedSpec().Value.Maintenance {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is not in maintenance mode")
	}

	if machineStatus.TypedSpec().Value.PowerState == specs.MachineStatusSpec_POWER_STATE_OFF {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is powered off, skip maintenance config update")
	}

	if machineStatus.TypedSpec().Value.Schematic.GetInAgentMode() {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine is in agent mode, cannot apply config, skip")
	}

	talosVersion := machineStatus.TypedSpec().Value.TalosVersion
	if talosVersion == "" {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine has no talos version yet")
	}

	if !quirks.New(talosVersion).SupportsMultidoc() {
		return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("talos version does not support multidoc, nothing to do")
	}

	maintenanceTalosClient, err := helper.maintenanceClientFactory(ctx, machineStatus.TypedSpec().Value.ManagementAddress)
	if err != nil {
		return fmt.Errorf("error creating maintenance client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	maintenanceConfig, err := maintenanceTalosClient.GetMachineConfig(ctx)
	if err != nil {
		return fmt.Errorf("error getting maintenance config: %w", err)
	}

	maintenanceConfigPatch, err := helper.getMaintenanceConfigPatch(talosVersion)
	if err != nil {
		return fmt.Errorf("error building machine config: %w", err)
	}

	var machineConfig config.Provider

	if maintenanceConfig != nil {
		machineConfig = maintenanceConfig.Provider()

		logger.Info("loaded existing maintenance config")
	} else {
		if machineConfig, err = container.New(); err != nil {
			return fmt.Errorf("error creating new config container: %w", err)
		}

		logger.Info("created new maintenance config")
	}

	patched, err := configpatcher.Apply(configpatcher.WithConfig(machineConfig), []configpatcher.Patch{maintenanceConfigPatch})
	if err != nil {
		return fmt.Errorf("error applying patch: %w", err)
	}

	patchedBytes, err := patched.Bytes()
	if err != nil {
		return fmt.Errorf("error encoding patched config: %w", err)
	}

	if _, err = maintenanceTalosClient.ApplyConfiguration(ctx, &machine.ApplyConfigurationRequest{
		Data: patchedBytes,
		Mode: machine.ApplyConfigurationRequest_AUTO,
	}); err != nil {
		if grpcstatus.Code(err) == codes.Unimplemented {
			return xerrors.NewTaggedf[qtransform.SkipReconcileTag]("machine does not support applying configuration: %w", err)
		}

		return fmt.Errorf("error applying maintenance config: %w", err)
	}

	logger.Info("applied maintenance config")

	status.TypedSpec().Value.PublicKeyAtLastApply = link.TypedSpec().Value.NodePublicKey

	return nil
}
