// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/system"
)

type migrationContext struct {
	migrations       []*migration
	initialDBVersion uint64
}

// Callback represents a single migration callback.
type Callback func(ctx context.Context, state state.State, logger *zap.Logger, migrationContext migrationContext) error

type migration struct {
	callback Callback
	name     string
}

// Manager runs COSI state migrations.
type Manager struct {
	state      state.State
	logger     *zap.Logger
	migrations []*migration
}

// NewManager creates new Manager.
func NewManager(state state.State, logger *zap.Logger) *Manager {
	v1_0_0 := "v1.0.0"

	return &Manager{
		state:  state,
		logger: logger,
		migrations: []*migration{
			// The order of migrations is important.
			{
				callback: gapError(v1_0_0),
				name:     "clusterInfo",
			},
			{
				callback: gapError(v1_0_0),
				name:     "deprecateClusterMachineTemplates",
			},
			{
				callback: gapError(v1_0_0),
				name:     "clusterMachinesToMachineSets",
			},
			{
				callback: gapError(v1_0_0),
				name:     "changePublicKeyOwner",
			},
			{
				callback: gapError(v1_0_0),
				name:     "addDefaultScopesToUsers",
			},
			{
				callback: gapError(v1_0_0),
				name:     "setRollingStrategyOnControlPlaneMachineSets",
			},
			{
				callback: gapError(v1_0_0),
				name:     "updateConfigPatchLabels",
			},
			{
				callback: gapError(v1_0_0),
				name:     "updateMachineFinalizers",
			},
			{
				callback: gapError(v1_0_0),
				name:     "labelConfigPatches",
			},
			{
				callback: gapError(v1_0_0),
				name:     "updateMachineStatusClusterRelations",
			},
			{
				callback: gapError(v1_0_0),
				name:     "updateMachineFinalizersV2",
			},
			{
				callback: gapError(v1_0_0),
				name:     "labelConfigPatchesV2",
			},
			{
				callback: gapError(v1_0_0),
				name:     "updateMachineStatusClusterRelationsV2",
			},
			{
				callback: gapError(v1_0_0),
				name:     "addServiceAccountScopesToUsers",
			},
			{
				callback: gapError(v1_0_0),
				name:     "clusterInstallImageToTalosVersion",
			},
			{
				callback: gapError(v1_0_0),
				name:     "migrateLabels",
			},
			{
				callback: gapError(v1_0_0),
				name:     "dropOldLabels",
			},
			{
				callback: gapError(v1_0_0),
				name:     "convertScopesToRoles",
			},
			{
				callback: gapError(v1_0_0),
				name:     "lowercaseAllIdentities",
			},
			{
				callback: gapError(v1_0_0),
				name:     "removeConfigPatchesFromClusterMachines",
			},
			{
				callback: gapError(v1_0_0),
				name:     "machineInstallDiskPatches",
			},
			{
				callback: gapError(v1_0_0),
				name:     "siderolinkCounters",
			},
			{
				callback: gapError(v1_0_0),
				name:     "fixClusterTalosVersionOwnership",
			},
			{
				callback: gapError(v1_0_0),
				name:     "updateClusterMachineConfigPatchesLabels",
			},
			{
				callback: gapError(v1_0_0),
				name:     "clearEmptyConfigPatches",
			},
			{
				callback: gapError(v1_0_0),
				name:     "cleanupDanglingSchematicConfigurations",
			},
			{
				callback: gapError(v1_0_0),
				name:     "cleanupExtensionsConfigurationStatuses",
			},
			{
				callback: gapError(v1_0_0),
				name:     "dropSchematicConfigurationsControllerFinalizer",
			},
			{
				callback: gapError(v1_0_0),
				name:     "generateAllMaintenanceConfigs",
			},
			{
				callback: gapError(v1_0_0),
				name:     "setMachineStatusSnapshotOwner",
			},
			{
				callback: gapError(v1_0_0),
				name:     "migrateInstallImageConfigIntoGenOptions",
			},
			{
				callback: gapError(v1_0_0),
				name:     "dropGeneratedMaintenanceConfigs",
			},
			{
				callback: gapError(v1_0_0),
				name:     "deleteMachineSetRequiredMachines",
			},
			{
				callback: gapError(v1_0_0),
				name:     "deleteMachineClassStatuses",
			},
			{
				callback: gapError(v1_0_0),
				name:     "removeMaintenanceConfigPatchFinalizers",
			},
			{
				callback: gapError(v1_0_0),
				name:     "compressMachineConfigsAndPatches",
			},
			{
				callback: gapError(v1_0_0),
				name:     "compressConfigsAndMachinePatches",
			},
			{
				callback: gapError(v1_0_0),
				name:     "compressConfigPatches",
			},
			{
				callback: gapError(v1_0_0),
				name:     "moveEtcdBackupStatuses",
			},
			{
				callback: gapError(v1_0_0),
				name:     "oldVersionContractFix",
			},
			{
				callback: gapError(v1_0_0),
				name:     "dropObsoleteConfigPatches",
			},
			{
				callback: gapError(v1_0_0),
				name:     "markVersionContract",
			},
			{
				callback: gapError(v1_0_0),
				name:     "dropMachineClassStatusFinalizers",
			},
			{
				callback: gapError(v1_0_0),
				name:     "createProviders",
			},
			{
				callback: gapError(v1_0_0),
				name:     "migrateConnectionParamsToController",
			},
			{
				callback: gapError(v1_0_0),
				name:     "populateJoinTokenUsage",
			},
			{
				callback: gapError(v1_0_0),
				name:     "populateNodeUniqueTokens",
			},
			// The migrations > v1.0 (i.e., >=v1.1.0) below
			{
				callback: moveClusterTaintFromResourceToLabel,
				name:     "moveClusterTaintFromResourceToLabel",
			},
			{
				callback: dropExtraInputFinalizers,
				name:     "dropExtraInputFinalizers",
			},
			{
				callback: moveInfraProviderAnnotationsToLabels,
				name:     "moveInfraProviderAnnotationsToLabels",
			},
			{
				callback: dropSchematicConfigFinalizerFromClusterMachines,
				name:     "dropSchematicConfigFinalizerFromClusterMachines",
			},
			{
				callback: dropTalosUpgradeStatusFinalizersFromSchematicConfigs,
				name:     "dropTalosUpgradeStatusFinalizersFromSchematicConfigs",
			},
			{
				callback: makeMachineSetNodesOwnerEmpty,
				name:     "makeMachineSetNodesOwnerEmpty",
			},
		},
	}
}

