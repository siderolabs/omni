// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

// ConfigPatchCleanupController removes orphaned ConfigPatch resources.
type ConfigPatchCleanupController struct {
	CheckInterval   time.Duration
	DeleteOlderThan time.Duration
}

// Name implements controller.Controller interface.
func (ctrl *ConfigPatchCleanupController) Name() string {
	return "ConfigPatchCleanupController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ConfigPatchCleanupController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ConfigPatchType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineSetType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.ClusterMachineType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: resources.DefaultNamespace,
			Type:      omni.MachineType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ConfigPatchCleanupController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: omni.ConfigPatchType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *ConfigPatchCleanupController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctrl.initDefaults()

	logger = logger.With(zap.Duration("check_interval", ctrl.CheckInterval), zap.Duration("delete_older_than", ctrl.DeleteOlderThan))

	ticker := time.NewTicker(ctrl.CheckInterval)
	defer ticker.Stop()

	initial := true

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if !initial {
				continue
			}
		case <-ticker.C:
		}

		if err := ctrl.reconcile(ctx, r, logger); err != nil {
			return fmt.Errorf("reconciliation failed: %w", err)
		}

		initial = false

		r.ResetRestartBackoff()
	}
}

func (ctrl *ConfigPatchCleanupController) reconcile(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger) error {
	allConfigPatches, err := safe.ReaderListAll[*omni.ConfigPatch](ctx, r)
	if err != nil {
		return err
	}

	var errs error

	for configPatch := range allConfigPatches.All() {
		if err = ctrl.processConfigPatch(ctx, r, configPatch, logger); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

// processConfigPatch processes a single the ConfigPatch resource, checking if it is orphaned and cleaning it up if it is.
func (ctrl *ConfigPatchCleanupController) processConfigPatch(ctx context.Context, r controller.ReaderWriter, configPatch *omni.ConfigPatch, logger *zap.Logger) error {
	logger = logger.With(zap.String("id", configPatch.Metadata().ID()))

	isOrphan, err := ctrl.isOrphan(ctx, r, configPatch)
	if err != nil {
		return err
	}

	if !isOrphan {
		return nil
	}

	logger.Info("destroy orphaned config patch", zap.Any("labels", configPatch.Metadata().Labels().Raw()))

	_, err = helpers.TeardownAndDestroy(ctx, r, configPatch.Metadata(), controller.WithOwner(""))

	return err
}

// initDefaults initializes default values for the controller if they are not set.
func (ctrl *ConfigPatchCleanupController) initDefaults() {
	if ctrl.CheckInterval == 0 {
		ctrl.CheckInterval = time.Hour
	}

	if ctrl.DeleteOlderThan == 0 { // 30 days
		ctrl.DeleteOlderThan = time.Hour * 24 * 30
	}
}

// isOrphanByLabel returns true if the resource is orphaned by label and linked resource T.
func isOrphanByLabel[T generic.ResourceWithRD](ctx context.Context, r controller.Reader, label string, configPatch *omni.ConfigPatch) (bool, error) {
	value, ok := configPatch.Metadata().Labels().Get(label)
	if !ok {
		return true, nil
	}

	_, err := safe.ReaderGetByID[T](ctx, r, value)
	if err != nil && state.IsNotFoundError(err) {
		return true, nil
	}

	if err != nil {
		return false, err
	}

	return false, nil
}

// isOrphan checks if the ConfigPatch is orphaned.
func (ctrl *ConfigPatchCleanupController) isOrphan(ctx context.Context, r controller.Reader, configPatch *omni.ConfigPatch) (bool, error) {
	if configPatch.Metadata().Owner() != "" {
		return false, nil
	}

	if time.Since(configPatch.Metadata().Updated()) < ctrl.DeleteOlderThan || time.Since(configPatch.Metadata().Created()) < ctrl.DeleteOlderThan {
		return false, nil
	}

	for _, labelCheck := range []func() (bool, error){
		func() (bool, error) {
			return isOrphanByLabel[*omni.Machine](ctx, r, omni.LabelMachine, configPatch)
		},
		func() (bool, error) {
			return isOrphanByLabel[*omni.ClusterMachine](ctx, r, omni.LabelClusterMachine, configPatch)
		},
		func() (bool, error) {
			return isOrphanByLabel[*omni.MachineSet](ctx, r, omni.LabelMachineSet, configPatch)
		},
		func() (bool, error) {
			return isOrphanByLabel[*omni.Cluster](ctx, r, omni.LabelCluster, configPatch)
		},
	} {
		orphan, err := labelCheck()
		if err != nil {
			return false, err
		}

		if !orphan {
			return false, nil
		}
	}

	// only if orphaned by all labels
	return true, nil
}
