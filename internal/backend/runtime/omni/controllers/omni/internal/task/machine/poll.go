// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package machine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"google.golang.org/grpc/codes"

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
	runtime.DiagnosticType:       pollDiagnostics,
	runtime.KernelCmdlineType:    pollKernelCmdline,
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
	// the resource is still watched: the FIPS state follows the installed image, so it changes across upgrades.
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
		info.TalosVersion = new(msg.GetVersion().GetTag())
		info.Arch = new(msg.GetVersion().GetArch())
	}

	return pollVersionName(ctx, c, info)
}

// pollVersionName reads the version name (e.g. "Talos Enterprise") from the Version resource.
//
// The name is only updated on a definitive outcome, as a partially populated Info is sent even when the poll fails,
// and a transient error must not wipe the previously known name.
//
// The resource is not defined on older Talos versions, in which case the name is explicitly set to empty,
// so that the information is cleared if the machine was downgraded.
//
// If the resource is defined but not created yet (e.g. early on boot), the name is left untouched,
// as the update on the resource creation triggers another poll.
func pollVersionName(ctx context.Context, c *client.Client, info *Info) error {
	if _, err := safe.StateGetByID[*meta.ResourceDefinition](ctx, c.COSI, strings.ToLower(runtime.VersionType)); err != nil {
		if !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get version rd: %w", err)
		}

		info.TalosVersionName = new("")

		return nil
	}

	version, err := safe.StateGetByID[*runtime.Version](ctx, c.COSI, runtime.NewVersion().Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("failed to get version: %w", err)
	}

	info.TalosVersionName = new(version.TypedSpec().Name)

	return nil
}

func pollHostname(ctx context.Context, c *client.Client, info *Info) error {
	info.Hostname = new("")
	info.Domainname = new("")

	return forEachResource(
		ctx,
		c,
		network.NamespaceName,
		network.HostnameStatusType,
		func(r *network.HostnameStatus) error {
			info.Hostname = new(r.TypedSpec().Hostname)
			info.Domainname = new(r.TypedSpec().Domainname)

			return nil
		},
	)
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
		},
	)
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
		},
	)
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
		},
	)
}

func pollProcessors(ctx context.Context, c *client.Client, info *Info) error {
	info.Processors = nil

	return forEachResource(
		ctx,
		c,
		hardware.NamespaceName,
		hardware.ProcessorType,
		func(r *hardware.Processor) error {
			if r.TypedSpec().CoreCount == 0 && r.TypedSpec().MaxSpeed == 0 {
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
		},
	)
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
		},
	)
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
				Tags:         r.TypedSpec().Tags,
			}

			return nil
		},
	)
}

func pollSecurityState(ctx context.Context, c *client.Client, info *Info) error {
	probeSecurityState := func() (isSecureBoot, bootedWithUki bool, fipsState specs.SecurityState_FIPSState, err error) {
		if _, err = safe.StateGetByID[*meta.ResourceDefinition](ctx, c.COSI, strings.ToLower(runtime.SecurityStateType)); err != nil {
			if !state.IsNotFoundError(err) {
				return false, false, specs.SecurityState_FIPS_STATE_DISABLED, fmt.Errorf("failed to get security state rd: %w", err)
			}

			return false, false, specs.SecurityState_FIPS_STATE_DISABLED, nil
		}

		securityState, err := safe.StateGetByID[*runtime.SecurityState](ctx, c.COSI, runtime.SecurityStateID)
		if err != nil {
			return false, false, specs.SecurityState_FIPS_STATE_DISABLED, fmt.Errorf("failed to get security state: %w", err)
		}

		isSecureBoot = securityState.TypedSpec().SecureBoot
		bootedWithUki = isSecureBoot || securityState.TypedSpec().BootedWithUKI

		switch securityState.TypedSpec().FIPSState {
		case runtime.FIPSStateDisabled:
			fipsState = specs.SecurityState_FIPS_STATE_DISABLED
		case runtime.FIPSStateEnabled:
			fipsState = specs.SecurityState_FIPS_STATE_ENABLED
		case runtime.FIPSStateStrict:
			fipsState = specs.SecurityState_FIPS_STATE_STRICT
		}

		return isSecureBoot, bootedWithUki, fipsState, nil
	}

	isSecureBoot, bootedWithUki, fipsState, err := probeSecurityState()
	if err != nil {
		return err
	}

	info.SecurityState = &specs.SecurityState{
		SecureBoot:    isSecureBoot,
		BootedWithUki: bootedWithUki,
		FipsState:     fipsState,
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
				Wwid:       strings.ToValidUTF8(spec.WWID, ""),
				Type:       diskType.String(),
				BusPath:    spec.BusPath,
				SystemDisk: systemDisk != nil && disk.Metadata().ID() == systemDisk.TypedSpec().DiskID,
				Readonly:   spec.Readonly,
				Transport:  spec.Transport,
			})

			return nil
		},
	)
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
				Wwid:       strings.ToValidUTF8(disk.GetWwid(), ""),
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
		},
	)
}

func pollExtensions(ctx context.Context, c *client.Client, info *Info) error {
	var err error

	schematicInfo, err := talos.GetSchematicInfo(ctx, c.COSI, info.DefaultKernelArgs)
	if err != nil {
		if errors.Is(err, talos.ErrInvalidSchematic) {
			info.Schematic = &SchematicInfo{
				Invalid: true,
			}

			return nil
		}

		return err
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
		},
	); err != nil {
		return err
	}

	if len(info.Diagnostics) == 0 { // polling was successful, so ensure that MachineStatus gets updated
		info.Diagnostics = []*specs.MachineStatusSpec_Diagnostic{}
	}

	return nil
}

func pollKernelCmdline(ctx context.Context, c *client.Client, info *Info) error {
	info.KernelCmdline = ""

	return forEachResource(
		ctx,
		c,
		runtime.NamespaceName,
		runtime.KernelCmdlineType,
		func(r *runtime.KernelCmdline) error {
			info.KernelCmdline = r.TypedSpec().Cmdline

			return nil
		},
	)
}
