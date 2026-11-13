// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package lifecycle

import (
	"context"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-kubernetes/kubernetes/nodedrain"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/imagefactory"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
)

// KubernetesClientProvider resolves a cluster's Kubernetes clientset for cordon/drain/uncordon.
type KubernetesClientProvider interface {
	GetKubernetesClientset(ctx context.Context, cluster string) (k8s.Interface, error)
}

// TalosClientFactory hands out a Talos client for a machine. The returned client must be closed by the caller.
type TalosClientFactory interface {
	GetForMachine(ctx context.Context, machineID string) (*talos.Client, error)
}

// operationState represents the operation running on a machine, or the last failed one.
type operationState struct {
	spec           *specs.MachineOperationSpec
	inFlight       bool
	machineRemoved bool
}

// Manager runs Talos's LifecycleService install/upgrade for a single machine, one in-flight op per machine.
type Manager struct {
	kubernetesClientProvider KubernetesClientProvider
	talosClientFactory       TalosClientFactory
	state                    state.State
	logger                   *zap.Logger
	containerdInstance       *common.ContainerdInstance
	operations               map[string]*operationState
	installEventCh           chan<- resource.ID
	imageFactoryClients      *imagefactory.Clients
	mu                       sync.Mutex
}

// NewManager creates a Manager. kubernetesClients may be nil for callers that never cordon/drain.
// installEventCh may be nil for callers that don't track install completion (e.g. tests).
// st may be nil for callers that don't track the operations in MachineOperation resources (e.g. tests).
func NewManager(
	logger *zap.Logger,
	imageFactoryClients *imagefactory.Clients,
	kubernetesClientProvider KubernetesClientProvider,
	talosClientFactory TalosClientFactory,
	installEventCh chan<- resource.ID,
	st state.State,
) *Manager {
	return &Manager{
		logger:                   logger,
		imageFactoryClients:      imageFactoryClients,
		kubernetesClientProvider: kubernetesClientProvider,
		talosClientFactory:       talosClientFactory,
		state:                    st,
		containerdInstance: &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CONTAINERD,
			Namespace: common.ContainerdNamespace_NS_SYSTEM,
		},
		operations:     map[string]*operationState{},
		installEventCh: installEventCh,
	}
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

// nodeRef names a cluster node. With bestEffort, cordon/drain failures do not abort the operation.
type nodeRef struct {
	clusterName string
	nodeName    string
	bestEffort  bool
}

// runConfig holds the per-call options resolved from Option values.
type runConfig struct {
	audit       AuditFunc
	progress    func(string)
	stage       func(specs.MachineOperationSpec_Stage)
	cordonDrain *nodeRef
	uncordon    *nodeRef
	preReboot   []PreRebootHook
}

// Option configures a single Run.
type Option func(*runConfig)

