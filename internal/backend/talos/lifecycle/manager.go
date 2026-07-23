// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle

import (
	"context"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/go-kubernetes/kubernetes/nodedrain"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// KubernetesClientProvider resolves a cluster's Kubernetes clientset for cordon/drain/uncordon.
type KubernetesClientProvider interface {
	GetKubernetesClientset(ctx context.Context, cluster string) (k8s.Interface, error)
}

// TalosClientFactory hands out a cached Talos client for a machine. The returned client must not be closed by the caller.
type TalosClientFactory interface {
	GetForMachine(ctx context.Context, machineID string) (*talos.Client, error)
}

// Manager runs Talos's LifecycleService install/upgrade for a single machine, one in-flight op per machine.
type Manager struct {
	kubernetesClientProvider KubernetesClientProvider
	talosClientFactory       TalosClientFactory
	logger                   *zap.Logger
	containerdInstance       *common.ContainerdInstance
	inFlight                 map[string]struct{}
	installEventCh           chan<- resource.ID
	imageFactoryHost         string
	talosRegistry            string
	mu                       sync.Mutex
}

// NewManager creates a Manager. kubernetesClients may be nil for callers that never cordon/drain.
// installEventCh may be nil for callers that don't track install completion (e.g. tests).
func NewManager(logger *zap.Logger, imageFactoryHost, talosRegistry string, kubernetesClientProvider KubernetesClientProvider,
	talosClientFactory TalosClientFactory, installEventCh chan<- resource.ID,
) *Manager {
	return &Manager{
		logger:                   logger,
		imageFactoryHost:         imageFactoryHost,
		talosRegistry:            talosRegistry,
		kubernetesClientProvider: kubernetesClientProvider,
		talosClientFactory:       talosClientFactory,
		containerdInstance: &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CONTAINERD,
			Namespace: common.ContainerdNamespace_NS_SYSTEM,
		},
		inFlight:       map[string]struct{}{},
		installEventCh: installEventCh,
	}
}

// ImageFactoryHost is the image factory host used to build install images, shared by every caller
// so they don't each need their own copy of what is really a single deployment-wide setting.
func (m *Manager) ImageFactoryHost() string {
	return m.imageFactoryHost
}

// TalosRegistry is the Talos installer registry used when a machine has no schematic, shared by every
// caller so they don't each need their own copy of what is really a single deployment-wide setting.
func (m *Manager) TalosRegistry() string {
	return m.talosRegistry
}

// clientset resolves the cluster's Kubernetes clientset, erroring when no provider was configured.
func (m *Manager) clientset(ctx context.Context, cluster string) (k8s.Interface, error) {
	if m.kubernetesClientProvider == nil {
		return nil, status.Error(codes.Internal, "kubernetes client provider not configured, cannot cordon/drain/uncordon node")
	}

	return m.kubernetesClientProvider.GetKubernetesClientset(ctx, cluster)
}

// PreRebootHook runs before the node is drained. An error aborts the Run.
type PreRebootHook func(ctx context.Context, talosClient *talosclient.Client) error

// nodeRef names a cluster node.
type nodeRef struct {
	clusterName string
	nodeName    string
}

// runConfig holds the per-call options resolved from Option values.
type runConfig struct {
	audit       AuditFunc
	progress    func(string)
	cordonDrain *nodeRef
	uncordon    *nodeRef
	preReboot   []PreRebootHook
}

// Option configures a single Run.
type Option func(*runConfig)

// WithCordonDrain makes Run cordon and drain the node before reboot. Set for cluster members.
func WithCordonDrain(clusterName, nodeName string) Option {
	return func(cfg *runConfig) {
		if clusterName != "" && nodeName != "" {
			cfg.cordonDrain = &nodeRef{clusterName: clusterName, nodeName: nodeName}
		}
	}
}

// WithUncordon makes FinalizeReboot uncordon the node once it is back up.
func WithUncordon(clusterName, nodeName string) Option {
	return func(cfg *runConfig) {
		if clusterName != "" && nodeName != "" {
			cfg.uncordon = &nodeRef{clusterName: clusterName, nodeName: nodeName}
		}
	}
}

// WithPreRebootHooks registers hooks to run after the install/upgrade and before the node is drained, in order.
func WithPreRebootHooks(hooks ...PreRebootHook) Option {
	return func(cfg *runConfig) {
		for _, h := range hooks {
			if h != nil {
				cfg.preReboot = append(cfg.preReboot, h)
			}
		}
	}
}

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
			"machine Talos version %q is not supported: the lifecycle API requires Talos %s or newer",
			version, omni.LifecycleServiceMinTalosVersion)
	}

	return nil
}

