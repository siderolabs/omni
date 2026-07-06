// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package snapshot implements a task which collects MachineStatus resource from a Machine.
package snapshot

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	machinetask "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/internal/task/machine"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

const (
	// bootIDProbeInterval is how often the collect task checks whether the machine bootID is stale.
	bootIDProbeInterval = 30 * time.Second
	// bootIDCheckTimeout is how long the collect task waits for the machine bootID to be available.
	bootIDCheckTimeout = 5 * time.Second
)

// InfoChan is a channel for sending machine info from tasks back to the controller.
type InfoChan chan<- *omni.MachineStatusSnapshot

// CollectTaskSpec describes a task to collect machine information.
type CollectTaskSpec struct {
	_ [0]func() // make uncomparable

	TalosConfig *omni.TalosConfig
	Endpoint    string
	MachineID   string
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
	if spec.Endpoint != other.Endpoint {
		return false
	}

	if !resourceEqual(spec.TalosConfig, other.TalosConfig) {
		return false
	}

	return true
}

// ID returns the task ID.
func (spec CollectTaskSpec) ID() string {
	return spec.MachineID
}

func (spec CollectTaskSpec) sendInfo(ctx context.Context, info *omni.MachineStatusSnapshot, notifyCh InfoChan) bool {
	return channel.SendWithContext(ctx, notifyCh, info)
}

// RunTask runs the machine status collector.
func (spec CollectTaskSpec) RunTask(ctx context.Context, logger *zap.Logger, notifyCh InfoChan) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	client, err := spec.getClient(ctx)
	if err != nil {
		return err
	}

	defer client.Close() //nolint:errcheck

	registeredTypes, err := machinetask.QueryRegisteredTypes(ctx, client.COSI)
	if err != nil {
		return err
	}

	if _, registered := registeredTypes[runtime.MachineStatusType]; !registered {
		return nil
	}

	bootID := spec.readBootID(ctx, client, logger)

	watchCh := make(chan state.Event)

	if err = client.COSI.Watch(ctx, runtime.NewMachineStatus().Metadata(), watchCh); err != nil {
		return err
	}

	bootIDTicker := time.NewTicker(bootIDProbeInterval)
	defer bootIDTicker.Stop()

	for {
		var event state.Event

		select {
		case <-ctx.Done():
			return nil
		case <-bootIDTicker.C:
			currentBootID := spec.readBootID(ctx, client, logger)
			if bootID != currentBootID {
				return fmt.Errorf("boot id changed, restarting the task (old: %s, new: %s)", bootID, currentBootID)
			}

			continue
		case event = <-watchCh:
		}

		switch event.Type {
		case state.Errored:
			return fmt.Errorf("error watching COSI resource: %w", event.Error)
		case state.Bootstrapped, state.Destroyed, state.Noop:
			// ignore
		case state.Created, state.Updated:
			snapshot := omni.NewMachineStatusSnapshot(spec.MachineID)

			machineStatusSpec := event.Resource.Spec().(*runtime.MachineStatusSpec) //nolint:forcetypeassert,errcheck

			ev, err := convertStatus(machineStatusSpec)
			if err != nil {
				return err
			}

			snapshot.TypedSpec().Value.MachineStatus = ev
			snapshot.TypedSpec().Value.BootId = bootID

			if !spec.sendInfo(ctx, snapshot, notifyCh) {
				return nil
			}
		}
	}
}

// readBootID reads the machine's kernel boot identifier from the node. It is best-effort: on certain scenarios boot_id might not be available, instead of failing with error it returns "".
func (spec CollectTaskSpec) readBootID(ctx context.Context, c *client.Client, logger *zap.Logger) string {
	ctx, cancel := context.WithTimeout(ctx, bootIDCheckTimeout)
	defer cancel()

	read := func() (string, error) {
		r, err := c.Read(ctx, "/proc/sys/kernel/random/boot_id")
		if err != nil {
			return "", err
		}

		defer r.Close() //nolint:errcheck

		data, err := io.ReadAll(r)
		if err != nil {
			return "", err
		}

		return strings.TrimSpace(string(data)), nil
	}

	bootID, err := read()
	if err != nil {
		logger.Debug("failed to read boot id", zap.Error(err))

		return ""
	}

	return bootID
}

func (spec CollectTaskSpec) getClient(ctx context.Context) (*client.Client, error) {
	opts := talos.GetSocketOptions(spec.Endpoint)

	talosConfig := spec.TalosConfig

retry:
	if talosConfig == nil {
		opts = append(opts, client.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}), client.WithEndpoints(spec.Endpoint))

		return client.New(ctx, opts...)
	}

	config := omni.NewTalosClientConfig(spec.TalosConfig, spec.Endpoint)

	opts = append(opts, client.WithConfig(config))

	c, err := client.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("error building Talos API client: %w", err)
	}

	// if the request failed retry once again with the insecure client
	_, err = c.Version(ctx)
	if err != nil {
		talosConfig = nil

		goto retry
	}

	return c, nil
}

func convertStage(stage runtime.MachineStage) (machine.MachineStatusEvent_MachineStage, error) {
	switch stage {
	case runtime.MachineStageUnknown:
		return machine.MachineStatusEvent_UNKNOWN, nil
	case runtime.MachineStageBooting:
		return machine.MachineStatusEvent_BOOTING, nil
	case runtime.MachineStageInstalling:
		return machine.MachineStatusEvent_INSTALLING, nil
	case runtime.MachineStageMaintenance:
		return machine.MachineStatusEvent_MAINTENANCE, nil
	case runtime.MachineStageRunning:
		return machine.MachineStatusEvent_RUNNING, nil
	case runtime.MachineStageRebooting:
		return machine.MachineStatusEvent_REBOOTING, nil
	case runtime.MachineStageShuttingDown:
		return machine.MachineStatusEvent_SHUTTING_DOWN, nil
	case runtime.MachineStageResetting:
		return machine.MachineStatusEvent_RESETTING, nil
	case runtime.MachineStageUpgrading:
		return machine.MachineStatusEvent_UPGRADING, nil
	default:
		return machine.MachineStatusEvent_UNKNOWN, fmt.Errorf("unknown stage: %d", stage)
	}
}

func convertStatus(spec *runtime.MachineStatusSpec) (*machine.MachineStatusEvent, error) {
	statusEventMachineStage, err := convertStage(spec.Stage)
	if err != nil {
		return nil, err
	}

	return &machine.MachineStatusEvent{
		Stage: statusEventMachineStage,
		Status: &machine.MachineStatusEvent_MachineStatus{
			Ready: spec.Status.Ready,
			UnmetConditions: xslices.Map(spec.Status.UnmetConditions, func(t runtime.UnmetCondition) *machine.MachineStatusEvent_MachineStatus_UnmetCondition {
				return &machine.MachineStatusEvent_MachineStatus_UnmetCondition{
					Name:   t.Name,
					Reason: t.Reason,
				}
			}),
		},
	}, nil
}