// Options represents Manager options.
type Options struct {
	filter     func(string) bool
	maxVersion int
}

// Option represents Manager option.
type Option func(*Options)

// WithFilter allows to filter migrations to run.
func WithFilter(filter func(string) bool) Option {
	return func(o *Options) {
		o.filter = filter
	}
}

// WithMaxVersion allows limiting the maximum migration version.
func WithMaxVersion(version int) Option {
	return func(o *Options) {
		o.maxVersion = version
	}
}

// Run COSI state migrations.
func (m *Manager) Run(ctx context.Context, opt ...Option) error {
	opts := Options{}

	for _, o := range opt {
		o(&opts)
	}

	version, err := safe.StateGet[*system.DBVersion](
		ctx,
		m.state,
		system.NewDBVersion(resources.DefaultNamespace, system.DBVersionID).Metadata(),
	)
	if err != nil {
		if !state.IsNotFoundError(err) {
			return err
		}
	}

	logger := m.logger.With(zap.Bool("filter_enabled", opts.filter != nil), zap.Bool("fresh_omni", version == nil))

	if version == nil {
		version = system.NewDBVersion(resources.DefaultNamespace, system.DBVersionID)

		if err = m.state.Create(ctx, version); err != nil {
			return err
		}
	}

	currentVersion := version.TypedSpec().Value.Version
	opts.maxVersion = cmp.Or(opts.maxVersion, len(m.migrations))

	logger = logger.With(zap.Uint64("start_version", currentVersion), zap.Int("target_version", opts.maxVersion))

	if len(m.migrations) < int(currentVersion) {
		return fmt.Errorf("the current version of Omni is too old to run with the current DB version: %d", currentVersion)
	}

	migrations := m.migrations[currentVersion:opts.maxVersion]
	if len(migrations) > 0 {
		logger.Info("must do those migrations", zap.Int("total", len(migrations)))

		for i, mig := range migrations {
			logger.Info("migration", zap.String("name", mig.name), zap.Int("version", i))
		}
	} else {
		logger.Info("no migrations to run")
	}

	total := time.Now()

	for i, mig := range migrations {
		if opts.filter != nil && !opts.filter(mig.name) {
			logger.Info("skipping migration", zap.String("migration_name", mig.name), zap.Int("version", i))

			continue
		}

		start := time.Now()
		mLogger := logger.With(zap.String("migration_name", mig.name), zap.Int("at_version", i))

		mLogger.Info("running migration")

		if err = mig.callback(ctx, m.state, mLogger, migrationContext{
			initialDBVersion: currentVersion,
			migrations:       m.migrations,
		}); err != nil {
			return fmt.Errorf("migration %s failed: %w", mig.name, err)
		}

		mLogger.Info("migration completed", zap.Duration("took", time.Since(start)))

		if _, err = safe.StateUpdateWithConflicts(ctx, m.state, version.Metadata(), func(dbVer *system.DBVersion) error {
			dbVer.TypedSpec().Value.Version = currentVersion + uint64(i+1)

			return nil
		}); err != nil {
			return err
		}
	}

	logger.Info("all migrations completed", zap.Duration("total", time.Since(total)))

	return nil
}
