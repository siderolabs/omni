// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package redactedmachineconfig

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// ControllerName is the name of Controller.
const ControllerName = "RedactedClusterMachineConfigController"

// Controller manages machine configurations for each ClusterMachine.
//
// Controller generates machine configuration for each created machine.
type Controller struct {
	modifiedAtAnnotationKey string
	options                 ControllerOptions
}

// ControllerOptions contains options for Controller.
type ControllerOptions struct {
	CleanupCh           <-chan struct{}
	DiffCleanupInterval time.Duration
	DiffMaxAge          time.Duration
	DiffMaxCount        int
}

// NewController creates a new instance of Controller with the provided options.
func NewController(options ControllerOptions) *Controller {
	if options.DiffCleanupInterval == 0 {
		options.DiffCleanupInterval = 24 * time.Hour
	}

	if options.DiffMaxAge == 0 {
		options.DiffMaxAge = 30 * 24 * time.Hour
	}

	if options.DiffMaxCount == 0 {
		options.DiffMaxCount = 1000
	}

	return &Controller{
		options:                 options,
		modifiedAtAnnotationKey: omni.SystemLabelPrefix + "modified-at",
	}
}

// Name implements controller.QController interface.
func (ctrl *Controller) Name() string {
	return ControllerName
}

// Settings implements controller.QController interface.
func (ctrl *Controller) Settings() controller.QSettings {
	return controller.QSettings{
		RunHook: func(ctx context.Context, logger *zap.Logger, r controller.QRuntime) error {
			ticker := time.NewTicker(ctrl.options.DiffCleanupInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ctrl.options.CleanupCh:
					logger.Debug("machine config diff cleanup triggered via channel")
				case <-ticker.C:
				}

				if err := ctrl.cleanup(ctx, r, logger); err != nil {
					return err
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
func (ctrl *Controller) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
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
func (ctrl *Controller) MapInput(_ context.Context, _ *zap.Logger, _ controller.QRuntime, pointer controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	switch pointer.Type() {
	case omni.ClusterMachineConfigType:
		return []resource.Pointer{
			omni.NewRedactedClusterMachineConfig(pointer.ID()).Metadata(),
		}, nil
	default:
		return nil, fmt.Errorf("unexpected input type %q", pointer.Type())
	}
}

func (ctrl *Controller) reconcileRunning(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, cmc *omni.ClusterMachineConfig) error {
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

func (ctrl *Controller) reconcileTearingDown(ctx context.Context, r controller.ReaderWriter, cmc *omni.ClusterMachineConfig) error {
	var listFunc func(context.Context, resource.Kind, ...state.ListOption) (resource.List, error)

	uncachedReader, ok := r.(controller.UncachedReader)
	if ok {
		listFunc = uncachedReader.ListUncached
	} else {
		listFunc = r.List
	}

	list, err := listFunc(ctx, omni.NewMachineConfigDiff("").Metadata(), state.WithLabelQuery(resource.LabelEqual(omni.LabelMachine, cmc.Metadata().ID())))
	if err != nil {
		return fmt.Errorf("failed to list diffs for machine config %q: %w", cmc.Metadata().ID(), err)
	}

	pointers := xslices.Map(list.Items, func(res resource.Resource) resource.Pointer {
		return res.Metadata()
	})

	if _, err = helpers.TeardownAndDestroyAll(ctx, r, slices.Values(pointers)); err != nil {
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

// modifiedAtFormat is similar to time.RFC3339Nano but with fixed number of nanoseconds digits - i.e., if there are trailing zeros, they are not trimmed.
//
// This is needed to ensure lexicographical ordering (needed by the cleanup task to remove the correct diffs) works as expected.
const modifiedAtFormat = "2006-01-02T15:04:05.000000000Z07:00"

func (ctrl *Controller) saveDiff(ctx context.Context, r controller.ReaderWriter,
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

	modifiedAt := time.Now().UTC().Format(modifiedAtFormat)

	diffID := cmcr.Metadata().ID() + "-" + modifiedAt
	diffRes := omni.NewMachineConfigDiff(diffID)

	if err := safe.WriterModify(ctx, r, diffRes, func(res *omni.MachineConfigDiff) error {
		res.Metadata().Annotations().Set(ctrl.modifiedAtAnnotationKey, modifiedAt)
		res.Metadata().Labels().Set(omni.LabelMachine, cmcr.Metadata().ID())

		res.TypedSpec().Value.Diff = diffStr

		helpers.CopyAllLabels(cmcr, res)

		return nil
	}); err != nil {
		return fmt.Errorf("failed to write diff: %w", err)
	}

	diffLines := strings.Split(diffStr, "\n")

	logger.Info("saved machine config diff", zap.Strings("diff", diffLines), zap.String("id", diffID))

	return nil
}

func (ctrl *Controller) cleanup(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger) error {
	logger.Info("run cleanup")

	list, err := safe.ReaderListAll[*omni.MachineConfigDiff](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list machine config diffs: %w", err)
	}

	now := time.Now()
	i := list.Len() - 1

	for diff := range list.All() {
		modified := diff.Metadata().Created()

		if modifiedStr, ok := diff.Metadata().Annotations().Get(ctrl.modifiedAtAnnotationKey); ok {
			if modified, err = time.Parse(modifiedAtFormat, modifiedStr); err != nil {
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
