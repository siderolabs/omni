// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	omnimeta "github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/talos"
)

type machinePollFunction func(ctx context.Context, c *client.Client, info *Info) error

var resourcePollers = map[string]machinePollFunction{
	network.HostnameStatusType:   pollHostname,
	network.RouteStatusType:      pollRoutes,
	network.NodeAddressType:      pollAddresses,
	network.LinkStatusType:       pollNetworkLinks,
	hardware.ProcessorType:       pollProcessors,
	hardware.MemoryModuleType:    pollMemory,
	runtime.PlatformMetadataType: pollPlatformMetadata,
	runtime.MetaKeyType:          pollMeta,
	runtime.ExtensionStatusType:  pollExtensions,
	config.MachineConfigType:     pollMaintenanceConfig,
}

var machinePollers = map[string]machinePollFunction{
	"version": pollVersion,
	"disks":   pollDisks,
}

var allPollers = merged(resourcePollers, machinePollers)

func merged[K comparable, V any](m1, m2 map[K]V) map[K]V {
	res := maps.Clone(m1)

	maps.Copy(res, m2)

	return res
}

func poll(ctx context.Context, poller string, c *client.Client, info *Info) error {
	f, ok := allPollers[poller]
	if !ok {
		panic(fmt.Sprintf("unknown poller %q", poller))
	}

	return f(ctx, c, info)
}

func pollVersion(ctx context.Context, c *client.Client, info *Info) error {
	versionResp, err := c.Version(ctx)
	if err != nil && client.StatusCode(err) != codes.Unimplemented {
		return err
	}

	for _, msg := range versionResp.GetMessages() {
		info.TalosVersion = pointer.To(msg.GetVersion().GetTag())
		info.Arch = pointer.To(msg.GetVersion().GetArch())
	}

	return nil
}

func pollHostname(ctx context.Context, c *client.Client, info *Info) error {
	return forEachResource(
		ctx,
		c,
		network.NamespaceName,
		network.HostnameStatusType,
		func(r *network.HostnameStatus) error {
			info.Hostname = pointer.To(r.TypedSpec().Hostname)
			info.Domainname = pointer.To(r.TypedSpec().Domainname)

			return nil
		})
}

func filterAddresses(maintenanceMode bool) func(r *network.NodeAddress) bool {
	if maintenanceMode {
		return func(r *network.NodeAddress) bool {
			return r.Metadata().ID() == network.NodeAddressCurrentID
		}
	}

	return func(r *network.NodeAddress) bool {
		return r.Metadata().ID() == network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s)
	}
}

func pollAddresses(ctx context.Context, c *client.Client, info *Info) error {
	return forEachResource(
		ctx,
		c,
		network.NamespaceName,
		network.NodeAddressType,
		func(r *network.NodeAddress) error {
			if info.MaintenanceMode {
				// in maintenance mode, there is no Kubernetes, and filtered addresses
				if r.Metadata().ID() != network.NodeAddressCurrentID {
					return nil
				}
			} else {
				// in normal mode, use filtered addresses (without Kubernetes)
				if r.Metadata().ID() != network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s) {
					return nil
				}
			}

			info.Addresses = make([]string, 0, len(r.TypedSpec().Addresses))

			for _, addr := range r.TypedSpec().Addresses {
				// skip SideroLink addresses
				if network.IsULA(addr.Addr(), network.ULASideroLink) {
					continue
				}

				info.Addresses = append(info.Addresses, addr.String())
			}

			return nil
		})
}

func filterRoutes(r *network.RouteStatus) bool {
	return value.IsZero(r.TypedSpec().Destination) && r.TypedSpec().Gateway.IsValid() && r.TypedSpec().Scope == nethelpers.ScopeGlobal
}