// WithCordonDrain makes Run cordon and drain the node before reboot. Set for cluster members.
//
// With bestEffort, cordon and drain failures do not abort the operation. Set it when the operation
// is the recovery path of a machine that is itself unhealthy: nothing may block its reboot then,
// and the workload Kubernetes API may be served by the very machine being recovered. The drain is
// still attempted, because an unhealthy machine does not imply an unhealthy node: a machine can
// stay Talos-not-ready (e.g. an extension service missing its configuration) while its node keeps
// serving pods, and those deserve an eviction before the reboot. On a node that cannot finish
// evictions the attempt costs at most the drain timeout, acceptable for a recovery.
func WithCordonDrain(clusterName, nodeName string, bestEffort bool) Option {
	return func(cfg *runConfig) {
		if clusterName != "" && nodeName != "" {
			cfg.cordonDrain = &nodeRef{clusterName: clusterName, nodeName: nodeName, bestEffort: bestEffort}
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

// GetForMachine returns the Talos client for a machine, selecting maintenance or cluster mode from live state.
// The returned client must be closed by the caller.
func (m *Manager) GetForMachine(ctx context.Context, machineID string) (*talos.Client, error) {
	if m.talosClientFactory == nil {
		return nil, status.Error(codes.Internal, "talos client factory not configured")
	}

	return m.talosClientFactory.GetForMachine(ctx, machineID)
}

// Run performs the install or upgrade synchronously, returning when the machine has been rebooted into the target Talos or on error.
// Returns ErrAlreadyInFlight if another operation holds this machine.
func (m *Manager) Run(ctx context.Context, op Operation, opts ...Option) (err error) {
	cfg := runConfig{audit: NoAudit, progress: func(string) {}}
	for _, opt := range opts {
		opt(&cfg)
	}

	entry, acquired := m.acquire(ctx, op.MachineID, specs.MachineOperationSpec_PREPARING_IMAGE)
	if !acquired {
		return ErrAlreadyInFlight
	}

	var (
		rebooted bool
		bootID   string
	)

	defer func() {
		switch {
		case !rebooted:
			m.release(ctx, op.MachineID, entry, err)
		case bootID != "":
			m.releaseRebooting(ctx, op.MachineID, entry, bootID)
		default:
			m.release(ctx, op.MachineID, entry, nil) // the machine rebooted into the target, even if the install event was not sent
		}
	}()

	m.track(ctx, op.MachineID, entry, &cfg)

	installImageStr, err := m.buildInstallImage(ctx, op.MachineID, op.MachineStatus, op.Version, op.InstallImage)
	if err != nil {
		return WrapErr(err, "failed to build install image")
	}

	m.logger.Info("built installer image",
		zap.String("machine_id", op.MachineID),
		zap.String("image", installImageStr),
		zap.Stringer("operation", op.Kind))

	// the client is kept open for the whole operation
	nodeClient, err := m.GetForMachine(ctx, op.MachineID)
	if err != nil {
		return err
	}

	defer nodeClient.Close() //nolint:errcheck

	talosClient := nodeClient.Client

	cfg.stage(specs.MachineOperationSpec_PULLING_IMAGE)

	// Install/Upgrade expect the image already present in containerd so we pull it here.
	resolvedImage, err := m.pullInstallerImage(ctx, talosClient, installImageStr, op.MachineID, cfg)
	if err != nil {
		return WrapErr(err, "failed to pull installer image")
	}

	if err = m.installOrUpgrade(ctx, talosClient, resolvedImage, op, cfg); err != nil {
		return err
	}

	if len(cfg.preReboot) > 0 {
		cfg.stage(specs.MachineOperationSpec_PREPARING_REBOOT)
	}

	for _, hook := range cfg.preReboot {
		if err = m.runPreRebootHook(ctx, hook, talosClient); err != nil {
			return WrapErr(err, "pre-reboot hook failed")
		}
	}

	if err = m.runCordonAndDrain(ctx, cfg); err != nil {
		return WrapErr(err, "cordon/drain before reboot failed")
	}

	cfg.stage(specs.MachineOperationSpec_REBOOTING)

	bootID, err = m.bootID(ctx, op.MachineID)
	if err != nil {
		return err
	}

	if err = m.reboot(ctx, talosClient, op.MachineID, cfg); err != nil {
		return WrapErr(err, "failed to reboot machine after install/upgrade")
	}

	rebooted = true

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

func (m *Manager) installOrUpgrade(ctx context.Context, talosClient *talosclient.Client, resolvedImage string, op Operation, cfg runConfig) error {
	switch op.Kind {
	case KindInstall:
		cfg.stage(specs.MachineOperationSpec_INSTALLING)

		err := m.install(ctx, talosClient, resolvedImage, op.Disk, op.MachineID, cfg)

		// AlreadyExists means Talos already has Talos on disk, which is the precondition Upgrade needs.
		// The intent is "bring the machine to the target image", so fall back to upgrade.
		if status.Code(err) == codes.AlreadyExists {
			cfg.stage(specs.MachineOperationSpec_UPGRADING)

			cfg.progress("[omni] Talos is already installed on disk; falling back to upgrade")

			m.logger.Info("install rejected as already installed, falling back to upgrade", zap.String("machine_id", op.MachineID))

			err = m.upgrade(ctx, talosClient, resolvedImage, op.MachineID, cfg)
		}

		return err
	case KindUpgrade:
		cfg.stage(specs.MachineOperationSpec_UPGRADING)

		return m.upgrade(ctx, talosClient, resolvedImage, op.MachineID, cfg)
	default:
		return status.Errorf(codes.InvalidArgument, "unknown operation kind: %d", op.Kind)
	}
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

	cfg.stage(specs.MachineOperationSpec_CORDONING)

	clientset, err := m.clientset(ctx, target.clusterName)
	if err != nil {
		err = WrapErr(err, "failed to get kubernetes client to cordon/drain node")

		if target.bestEffort {
			emitf(cfg.progress, "[omni] skipping cordon and drain of node %s: %s", target.nodeName, err)

			return nil
		}

		return err
	}

	cordonCtx, cancel := context.WithTimeout(ctx, CordonTimeout)
	defer cancel()

	emitf(cfg.progress, "[omni] cordoning node %s", target.nodeName)

	if err = nodedrain.Cordon(cordonCtx, clientset, target.nodeName, omni.KubernetesNodeCordoned); err != nil {
		if target.bestEffort {
			emitf(cfg.progress, "[omni] failed to cordon node %s, proceeding to reboot: %s", target.nodeName, err)

			return nil
		}

		return err
	}

	if err = drain(ctx, clientset, cfg, target.nodeName); err != nil {
		// See WithCordonDrain for why a best-effort operation tolerates a failed drain.
		if target.bestEffort {
			emitf(cfg.progress, "[omni] failed to drain node %s, proceeding to reboot: %s", target.nodeName, err)

			return nil
		}

		return err
	}

	return nil
}

// drain evicts the node's pods, bounding itself with nodedrain's own drain timeout.
func drain(ctx context.Context, clientset k8s.Interface, cfg runConfig, nodeName string) error {
	cfg.stage(specs.MachineOperationSpec_DRAINING)

	emitf(cfg.progress, "[omni] draining node %s", nodeName)

	if err := nodedrain.Drain(ctx, clientset, nodeName, nodedrain.DrainOptions{
		Progress: func(msg string) { cfg.progress("[omni] " + msg) },
	}); err != nil {
		return err
	}

	emitf(cfg.progress, "[omni] node %s drained", nodeName)

	return nil
}

// FinalizeReboot uncordons the node, separate from Run because the caller decides when the machine is back.
// Returns ErrAlreadyInFlight if another operation holds this machine.
func (m *Manager) FinalizeReboot(ctx context.Context, machineID string, opts ...Option) (err error) {
	cfg := runConfig{progress: func(string) {}}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.uncordon == nil {
		return nil
	}

	entry, acquired := m.acquire(ctx, machineID, specs.MachineOperationSpec_UNCORDONING)
	if !acquired {
		return ErrAlreadyInFlight
	}

	defer func() { m.release(ctx, machineID, entry, err) }()

	m.track(ctx, machineID, entry, &cfg)

	target := cfg.uncordon

	clientset, err := m.clientset(ctx, target.clusterName)
	if err != nil {
		return WrapErr(err, "failed to get kubernetes client to uncordon node")
	}

	uncordonCtx, cancel := context.WithTimeout(ctx, CordonTimeout)
	defer cancel()

	emitf(cfg.progress, "[omni] uncordoning node %s", target.nodeName)

	if err = nodedrain.Uncordon(uncordonCtx, clientset, target.nodeName, omni.KubernetesNodeCordoned); err != nil {
		return WrapErr(err, "failed to uncordon node")
	}

	return nil
}

// Forget drops the failed or the kept operation of the machine. It does nothing while an operation is in flight,
// unless the machine is removed: then the operation is dropped as soon as it finishes.
func (m *Manager) Forget(machineID string, machineRemoved bool) {
	m.update(context.Background(), machineID, func() bool {
		entry, ok := m.operations[machineID]
		if !ok {
			return false
		}

		if entry.inFlight {
			entry.machineRemoved = entry.machineRemoved || machineRemoved

			return false
		}

		delete(m.operations, machineID)

		return true
	})
}

func (m *Manager) track(ctx context.Context, machineID string, entry *operationState, cfg *runConfig) {
	progress := cfg.progress

	cfg.progress = func(msg string) {
		m.setMessage(ctx, machineID, entry, msg)

		progress(msg)
	}

	cfg.stage = func(stage specs.MachineOperationSpec_Stage) {
		m.setStage(ctx, machineID, entry, stage)
	}
}

// acquire claims the in-flight slot for the machine and starts a new operation in the given stage, returning false if one is already held.
func (m *Manager) acquire(ctx context.Context, machineID string, stage specs.MachineOperationSpec_Stage) (*operationState, bool) {
	var entry *operationState

	acquired := m.update(ctx, machineID, func() bool {
		if current, ok := m.operations[machineID]; ok && current.inFlight {
			return false
		}

		entry = &operationState{
			spec: &specs.MachineOperationSpec{
				Stage:          stage,
				StageStartedAt: timestamppb.Now(),
			},
			inFlight: true,
		}

		m.operations[machineID] = entry

		return true
	})

	return entry, acquired
}

// release frees the in-flight slot for the machine. A failed operation is kept with its error and the stage it failed in.
func (m *Manager) release(ctx context.Context, machineID string, entry *operationState, err error) {
	m.update(ctx, machineID, func() bool {
		if entry == nil || m.operations[machineID] != entry {
			return false
		}

		if err == nil || entry.machineRemoved {
			delete(m.operations, machineID)

			return true
		}

		entry.spec = &specs.MachineOperationSpec{
			Message:     entry.spec.Message,
			LastError:   status.Convert(err).Message(),
			FailedStage: entry.spec.Stage,
		}
		entry.inFlight = false

		return true
	})
}

// releaseRebooting frees the in-flight slot for the machine and keeps its REBOOTING stage with the boot ID the machine had before the reboot.
func (m *Manager) releaseRebooting(ctx context.Context, machineID string, entry *operationState, bootID string) {
	m.update(ctx, machineID, func() bool {
		if m.operations[machineID] != entry {
			return false
		}

		if entry.machineRemoved {
			delete(m.operations, machineID)

			return true
		}

		entry.spec.PreRebootBootId = bootID
		entry.inFlight = false

		return true
	})
}

// setStage and setMessage ignore an operation which is no longer in flight, as late progress may arrive after it ended.
func (m *Manager) setStage(ctx context.Context, machineID string, entry *operationState, stage specs.MachineOperationSpec_Stage) {
	m.update(ctx, machineID, func() bool {
		if m.operations[machineID] != entry || !entry.inFlight {
			return false
		}

		entry.spec = &specs.MachineOperationSpec{
			Stage:          stage,
			StageStartedAt: timestamppb.Now(),
		}

		return true
	})
}

func (m *Manager) setMessage(ctx context.Context, machineID string, entry *operationState, msg string) {
	m.update(ctx, machineID, func() bool {
		if m.operations[machineID] != entry || !entry.inFlight {
			return false
		}

		entry.spec.Message = msg

		return true
	})
}

// bootID returns the boot ID of the machine, or an empty string if it is not known.
func (m *Manager) bootID(ctx context.Context, machineID string) (string, error) {
	if m.state == nil {
		return "", nil
	}

	snapshot, err := safe.StateGetByID[*omni.MachineStatusSnapshot](ctx, m.state, machineID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", nil
		}

		return "", err
	}

	return snapshot.TypedSpec().Value.BootId, nil
}

// update runs change and writes the operation of the machine if change reports a change.
// The write is done under m.mu, so that the writes of a machine are done in the order of its changes.
func (m *Manager) update(ctx context.Context, machineID string, change func() bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !change() {
		return false
	}

	var spec *specs.MachineOperationSpec

	if entry, ok := m.operations[machineID]; ok {
		spec = entry.spec.CloneVT()
	}

	m.writeOperation(ctx, machineID, spec)

	return true
}

// writeOperation writes the MachineOperation of the machine, or destroys it if there is no operation.
// A failed write is only logged, so that it never fails the operation.
func (m *Manager) writeOperation(ctx context.Context, machineID string, spec *specs.MachineOperationSpec) {
	if m.state == nil {
		return
	}

	// not canceled with the caller, so that an operation canceled by its caller still records its failure
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), operationWriteTimeout)
	defer cancel()

	var err error

	if spec == nil {
		err = m.state.Destroy(ctx, omni.NewMachineOperation(machineID).Metadata())
	} else {
		err = safe.StateModify(ctx, m.state, omni.NewMachineOperation(machineID), func(res *omni.MachineOperation) error {
			res.TypedSpec().Value = spec

			return nil
		})
	}

	if err != nil && !state.IsNotFoundError(err) {
		m.logger.Error("failed to write the machine operation", zap.String("machine_id", machineID), zap.Error(err))
	}
}
