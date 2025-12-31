// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/jonboulle/clockwork"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// RedactedClusterMachineConfigController manages machine configurations for each ClusterMachine.
//
// RedactedClusterMachineConfigController generates machine configuration for each created machine.
type RedactedClusterMachineConfigController struct {
	modifiedAtAnnotationKey string
	options                 RedactedClusterMachineConfigControllerOptions
}

// RedactedClusterMachineConfigControllerOptions contains options for RedactedClusterMachineConfigController.
type RedactedClusterMachineConfigControllerOptions struct {
	Clock               clockwork.Clock
	DiffCleanupInterval time.Duration
	DiffMaxAge          time.Duration
	DiffMaxCount        int
}

// NewRedactedClusterMachineConfigController creates a new instance of RedactedClusterMachineConfigController with the provided options.
func NewRedactedClusterMachineConfigController(options RedactedClusterMachineConfigControllerOptions) *RedactedClusterMachineConfigController {
	if options.DiffCleanupInterval == 0 {
		options.DiffCleanupInterval = 24 * time.Hour
	}

	if options.DiffMaxAge == 0 {
		options.DiffMaxAge = 30 * 24 * time.Hour
	}

	if options.DiffMaxCount == 0 {
		options.DiffMaxCount = 1000
	}

	if options.Clock == nil {
		options.Clock = clockwork.NewRealClock()
	}

	return &RedactedClusterMachineConfigController{
		options:                 options,
		modifiedAtAnnotationKey: omni.SystemLabelPrefix + "modified-at",
	}
}

// Name implements controller.QController interface.
func (ctrl *RedactedClusterMachineConfigController) Name() string {
	return "RedactedClusterMachineConfigController"
}

// Settings implements controller.QController interface.
func (ctrl *RedactedClusterMachineConfigController) Settings() controller.QSettings {
	return controller.QSettings{
		RunHook: func(ctx context.Context, logger *zap.Logger, r controller.QRuntime) error {
			ticker := ctrl.options.Clock.NewTicker(ctrl.options.DiffCleanupInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.Chan():
					if err := ctrl.cleanup(ctx, r, logger); err != nil {
						return err
					}
				}
			}
		},
		Inputs: []controller.Input{
			{
				Namespace: resources.DefaultNamespace,
				Type:      omni.ClusterMachineConfigType,
				Kind:      controller.InputQPrimary,
			},
		},
		Outputs: []controller.Output{
			{
				Type: omni.RedactedClusterMachineConfigType,
				Kind: controller.OutputExclusive,
			},
			{
				Type: omni.MachineConfigDiffType,
				Kind: controller.OutputExclusive,
			},
		},
	}
}

// Reconcile implements controller.QController interface.
func (ctrl *RedactedClusterMachineConfigController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	cmc, err := safe.ReaderGet[*omni.ClusterMachineConfig](ctx, r, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if cmc.Metadata().Phase() == resource.PhaseTearingDown {
		return ctrl.reconcileTearingDown(ctx, r, cmc)
	}

	return ctrl.reconcileRunning(ctx, r, logger, cmc)
}

// MapInput implements controller.QController interface.
func (ctrl *RedactedClusterMachineConfigController) MapInput(_ context.Context, _ *zap.Logger, _ controller.QRuntime, pointer controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	switch pointer.Type() {
	case omni.ClusterMachineConfigType:
		return []resource.Pointer{
			omni.NewRedactedClusterMachineConfig(pointer.ID()).Metadata(),
		}, nil
	default:
		return nil, fmt.Errorf("unexpected input type %q", pointer.Type())
	}
}

func (ctrl *RedactedClusterMachineConfigController) reconcileRunning(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, cmc *omni.ClusterMachineConfig) error {
	if !cmc.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, cmc.Metadata(), ctrl.Name()); err != nil {
			return err
		}
	}

	err := safe.WriterModify(ctx, r, omni.NewRedactedClusterMachineConfig(cmc.Metadata().ID()), func(res *omni.RedactedClusterMachineConfig) error {
		if !helpers.UpdateInputsVersions(res, cmc) { // config input hasn't changed, skip the update
			return nil
		}

		buffer, err := cmc.TypedSpec().Value.GetUncompressedData()
		if err != nil {
			return err
		}

		defer buffer.Free()

		data := buffer.Data()

		if data == nil {
			if err = res.TypedSpec().Value.SetUncompressedData(nil); err != nil {
				return err
			}

			return nil
		}

		config, err := configloader.NewFromBytes(data)
		if err != nil {
			return err
		}

		previousConfig, err := res.TypedSpec().Value.GetUncompressedData()
		if err != nil {
			return err
		}

		defer previousConfig.Free()

		redactedData, err := config.RedactSecrets(x509.Redacted).EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		if err != nil {
			return err
		}

		if err = res.TypedSpec().Value.SetUncompressedData(redactedData); err != nil {
			return err
		}

		helpers.CopyAllLabels(cmc, res)

		previousConfigData := previousConfig.Data()
		if len(previousConfigData) > 0 { // compute and store the diff
			if err = ctrl.saveDiff(ctx, r, res, previousConfigData, redactedData, logger); err != nil {
				return fmt.Errorf("failed to save diff: %w", err)
			}
		}

		return nil
	})

	return err
}

