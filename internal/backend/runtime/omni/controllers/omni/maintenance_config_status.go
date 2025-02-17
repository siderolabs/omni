// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
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
	machineConfig, err := safe.ReaderGetByID[*configres.MachineConfig](ctx, c.talosClient.COSI, configres.V1Alpha1ID)
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
type MaintenanceConfigStatusController = qtransform.QController[*siderolink.Link, *omni.MaintenanceConfigStatus]

// NewMaintenanceConfigStatusController initializes MaintenanceConfigStatusController.
func NewMaintenanceConfigStatusController(maintenanceClientFactory MaintenanceClientFactory, siderolinkListenHost string, eventSinkPort, logServerPort int) *MaintenanceConfigStatusController {
	helper := newMaintenanceConfigStatusControllerHelper(maintenanceClientFactory, siderolinkListenHost, eventSinkPort, logServerPort)

	return qtransform.NewQController(
		qtransform.Settings[*siderolink.Link, *omni.MaintenanceConfigStatus]{
			Name: "MaintenanceConfigStatusController",
			MapMetadataFunc: func(link *siderolink.Link) *omni.MaintenanceConfigStatus {
				return omni.NewMaintenanceConfigStatus(link.Metadata().ID())
			},
			UnmapMetadataFunc: func(status *omni.MaintenanceConfigStatus) *siderolink.Link {
				return siderolink.NewLink(resources.DefaultNamespace, status.Metadata().ID(), nil)
			},
			TransformFunc: helper.transform,
		},
		qtransform.WithExtraMappedInput(
			qtransform.MapperSameID[*omni.MachineStatus, *siderolink.Link](),
		),
		qtransform.WithConcurrency(32),
	)
}

type maintenanceConfigStatusControllerHelper struct {
	getMachineConfigPatch    func() (configpatcher.Patch, error)
	maintenanceClientFactory MaintenanceClientFactory
}

func newMaintenanceConfigStatusControllerHelper(maintenanceClientFactory MaintenanceClientFactory,
	siderolinkListenHost string, eventSinkPort, logServerPort int,
) *maintenanceConfigStatusControllerHelper {
	if maintenanceClientFactory == nil {
		maintenanceClientFactory = func(ctx context.Context, managementAddress string) (MaintenanceClient, error) {
			talosClient, err := client.New(ctx, client.WithTLSConfig(insecureTLSConfig), client.WithEndpoints(managementAddress))
			if err != nil {
				return nil, fmt.Errorf("error creating maintenance client: %w", err)
			}

			return &maintenanceClient{
				talosClient: talosClient,
			}, nil
		}
	}

	return &maintenanceConfigStatusControllerHelper{
		maintenanceClientFactory: maintenanceClientFactory,
		getMachineConfigPatch: sync.OnceValues(func() (configpatcher.Patch, error) {
			eventSinkConfig := runtime.NewEventSinkV1Alpha1()
			eventSinkConfig.Endpoint = net.JoinHostPort(siderolinkListenHost, strconv.Itoa(eventSinkPort))

			kmsgLogURL, err := url.Parse("tcp://" + net.JoinHostPort(siderolinkListenHost, strconv.Itoa(logServerPort)))
			if err != nil {
				return nil, fmt.Errorf("failed to parse kmsg log URL: %w", err)
			}

			kmsgLogConfig := runtime.NewKmsgLogV1Alpha1()
			kmsgLogConfig.MetaName = "omni-kmsg"
			kmsgLogConfig.KmsgLogURL = meta.URL{
				URL: kmsgLogURL,
			}

			configContainer, err := container.New(eventSinkConfig, kmsgLogConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to create config container: %w", err)
			}

			configBytes, err := configContainer.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
			if err != nil {
				return nil, fmt.Errorf("failed to encode config container: %w", err)
			}

			patch, err := configpatcher.LoadPatch(configBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to load patch: %w", err)
			}

			return patch, nil
		}),
	}
}

func (helper *maintenanceConfigStatusControllerHelper) transform(ctx context.Context, r controller.Reader, logger *zap.Logger, link *siderolink.Link, status *omni.MaintenanceConfigStatus) error {
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

	maintenanceConfigPatch, err := helper.getMachineConfigPatch()
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
		return fmt.Errorf("error applying maintenance config: %w", err)
	}

	logger.Info("applied maintenance config")

	status.TypedSpec().Value.PublicKeyAtLastApply = link.TypedSpec().Value.NodePublicKey

	return nil
}