func pollRoutes(ctx context.Context, c *client.Client, info *Info) error {
	info.DefaultGateways = nil

	return forEachResource(
		ctx,
		c,
		network.NamespaceName,
		network.RouteStatusType,
		func(r *network.RouteStatus) error {
			if value.IsZero(r.TypedSpec().Destination) && r.TypedSpec().Gateway.IsValid() && r.TypedSpec().Scope == nethelpers.ScopeGlobal {
				info.DefaultGateways = append(info.DefaultGateways, r.TypedSpec().Gateway.String())
			}

			return nil
		})
}

func filterNetworkLinks(r *network.LinkStatus) bool {
	return r.TypedSpec().Physical()
}

func pollNetworkLinks(ctx context.Context, c *client.Client, info *Info) error {
	info.NetworkLinks = nil

	return forEachResource(
		ctx,
		c,
		network.NamespaceName,
		network.LinkStatusType,
		func(r *network.LinkStatus) error {
			if !r.TypedSpec().Physical() {
				return nil
			}

			info.NetworkLinks = append(info.NetworkLinks, &specs.MachineStatusSpec_NetworkStatus_NetworkLinkStatus{
				LinuxName:       r.Metadata().ID(),
				HardwareAddress: r.TypedSpec().HardwareAddr.String(),
				SpeedMbps:       uint32(r.TypedSpec().SpeedMegabits),
				LinkUp:          r.TypedSpec().LinkState,
				Description:     fmt.Sprintf("%s %s", r.TypedSpec().Vendor, r.TypedSpec().Product),
			})

			return nil
		})
}

func pollProcessors(ctx context.Context, c *client.Client, info *Info) error {
	info.Processors = nil

	return forEachResource(
		ctx,
		c,
		hardware.NamespaceName,
		hardware.ProcessorType,
		func(r *hardware.Processor) error {
			if r.TypedSpec().CoreCount == 0 || r.TypedSpec().MaxSpeed == 0 {
				return nil
			}

			info.Processors = append(info.Processors, &specs.MachineStatusSpec_HardwareStatus_Processor{
				CoreCount:    r.TypedSpec().CoreCount,
				ThreadCount:  r.TypedSpec().ThreadCount,
				Frequency:    r.TypedSpec().MaxSpeed,
				Manufacturer: r.TypedSpec().Manufacturer,
				Description:  fmt.Sprintf("%s %s", r.TypedSpec().Manufacturer, r.TypedSpec().ProductName),
			})

			return nil
		})
}

func pollMemory(ctx context.Context, c *client.Client, info *Info) error {
	info.MemoryModules = nil

	return forEachResource(
		ctx,
		c,
		hardware.NamespaceName,
		hardware.MemoryModuleType,
		func(r *hardware.MemoryModule) error {
			if r.TypedSpec().Size == 0 {
				return nil
			}

			info.MemoryModules = append(info.MemoryModules, &specs.MachineStatusSpec_HardwareStatus_MemoryModule{
				SizeMb:      r.TypedSpec().Size,
				Description: r.TypedSpec().Manufacturer,
			})

			return nil
		})
}

func pollPlatformMetadata(ctx context.Context, c *client.Client, info *Info) error {
	return forEachResource(
		ctx,
		c,
		runtime.NamespaceName,
		runtime.PlatformMetadataType,
		func(r *runtime.PlatformMetadata) error {
			info.PlatformMetadata = &specs.MachineStatusSpec_PlatformMetadata{
				Platform:     r.TypedSpec().Platform,
				Hostname:     r.TypedSpec().Hostname,
				Region:       r.TypedSpec().Region,
				Zone:         r.TypedSpec().Zone,
				InstanceType: r.TypedSpec().InstanceType,
				InstanceId:   r.TypedSpec().InstanceID,
				ProviderId:   r.TypedSpec().ProviderID,
				Spot:         r.TypedSpec().Spot,
			}

			return nil
		})
}