// GetForMachine returns the cached Talos client for a machine, selecting maintenance or cluster mode from
// live state. The client is owned by the factory and must not be closed by the caller.
func (m *Manager) GetForMachine(ctx context.Context, machineID string) (*talos.Client, error) {
	if m.talosClientFactory == nil {
		return nil, status.Error(codes.Internal, "talos client factory not configured")
	}

	return m.talosClientFactory.GetForMachine(ctx, machineID)
}

// CheckAlive does a quick Version call on the machine's Talos client, so callers can fail fast instead of committing the full operation to an unresponsive machine.
func (m *Manager) CheckAlive(ctx context.Context, machineID string) error {
	talosClient, err := m.GetForMachine(ctx, machineID)
	if err != nil {
		return err
	}

	if _, err = talosClient.Version(ctx); err != nil {
		return err
	}

	return nil
}

// Run performs the install or upgrade synchronously, returning when the machine has been rebooted into the target Talos or on error.
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

	m.logger.Info("built installer image",
		zap.String("machine_id", op.MachineID),
		zap.String("image", installImageStr),
		zap.Stringer("operation", op.Kind))

	// The factory owns the cached client, so we never close it.
	nodeClient, err := m.GetForMachine(ctx, op.MachineID)
	if err != nil {
		return err
	}

	talosClient := nodeClient.Client

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

	for _, hook := range cfg.preReboot {
		if err = m.runPreRebootHook(ctx, hook, talosClient); err != nil {
			return WrapErr(err, "pre-reboot hook failed")
		}
	}

	if err = m.runCordonAndDrain(ctx, cfg); err != nil {
		return WrapErr(err, "cordon/drain before reboot failed")
	}

	if err = m.reboot(ctx, talosClient, op.MachineID, cfg); err != nil {
		return WrapErr(err, "failed to reboot machine after install/upgrade")
	}

	// The install succeeded and the machine rebooted into it. This path emits no Talos boot event for
	// the event sink to catch, so signal it here to bump InstallEventId.
	if op.Kind == KindInstall && m.installEventCh != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m.installEventCh <- op.MachineID:
		}
	}

	return nil
}

// runPreRebootHook runs a single pre-reboot hook under its own timeout.
func (m *Manager) runPreRebootHook(ctx context.Context, hook PreRebootHook, talosClient *talosclient.Client) error {
	ctx, cancel := context.WithTimeout(ctx, PreRebootHookTimeout)
	defer cancel()

	return hook(ctx, talosClient)
}

// runCordonAndDrain cordons and drains the node when WithCordonDrain was set.
func (m *Manager) runCordonAndDrain(ctx context.Context, cfg runConfig) error {
	if cfg.cordonDrain == nil {
		return nil
	}

	target := cfg.cordonDrain

	clientset, err := m.clientset(ctx, target.clusterName)
	if err != nil {
		return WrapErr(err, "failed to get kubernetes client to cordon/drain node")
	}

	cordonCtx, cancel := context.WithTimeout(ctx, CordonTimeout)
	defer cancel()

	emitf(cfg.progress, "[omni] cordoning node %s", target.nodeName)

	if err = nodedrain.Cordon(cordonCtx, clientset, target.nodeName); err != nil {
		return err
	}

	emitf(cfg.progress, "[omni] draining node %s", target.nodeName)

	// Drain bounds itself with its own DefaultDrainTimeout.
	if err = nodedrain.Drain(ctx, clientset, target.nodeName, nodedrain.DrainOptions{
		Progress: func(msg string) { cfg.progress("[omni] " + msg) },
	}); err != nil {
		return err
	}

	emitf(cfg.progress, "[omni] node %s drained", target.nodeName)

	return nil
}

// FinalizeReboot uncordons the node, separate from Run because the caller decides when the machine is back.
func (m *Manager) FinalizeReboot(ctx context.Context, opts ...Option) error {
	cfg := runConfig{progress: func(string) {}}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.uncordon == nil {
		return nil
	}

	target := cfg.uncordon

	clientset, err := m.clientset(ctx, target.clusterName)
	if err != nil {
		return WrapErr(err, "failed to get kubernetes client to uncordon node")
	}

	uncordonCtx, cancel := context.WithTimeout(ctx, CordonTimeout)
	defer cancel()

	emitf(cfg.progress, "[omni] uncordoning node %s", target.nodeName)

	if err = nodedrain.Uncordon(uncordonCtx, clientset, target.nodeName); err != nil {
		return WrapErr(err, "failed to uncordon node")
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
