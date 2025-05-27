// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/image-factory/pkg/schematic"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	omnimeta "github.com/siderolabs/omni/client/pkg/meta"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/boards"
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
	runtime.DiagnosticType:       pollDiagnostics,
	block.DiskType:               pollDisks,
}

var machinePollers = map[string]machinePollFunction{
	"version": pollVersion,
	"disks":   pollDisksLegacy,

	// we do not use a resource poller for the SecurityState, as we want to mark
	// secure boot / UKI explicitly to disabled (contrary to leaving it nil) if the feature is not available (i.e., older Talos versions).
	//
	// resourcePollers skip polling the resource if it is not defined on the Talos API.
	//
	// furthermore, by doing this, we skip watching this resource, which is what we want, since it does not change over time.
	"securityState": pollSecurityState,
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
	info.Hostname = pointer.To("")
	info.Domainname = pointer.To("")

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

func pollSecurityState(ctx context.Context, c *client.Client, info *Info) error {
	probeSecurityState := func() (isSecureBoot, bootedWithUki bool, err error) {
		if _, err = safe.StateGetByID[*meta.ResourceDefinition](ctx, c.COSI, strings.ToLower(runtime.SecurityStateType)); err != nil {
			if !state.IsNotFoundError(err) {
				return false, false, fmt.Errorf("failed to get security state rd: %w", err)
			}

			return false, false, nil
		}

		securityState, err := safe.StateGetByID[*runtime.SecurityState](ctx, c.COSI, runtime.SecurityStateID)
		if err != nil {
			return false, false, fmt.Errorf("failed to get security state: %w", err)
		}

		isSecureBoot = securityState.TypedSpec().SecureBoot
		bootedWithUki = isSecureBoot || securityState.TypedSpec().BootedWithUKI

		return isSecureBoot, bootedWithUki, nil
	}

	isSecureBoot, bootedWithUki, err := probeSecurityState()
	if err != nil {
		return err
	}

	info.SecurityState = &specs.SecurityState{
		SecureBoot:    isSecureBoot,
		BootedWithUki: bootedWithUki,
	}

	return nil
}

func pollDisks(ctx context.Context, c *client.Client, info *Info) error {
	info.Blockdevices = nil

	systemDisk, err := safe.StateGetByID[*block.SystemDisk](ctx, c.COSI, block.SystemDiskID)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	return forEachResource(
		ctx,
		c,
		block.NamespaceName,
		block.DiskType,
		func(disk *block.Disk) error {
			if strings.HasPrefix(disk.TypedSpec().DevPath, "/dev/loop") {
				return nil
			}

			spec := disk.TypedSpec()

			var diskType storage.Disk_DiskType

			switch {
			case spec.CDROM:
				diskType = storage.Disk_CD
			case spec.Transport == "nvme":
				diskType = storage.Disk_NVME
			case spec.Transport == "mmc":
				diskType = storage.Disk_SD
			case spec.Rotational:
				diskType = storage.Disk_HDD
			case spec.Transport != "":
				diskType = storage.Disk_SSD
			}

			if diskType == storage.Disk_UNKNOWN && spec.Modalias == "" && spec.SubSystem != "/sys/class/block" {
				return nil
			}

			if len(spec.SecondaryDisks) > 0 || spec.BusPath == "/virtual" { // not a real disk, e.g., a device mapper device (lvm)
				return nil
			}

			info.Blockdevices = append(info.Blockdevices, &specs.MachineStatusSpec_HardwareStatus_BlockDevice{
				Size:       spec.Size,
				Model:      spec.Model,
				LinuxName:  filepath.Join("/dev", disk.Metadata().ID()),
				Serial:     spec.Serial,
				Uuid:       spec.UUID,
				Wwid:       spec.WWID,
				Type:       diskType.String(),
				BusPath:    spec.BusPath,
				SystemDisk: systemDisk != nil && disk.Metadata().ID() == systemDisk.TypedSpec().DiskID,
				Readonly:   spec.Readonly,
				Transport:  spec.Transport,
			})

			return nil
		})
}

func pollDisksLegacy(ctx context.Context, c *client.Client, info *Info) error {
	info.Blockdevices = nil

	disksResp, err := c.Disks(ctx)
	if err != nil {
		return err
	}

	for _, msg := range disksResp.GetMessages() {
		for _, disk := range msg.GetDisks() {
			if strings.HasPrefix(disk.GetDeviceName(), "/dev/loop") {
				continue
			}

			if disk.Type == storage.Disk_UNKNOWN && disk.Modalias == "" && disk.Subsystem != "/sys/class/block" {
				continue
			}

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
				Readonly:   disk.GetReadonly(),
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

func detectOverlay(ctx context.Context, c *client.Client) (*schematic.Overlay, error) {
	reader, err := c.Read(ctx, "/proc/cmdline")
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	cmdline := procfs.NewCmdline(string(data))

	value := cmdline.Get(constants.KernelParamBoard)

	if value == nil || value.Get(0) == nil {
		return nil, nil //nolint:nilnil
	}

	return boards.GetOverlay(*value.Get(0)), nil
}

func pollExtensions(ctx context.Context, c *client.Client, info *Info) error {
	var err error

	schematicInfo, err := talos.GetSchematicInfo(ctx, c, info.DefaultKernelArgs)
	if err != nil {
		if errors.Is(err, talos.ErrInvalidSchematic) {
			info.Schematic = &SchematicInfo{
				Invalid: true,
			}

			return nil
		}

		return err
	}

	// In the agent mode, the Read API is not supported, so we can skip the overlay detection.
	if !schematicInfo.InAgentMode && schematicInfo.Overlay.Name == "" {
		overlay, err := detectOverlay(ctx, c)
		if err != nil && status.Code(err) != codes.Unimplemented {
			return err
		}

		if overlay != nil {
			schematicInfo.Overlay = *overlay
		}
	}

	info.Schematic = &SchematicInfo{
		SchematicInfo: schematicInfo,
	}

	return nil
}

func pollDiagnostics(ctx context.Context, c *client.Client, info *Info) error {
	info.Diagnostics = nil

	if err := forEachResource(
		ctx,
		c,
		runtime.NamespaceName,
		runtime.DiagnosticType,
		func(r *runtime.Diagnostic) error {
			info.Diagnostics = append(info.Diagnostics, &specs.MachineStatusSpec_Diagnostic{
				Id:      r.Metadata().ID(),
				Message: r.TypedSpec().Message,
				Details: r.TypedSpec().Details,
			})

			return nil
		}); err != nil {
		return err
	}

	if len(info.Diagnostics) == 0 { // polling was successful, so ensure that MachineStatus gets updated
		info.Diagnostics = []*specs.MachineStatusSpec_Diagnostic{}
	}

	return nil
}