func pollDisks(ctx context.Context, c *client.Client, info *Info) error {
	info.Blockdevices = nil

	disksResp, err := c.Disks(ctx)
	if err != nil {
		return err
	}

	for _, msg := range disksResp.GetMessages() {
		for _, disk := range msg.GetDisks() {
			info.Blockdevices = append(info.Blockdevices, &specs.MachineStatusSpec_HardwareStatus_BlockDevice{
				Size:       disk.GetSize(),
				Model:      disk.GetModel(),
				LinuxName:  disk.GetDeviceName(),
				Name:       disk.GetName(),
				Serial:     disk.GetSerial(),
				Uuid:       disk.GetUuid(),
				Wwid:       disk.GetWwid(),
				Type:       disk.GetType().String(),
				BusPath:    disk.GetBusPath(),
				SystemDisk: disk.GetSystemDisk(),
			})
		}
	}

	return nil
}

func pollMeta(ctx context.Context, c *client.Client, info *Info) error {
	return forEachResource(
		ctx,
		c,
		runtime.NamespaceName,
		runtime.MetaKeyType,
		func(metaKey *runtime.MetaKey) error {
			if metaKey.Metadata().ID() != runtime.MetaKeyTagToID(omnimeta.LabelsMeta) {
				return nil
			}

			imageLabels, err := omnimeta.ParseLabels([]byte(metaKey.TypedSpec().Value))
			if err != nil {
				return err
			}

			labels := imageLabels.Labels

			// fallback to legacy labels
			if labels == nil {
				labels = imageLabels.LegacyLabels
			}

			// filter out labels which are already defined in the machine labels resource
			if labels != nil && info.MachineLabels != nil {
				for _, k := range info.MachineLabels.Metadata().Labels().Keys() {
					delete(labels, k)
				}
			}

			info.ImageLabels = labels

			return nil
		})
}

func pollExtensions(ctx context.Context, c *client.Client, info *Info) error {
	machineSchematic := &specs.MachineStatusSpec_Schematic{}
	info.Schematic = machineSchematic

	var err error

	schematicInfo, err := talos.GetSchematicInfo(ctx, c)
	if err != nil {
		if errors.Is(err, talos.ErrInvalidSchematic) {
			machineSchematic.Invalid = true

			return nil
		}

		return err
	}

	machineSchematic.Id = schematicInfo.ID
	machineSchematic.Extensions = schematicInfo.Extensions
	machineSchematic.Overlay = &specs.MachineStatusSpec_Schematic_Overlay{
		Name:  schematicInfo.Schematic.Overlay.Name,
		Image: schematicInfo.Schematic.Overlay.Image,
	}

	return nil
}

func pollMaintenanceConfig(ctx context.Context, c *client.Client, info *Info) error {
	if !info.MaintenanceMode {
		return nil
	}

	conf, err := safe.StateGetByID[*config.MachineConfig](ctx, c.COSI, config.V1Alpha1ID)
	if err != nil {
		if state.IsNotFoundError(err) { // if the config is not found (unset), we mark it as ready by setting it to a non-nil value by empty config.
			info.MaintenanceConfig = &specs.MachineStatusSpec_MaintenanceConfig{}

			return nil
		}

		// this resource might not be authorized to be read over siderolink in the maintenance mode on Talos versions < 1.6.5.
		// in that case, if it was never set previously, we mark it as ready by setting it to a non-nil value with empty config.
		if code := status.Code(err); code == codes.PermissionDenied {
			if info.MaintenanceConfig == nil {
				info.MaintenanceConfig = &specs.MachineStatusSpec_MaintenanceConfig{}
			}

			return nil
		}

		return fmt.Errorf("failed to get maintenance config: %w", err)
	}

	for _, document := range conf.Container().Documents() {
		if document.Kind() == v1alpha1.Version {
			return fmt.Errorf("unexpected v1alpha1.Config document in maintenance config: %w", err)
		}
	}

	configBytes, err := conf.Container().Bytes()
	if err != nil {
		return fmt.Errorf("failed to read maintenance config bytes: %w", err)
	}

	info.MaintenanceConfig = &specs.MachineStatusSpec_MaintenanceConfig{
		Config: string(configBytes),
	}

	return nil
}