func (ctrl *RedactedClusterMachineConfigController) reconcileTearingDown(ctx context.Context, r controller.ReaderWriter, cmc *omni.ClusterMachineConfig) error {
	list, err := safe.ReaderListAll[*omni.MachineConfigDiff](ctx, r, state.WithLabelQuery(resource.LabelEqual(omni.LabelMachine, cmc.Metadata().ID())))
	if err != nil {
		return fmt.Errorf("failed to list diffs for machine config %q: %w", cmc.Metadata().ID(), err)
	}

	if _, err = helpers.TeardownAndDestroyAll(ctx, r, list.Pointers()); err != nil {
		return fmt.Errorf("failed to destroy diff for machine config %q: %w", cmc.Metadata().ID(), err)
	}

	cmcr := omni.NewRedactedClusterMachineConfig(cmc.Metadata().ID()).Metadata()

	if _, err = helpers.TeardownAndDestroy(ctx, r, cmcr); err != nil {
		return err
	}

	if cmc.Metadata().Finalizers().Has(ctrl.Name()) {
		if err = r.RemoveFinalizer(ctx, cmc.Metadata(), ctrl.Name()); err != nil {
			return err
		}
	}

	return err
}

func (ctrl *RedactedClusterMachineConfigController) saveDiff(ctx context.Context, r controller.ReaderWriter,
	cmcr *omni.RedactedClusterMachineConfig, previousData, newData []byte, logger *zap.Logger,
) error {
	oldConfig := string(previousData)
	newConfig := string(newData)

	edits := myers.ComputeEdits("", oldConfig, newConfig)
	diff := gotextdiff.ToUnified("", "", oldConfig, edits)
	diffStr := strings.TrimSpace(fmt.Sprint(diff))
	diffStr = strings.Replace(diffStr, "--- \n+++ \n", "", 1) // trim the URIs, as they do not make sense in this context

	if strings.TrimSpace(diffStr) == "" {
		return nil
	}

	const rfc3339Millis = "2006-01-02T15:04:05.000Z07:00"

	diffID := cmcr.Metadata().ID() + "-" + ctrl.options.Clock.Now().UTC().Format(rfc3339Millis)
	diffRes := omni.NewMachineConfigDiff(diffID)

	if err := safe.WriterModify(ctx, r, diffRes, func(res *omni.MachineConfigDiff) error {
		res.Metadata().Annotations().Set(ctrl.modifiedAtAnnotationKey, ctrl.options.Clock.Now().UTC().Format(time.RFC3339Nano))
		res.Metadata().Labels().Set(omni.LabelMachine, cmcr.Metadata().ID())

		res.TypedSpec().Value.Diff = diffStr

		helpers.CopyAllLabels(cmcr, res)

		return nil
	}); err != nil {
		return fmt.Errorf("failed to write diff: %w", err)
	}

	diffLines := strings.Split(diffStr, "\n")

	logger.Info("saved machine config diff", zap.Strings("diff", diffLines))

	return nil
}

func (ctrl *RedactedClusterMachineConfigController) cleanup(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger) error {
	logger.Info("run cleanup")

	list, err := safe.ReaderListAll[*omni.MachineConfigDiff](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list machine config diffs: %w", err)
	}

	now := ctrl.options.Clock.Now()

	i := list.Len() - 1

	for diff := range list.All() {
		modified := diff.Metadata().Created()

		if modifiedStr, ok := diff.Metadata().Annotations().Get(ctrl.modifiedAtAnnotationKey); ok {
			if modified, err = time.Parse(time.RFC3339Nano, modifiedStr); err != nil {
				return fmt.Errorf("failed to parse modified time for diff %q: %w", diff.Metadata().ID(), err)
			}
		}

		switch {
		case now.Sub(modified) > ctrl.options.DiffMaxAge:
			logger.Debug("clean up old machine config diff", zap.String("diff_id", diff.Metadata().ID()))

			if _, err = helpers.TeardownAndDestroy(ctx, r, diff.Metadata()); err != nil {
				return fmt.Errorf("failed to teardown and destroy diff %q: %w", diff.Metadata().ID(), err)
			}
		case i >= ctrl.options.DiffMaxCount:
			logger.Debug("clean up machine config diff above max count", zap.String("diff_id", diff.Metadata().ID()))

			if _, err = helpers.TeardownAndDestroy(ctx, r, diff.Metadata()); err != nil {
				return fmt.Errorf("failed to teardown and destroy diff %q: %w", diff.Metadata().ID(), err)
			}
		}

		i--
	}

	return nil
}
