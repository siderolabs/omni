// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package machine implements a task which collects information from a Machine (either joined to a cluster or not).
package machine

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// Info contains information gathered about a machine.
type Info struct { //nolint:govet
	TalosVersion  *string
	Arch          *string
	MachineLabels *omni.MachineLabels

	Hostname        *string
	Domainname      *string
	Addresses       []string
	DefaultGateways []string
	NetworkLinks    []*specs.MachineStatusSpec_NetworkStatus_NetworkLinkStatus
	ImageLabels     map[string]string

	Processors    []*specs.MachineStatusSpec_HardwareStatus_Processor
	MemoryModules []*specs.MachineStatusSpec_HardwareStatus_MemoryModule
	Blockdevices  []*specs.MachineStatusSpec_HardwareStatus_BlockDevice

	PlatformMetadata *specs.MachineStatusSpec_PlatformMetadata
	Schematic        *specs.MachineStatusSpec_Schematic

	LastError       error
	MachineID       string
	MaintenanceMode bool
	NoAccess        bool

	DefaultKernelArgs []string
	SecureBootStatus  *specs.MachineStatusSpec_SecureBootStatus
}

// InfoChan is a channel for sending machine info from tasks back to the controller.
type InfoChan chan<- Info

// CollectTaskSpec describes a task to collect machine information.
type CollectTaskSpec struct {
	_ [0]func() // make uncomparable

	TalosConfig                *omni.TalosConfig
	MachineLabels              *omni.MachineLabels
	Endpoint                   string
	MachineID                  string
	DefaultSchematicKernelArgs []string
	MaintenanceMode            bool
}

func resourceEqual[T any, S interface {
	resource.Resource
	*T
}](a, b S) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return resource.Equal(a, b)
}

// Equal compares two task specs for the same machine.
//
// If the task spec changes, the task will be restarted.
func (spec CollectTaskSpec) Equal(other CollectTaskSpec) bool {
	if spec.Endpoint != other.Endpoint || spec.MaintenanceMode != other.MaintenanceMode {
		return false
	}

	if !resourceEqual(spec.TalosConfig, other.TalosConfig) {
		return false
	}

	return resourceEqual(spec.MachineLabels, other.MachineLabels)
}

// ID returns the task ID.
func (spec CollectTaskSpec) ID() string {
	return spec.MachineID
}

func (spec CollectTaskSpec) sendInfo(ctx context.Context, info Info, notifyCh InfoChan, err error) bool {
	info.MaintenanceMode = spec.MaintenanceMode
	info.MachineID = spec.MachineID
	info.DefaultKernelArgs = spec.DefaultSchematicKernelArgs

	if err != nil {
		switch {
		case spec.MaintenanceMode && status.Code(err) == codes.Unavailable && strings.Contains(err.Error(), "tls: bad certificate") ||
			strings.Contains(err.Error(), "tls: certificate required") ||
			strings.Contains(err.Error(), "x509: certificate signed by unknown authority"):
			info.NoAccess = true
			info.LastError = errors.New("service expects the machine to run the maintenance mode, but the machine requires a certificate")
		case strings.Contains(err.Error(), "transport: authentication handshake failed: tls: failed to verify certificate: x509: certificate has expired"):
			info.NoAccess = true

			info.LastError = errors.New("the machine time is out of sync")
		case strings.Contains(err.Error(), "connect: network is unreachable") ||
			strings.Contains(err.Error(), "connect: no route to host"):
			// skip these types of errors as they are most likely caused by Wireguard not being ready
		default:
			info.LastError = errors.New("unknown error")
		}
	}

	return channel.SendWithContext(ctx, notifyCh, info)
}

