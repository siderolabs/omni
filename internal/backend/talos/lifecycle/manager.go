// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle

import (
	"context"
	"sync"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// Manager runs Talos's LifecycleService.Install/Upgrade for machines in maintenance mode. Run
// performs the full flow (pull, install/upgrade, reboot) synchronously and allows only one in-flight operation per machine.
type Manager struct {
	logger             *zap.Logger
	containerdInstance *common.ContainerdInstance
	inFlight           map[string]struct{}
	imageFactoryHost   string
	talosRegistry      string

	mu sync.Mutex
}

// NewManager creates a Manager.
func NewManager(logger *zap.Logger, imageFactoryHost, talosRegistry string) *Manager {
	return &Manager{
		logger:           logger,
		imageFactoryHost: imageFactoryHost,
		talosRegistry:    talosRegistry,
		containerdInstance: &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CONTAINERD,
			Namespace: common.ContainerdNamespace_NS_SYSTEM,
		},
		inFlight: map[string]struct{}{},
	}
}

// runConfig holds the per-call options resolved from Option values.
type runConfig struct {
	audit    AuditFunc
	progress func(string)
}

// Option configures a single Run.
type Option func(*runConfig)

// WithProgress sets the sink for operator-visible progress messages (gRPC stream or logger).
func WithProgress(fn func(string)) Option {
	return func(c *runConfig) {
		if fn != nil {
			c.progress = fn
		}
	}
}

// WithAudit sets the audit hook invoked once per Talos call. Without it, calls are not audited.
func WithAudit(fn AuditFunc) Option {
	return func(c *runConfig) {
		if fn != nil {
			c.audit = fn
		}
	}
}

// SupportsLifecycleManagement verifies the machine runs a Talos version exposing the LifecycleService APIs.
func (m *Manager) SupportsLifecycleManagement(version string) error {
	if _, ok := omni.ParseTalosVersionLifecycleSupport(version); !ok {
		return status.Errorf(codes.FailedPrecondition,
			"machine Talos version %q is not supported; the lifecycle API requires Talos %s or newer",
			version, omni.LifecycleServiceMinTalosVersion)
	}

	return nil
}

// CheckAlive verifies the machine is reachable over the maintenance API with a quick Version call, so callers can fail fast instead of committing the full operation to an unresponsive machine.
func (m *Manager) CheckAlive(ctx context.Context, address string) error {
	talosClient, err := talos.NewMaintenanceClient(ctx, address)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create talos client: %v", err)
	}

	defer talosClient.Close() //nolint:errcheck

	if _, err = talosClient.Version(ctx); err != nil {
		return err
	}

	return nil
}

// Run performs the maintenance flow synchronously, returning when the machine has been rebooted into the target Talos or on error.
// Returns ErrAlreadyInFlight if another operation holds this machine.
func (m *Manager) Run(ctx context.Context, op Operation, opts ...Option) error {
	cfg := runConfig{audit: NoAudit, progress: func(string) {}}
	for _, opt := range opts {
		opt(&cfg)
	}

	if !m.acquire(op.MachineID) {
		return ErrAlreadyInFlight
	}

	defer m.release(op.MachineID)

	installImageStr, err := m.buildInstallImage(op.MachineID, op.MachineStatus, op.Version, op.InstallImage)
	if err != nil {
		return WrapErr(err, "failed to build install image")
	}

	m.logger.Info("built maintenance lifecycle image",
		zap.String("machine_id", op.MachineID),
		zap.String("image", installImageStr),
		zap.Stringer("operation", op.Kind))

	talosClient, err := talos.NewMaintenanceClient(ctx, op.MachineStatus.TypedSpec().Value.ManagementAddress)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create talos client: %v", err)
	}

	defer talosClient.Close() //nolint:errcheck

	// Install/Upgrade expect the image already present in containerd so we pull it here.
	resolvedImage, err := m.pullInstallerImage(ctx, talosClient, installImageStr, op.MachineID, cfg)
	if err != nil {
		return WrapErr(err, "failed to pull installer image")
	}

	switch op.Kind {
	case KindInstall:
		err = m.install(ctx, talosClient, resolvedImage, op.Disk, op.MachineID, cfg)

		// AlreadyExists means Talos already has Talos on disk, which is the precondition Upgrade needs.
		// The intent is "bring the machine to the target image", so fall back to upgrade.
		if status.Code(err) == codes.AlreadyExists {
			cfg.progress("[omni] Talos is already installed on disk; falling back to upgrade")

			m.logger.Info("install rejected as already installed, falling back to upgrade", zap.String("machine_id", op.MachineID))

			err = m.upgrade(ctx, talosClient, resolvedImage, op.MachineID, cfg)
		}

		if err != nil {
			return err
		}
	case KindUpgrade:
		if err = m.upgrade(ctx, talosClient, resolvedImage, op.MachineID, cfg); err != nil {
			return err
		}
	default:
		return status.Errorf(codes.InvalidArgument, "unknown operation kind: %d", op.Kind)
	}

	// Renew context for the reboot so it doesn't run on residual timeout budget.
	rebootCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), RebootTimeout)
	defer cancel()

	if err = m.reboot(rebootCtx, talosClient, op.MachineID, cfg); err != nil {
		return WrapErr(err, "failed to reboot machine after lifecycle operation")
	}

	return nil
}

// acquire claims the in-flight slot for the machine, returning false if one is already held.
func (m *Manager) acquire(machineID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.inFlight[machineID]; ok {
		return false
	}

	m.inFlight[machineID] = struct{}{}

	return true
}

// release frees the in-flight slot for the machine.
func (m *Manager) release(machineID string) {
	m.mu.Lock()
	delete(m.inFlight, machineID)
	m.mu.Unlock()
}
