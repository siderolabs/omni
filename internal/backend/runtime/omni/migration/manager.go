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

	"github.com/cosi-project/runtime/pkg/resource"
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
	return &Manager{
		state:  state,
		logger: logger,
		migrations: []*migration{
			// The order of migrations is important.
			{
				callback: clusterInfo,
				name:     "clusterInfo",
			},
			{
				callback: deprecateClusterMachineTemplates,
				name:     "deprecateClusterMachineTemplates",
			},
			{
				callback: clusterMachinesToMachineSets,
				name:     "clusterMachinesToMachineSets",
			},
			{
				callback: changePublicKeyOwner,
				name:     "changePublicKeyOwner",
			},
			{
				callback: addDefaultScopesToUsers,
				name:     "addDefaultScopesToUsers",
			},
			{
				callback: setRollingStrategyOnControlPlaneMachineSets,
				name:     "setRollingStrategyOnControlPlaneMachineSets",
			},
			{
				callback: updateConfigPatchLabels,
				name:     "updateConfigPatchLabels",
			},
			{
				callback: updateMachineFinalizers,
				name:     "updateMachineFinalizers",
			},
			{
				callback: labelConfigPatches,
				name:     "labelConfigPatches",
			},
			{
				callback: updateMachineStatusClusterRelations,
				name:     "updateMachineStatusClusterRelations",
			},
			// re-run the following 3 migrations as 'V2' as there was a problem with concurrent Omni instances running
			{
				callback: updateMachineFinalizers,
				name:     "updateMachineFinalizersV2",
			},
			{
				callback: labelConfigPatches,
				name:     "labelConfigPatchesV2",
			},
			{
				callback: updateMachineStatusClusterRelations,
				name:     "updateMachineStatusClusterRelationsV2",
			},
			{
				callback: addServiceAccountScopesToUsers,
				name:     "addServiceAccountScopesToUsers",
			},
			{
				callback: clusterInstallImageToTalosVersion,
				name:     "clusterInstallImageToTalosVersion",
			},
			{
				callback: migrateLabels,
				name:     "migrateLabels",
			},
			{
				callback: dropOldLabels,
				name:     "dropOldLabels",
			},
			{
				callback: convertScopesToRoles,
				name:     "convertScopesToRoles",
			},
			{
				callback: lowercaseAllIdentities,
				name:     "lowercaseAllIdentities",
			},
			{
				callback: removeConfigPatchesFromClusterMachines,
				name:     "removeConfigPatchesFromClusterMachines",
			},
			{
				callback: machineInstallDiskPatches,
				name:     "machineInstallDiskPatches",
			},
			{
				callback: siderolinkCounters,
				name:     "siderolinkCounters",
			},
			{
				callback: fixClusterTalosVersionOwnership,
				name:     "fixClusterTalosVersionOwnership",
			},
			{
				callback: updateClusterMachineConfigPatchesLabels,
				name:     "updateClusterMachineConfigPatchesLabels",
			},
			{
				callback: clearEmptyConfigPatches,
				name:     "clearEmptyConfigPatches",
			},
			{
				callback: cleanupDanglingSchematicConfigurations,
				name:     "cleanupDanglingSchematicConfigurations",
			},
			{
				callback: cleanupExtensionsConfigurationStatuses,
				name:     "cleanupExtensionsConfigurationStatuses",
			},
			{
				callback: dropSchematicConfigurationsControllerFinalizer,
				name:     "dropSchematicConfigurationsControllerFinalizer",
			},
			{
				callback: generateAllMaintenanceConfigs,
				name:     "generateAllMaintenanceConfigs",
			},
			{
				callback: setMachineStatusSnapshotOwner,
				name:     "setMachineStatusSnapshotOwner",
			},
			{
				callback: migrateInstallImageConfigIntoGenOptions,
				name:     "migrateInstallImageConfigIntoGenOptions",
			},
			{
				callback: dropGeneratedMaintenanceConfigs,
				name:     "dropGeneratedMaintenanceConfigs",
			},
			{
				callback: deleteAllResources(resource.NewMetadata(resources.DefaultNamespace, MachineSetRequiredMachinesType, "", resource.VersionUndefined)),
				name:     "deleteMachineSetRequiredMachines",
			},
			{
				callback: deleteAllResources(resource.NewMetadata(resources.DefaultNamespace, MachineClassStatusType, "", resource.VersionUndefined)),
				name:     "deleteMachineClassStatuses",
			},
			{
				callback: removeMaintenanceConfigPatchFinalizers,
				name:     "removeMaintenanceConfigPatchFinalizers",
			},
			{
				callback: noopMigration,
				name:     "compressMachineConfigsAndPatches",
			},
			{
				callback: noopMigration,
				name:     "compressConfigsAndMachinePatches",
			},
			{
				callback: compressConfigPatches,
				name:     "compressConfigPatches",
			},
			{
				callback: moveEtcdBackupStatuses,
				name:     "moveEtcdBackupStatuses",
			},
			{
				callback: noopMigration,
				name:     "oldVersionContractFix",
			},
			{
				callback: dropObsoleteConfigPatches,
				name:     "dropObsoleteConfigPatches",
			},
			{
				callback: markVersionContract,
				name:     "markVersionContract",
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