// RunTask runs the machine info collect task.
//
// It creates either a maintenance Talos API client or a regular one (depends on the spec).
//
// It subscribes to resource updates and polls for resources that can't be watched.
//
//nolint:gocyclo,cyclop,gocognit
func (spec CollectTaskSpec) RunTask(ctx context.Context, logger *zap.Logger, notifyCh InfoChan) error {
	var (
		c   *client.Client
		err error
	)

	opts := talos.GetSocketOptions(spec.Endpoint)

	if spec.MaintenanceMode {
		opts = append(opts, client.WithTLSConfig(insecureTLSConfig), client.WithEndpoints(spec.Endpoint))

		c, err = client.New(ctx, opts...)
	} else {
		if spec.TalosConfig == nil {
			return errors.New("no talosconfig, and not in maintenance mode")
		}

		config := omni.NewTalosClientConfig(spec.TalosConfig, spec.Endpoint)

		opts = append(opts, client.WithConfig(config))

		c, err = client.New(ctx, opts...)
	}

	if err != nil {
		return fmt.Errorf("error building Talos API client: %w", err)
	}

	defer c.Close() //nolint:errcheck

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	const (
		disksPollInterval = 5 * time.Minute
		minPolInterval    = time.Second
	)

	disksTicker := time.NewTicker(disksPollInterval)
	defer disksTicker.Stop()

	pollTicker := time.NewTicker(minPolInterval)
	defer pollTicker.Stop()

	watchCh := make(chan state.Event)

	registeredTypes, err := QueryRegisteredTypes(ctx, c.COSI)
	if err != nil {
		// this is the first request to the Talos API
		// if it fails we handle it and update the machine status with the request error
		if !spec.sendInfo(ctx, Info{}, notifyCh, err) {
			return nil
		}

		return err
	}

	// as Talos < 1.3.0 doesn't support Bootstrapped event, we use a mixed approach:
	// watch is used to trigger polling on changes to the resources
	watchers := map[resource.Type]struct {
		filterFunc             func(r resource.Resource) bool
		namespace              resource.Namespace
		handlePermissionDenied bool
	}{
		// NB: keep in sync with machinePollers
		network.HostnameStatusType: {
			namespace: network.NamespaceName,
		},
		network.LinkStatusType: {
			namespace:  network.NamespaceName,
			filterFunc: typedFilter(filterNetworkLinks),
		},
		network.RouteStatusType: {
			namespace:  network.NamespaceName,
			filterFunc: typedFilter(filterRoutes),
		},
		network.NodeAddressType: {
			namespace:  network.NamespaceName,
			filterFunc: typedFilter(filterAddresses(spec.MaintenanceMode)),
		},
		hardware.ProcessorType: {
			namespace: hardware.NamespaceName,
		},
		hardware.MemoryModuleType: {
			namespace: hardware.NamespaceName,
		},
		runtime.PlatformMetadataType: {
			namespace: runtime.NamespaceName,
		},
		runtime.MetaKeyType: {
			namespace: runtime.NamespaceName,
		},
		runtime.ExtensionStatusType: {
			namespace: runtime.NamespaceName,
		},
	}

	for resourceType, watcher := range watchers {
		if _, registered := registeredTypes[resourceType]; !registered {
			continue
		}

		if err = c.COSI.WatchKind(ctx, resource.NewMetadata(watcher.namespace, resourceType, "", resource.VersionUndefined), watchCh); err != nil {
			if code := status.Code(err); code == codes.PermissionDenied && watcher.handlePermissionDenied {
				logger.Info("permission denied when watching resource, ignoring", zap.String("resource_type", resourceType))

				continue
			}

			return fmt.Errorf("error watching COSI resource: %w", err)
		}
	}

	dirtyPollers := map[string]struct{}{}

	// mark everything as dirty on start
	for k := range machinePollers {
		dirtyPollers[k] = struct{}{}
	}

	for k := range resourcePollers {
		if _, ok := registeredTypes[k]; !ok {
			continue
		}

		dirtyPollers[k] = struct{}{}
	}

	for {
		if len(dirtyPollers) > 0 {
			info, err := spec.poll(ctx, c, maps.Keys(dirtyPollers))

			if !spec.sendInfo(ctx, info, notifyCh, err) {
				return nil
			}

			if err != nil {
				return fmt.Errorf("poll failed: %w", err)
			}

			dirtyPollers = map[string]struct{}{}
		}

	waitLoop:
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-disksTicker.C:
				// poll disks as we have no way to watch for changes
				dirtyPollers["disks"] = struct{}{}
			case <-pollTicker.C:
				break waitLoop
			case event := <-watchCh:
				switch event.Type {
				case state.Errored:
					return fmt.Errorf("error watching COSI resource: %w", event.Error)
				case state.Bootstrapped:
					// ignore
				case state.Created, state.Updated, state.Destroyed:
					markAsDirty := true

					if watchers[event.Resource.Metadata().Type()].filterFunc != nil && !resource.IsTombstone(event.Resource) { // can't run filter for tombstones
						markAsDirty = watchers[event.Resource.Metadata().Type()].filterFunc(event.Resource)
					}

					if markAsDirty {
						dirtyPollers[event.Resource.Metadata().Type()] = struct{}{}
					}
				}
			}
		}
	}
}

func (spec CollectTaskSpec) poll(ctx context.Context, c *client.Client, pollers []string) (Info, error) {
	info := Info{
		// set this early to make pollers act on the maintenance/normal mode
		MaintenanceMode: spec.MaintenanceMode,
		// set this early to make pollers act on the machine labels
		MachineLabels: spec.MachineLabels,
		// set this early to allow machine schematic collector to be able to fall back to the default kernel args if they cannot be read from the Talos API
		DefaultKernelArgs: spec.DefaultSchematicKernelArgs,
	}

	for _, poller := range pollers {
		if err := poll(ctx, poller, c, &info); err != nil {
			return info, err
		}
	}

	return info, nil
}

var insecureTLSConfig = &tls.Config{
	InsecureSkipVerify: true,
}

func typedFilter[T resource.Resource](fn func(T) bool) func(r resource.Resource) bool {
	return func(r resource.Resource) bool {
		arg, ok := r.(T)
		if !ok {
			return false
		}

		return fn(arg)
	}
}
