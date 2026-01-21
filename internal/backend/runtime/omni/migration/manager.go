// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

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
	// failWithRequiredMaxVersion will make this migration fail if specified, with the message "please upgrade to at most %s first".
	//
	// Note: the versions above this value might still be able to migrate, but are not guaranteed, since the migration might already be dropped.
	failWithRequiredMaxVersion string
}

// Manager runs COSI state migrations.
type Manager struct {
	state      state.State
	logger     *zap.Logger
	migrations []*migration
}

// NewManager creates new Manager.
func NewManager(state state.State, logger *zap.Logger) *Manager {
	// We choose v1.1.0 as the cutoff point, not v1.0.0, because the embedded etcd version was updated from 3.5 to 3.6 in v1.1.0, which causes the following issue:
	// - User tries to upgrade from an old Omni version, e.g., v0.52.0 to the latest Omni version, e.g., v1.5.0.
	// - The migrations fail due to cutoff/cleanup, telling user to upgrade to v1.0.0 first.
	// - User tries to run v1.0.0, but it fails with etcd downgrade error - etcd is attempted to be downgraded from 3.6 to 3.5.
	v1dot1dot0 := "v1.1.0"

	return &Manager{
		state:  state,
		logger: logger,
		migrations: []*migration{
			// The order of migrations is important.
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "clusterInfo",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "deprecateClusterMachineTemplates",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "clusterMachinesToMachineSets",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "changePublicKeyOwner",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "addDefaultScopesToUsers",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "setRollingStrategyOnControlPlaneMachineSets",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "updateConfigPatchLabels",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "updateMachineFinalizers",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "labelConfigPatches",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "updateMachineStatusClusterRelations",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "updateMachineFinalizersV2",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "labelConfigPatchesV2",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "updateMachineStatusClusterRelationsV2",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "addServiceAccountScopesToUsers",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "clusterInstallImageToTalosVersion",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "migrateLabels",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "dropOldLabels",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "convertScopesToRoles",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "lowercaseAllIdentities",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "removeConfigPatchesFromClusterMachines",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "machineInstallDiskPatches",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "siderolinkCounters",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "fixClusterTalosVersionOwnership",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "updateClusterMachineConfigPatchesLabels",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "clearEmptyConfigPatches",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "cleanupDanglingSchematicConfigurations",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "cleanupExtensionsConfigurationStatuses",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "dropSchematicConfigurationsControllerFinalizer",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "generateAllMaintenanceConfigs",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "setMachineStatusSnapshotOwner",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "migrateInstallImageConfigIntoGenOptions",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "dropGeneratedMaintenanceConfigs",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "deleteMachineSetRequiredMachines",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "deleteMachineClassStatuses",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "removeMaintenanceConfigPatchFinalizers",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "compressMachineConfigsAndPatches",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "compressConfigsAndMachinePatches",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "compressConfigPatches",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "moveEtcdBackupStatuses",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "oldVersionContractFix",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "dropObsoleteConfigPatches",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "markVersionContract",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "dropMachineClassStatusFinalizers",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "createProviders",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "migrateConnectionParamsToController",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "populateJoinTokenUsage",
			},
			{
				failWithRequiredMaxVersion: v1dot1dot0,
				name:                       "populateNodeUniqueTokens",
			},
			// The migrations > v1.1.0 (i.e., >=v1.2.0) below
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
			{
				callback: changeClusterMachineConfigPatchesOwner,
				name:     "changeClusterMachineConfigPatchesOwner",
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

// ErrDropped is returned when a migration is too old and has been dropped.
//
// In this case, the user should upgrade to a version that supports the required max version first.
var ErrDropped = errors.New("migration is dropped (too old)")

// Result represents the result of running migrations.
type Result struct {
	DBVersion    *system.DBVersion
	TotalTime    time.Duration
	StartVersion uint64
}

// Run runs COSI state migrations.
func (m *Manager) Run(ctx context.Context, opt ...Option) (Result, error) {
	opts := Options{}

	for _, o := range opt {
		o(&opts)
	}

	version, err := safe.StateGet[*system.DBVersion](
		ctx,
		m.state,
		system.NewDBVersion(system.DBVersionID).Metadata(),
	)
	if err != nil && !state.IsNotFoundError(err) {
		return Result{}, err
	}

	logger := m.logger.With(zap.Bool("filter_enabled", opts.filter != nil), zap.Bool("fresh_omni", version == nil))

	isFreshInstall := version == nil
	if isFreshInstall {
		version = system.NewDBVersion(system.DBVersionID)

		for _, mig := range m.migrations {
			if mig.failWithRequiredMaxVersion == "" {
				break
			}

			version.TypedSpec().Value.Version++
		}

		if err = m.state.Create(ctx, version); err != nil {
			return Result{}, err
		}
	}

	currentVersion := version.TypedSpec().Value.Version
	opts.maxVersion = cmp.Or(opts.maxVersion, len(m.migrations))

	logger = logger.With(zap.Uint64("start_version", currentVersion), zap.Int("target_version", opts.maxVersion))

	if len(m.migrations) < int(currentVersion) {
		return Result{}, fmt.Errorf("the current version of Omni is too old to run with the current DB version: %d", currentVersion)
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

	var updatedDBVersion *system.DBVersion

	for i, mig := range migrations {
		if opts.filter != nil && !opts.filter(mig.name) {
			logger.Info("skipping migration", zap.String("migration_name", mig.name), zap.Int("version", i))

			continue
		}

		start := time.Now()
		mLogger := logger.With(zap.String("migration_name", mig.name), zap.Int("at_version", i))

		mLogger.Info("running migration")

		if mig.failWithRequiredMaxVersion != "" {
			return Result{}, fmt.Errorf("failed to run migration %q, need to upgrade to at most %q first: %w", mig.name, mig.failWithRequiredMaxVersion, ErrDropped)
		}

		if err = mig.callback(ctx, m.state, mLogger, migrationContext{
			initialDBVersion: currentVersion,
			migrations:       m.migrations,
		}); err != nil {
			return Result{}, fmt.Errorf("migration %s failed: %w", mig.name, err)
		}

		mLogger.Info("migration completed", zap.Duration("took", time.Since(start)))

		if updatedDBVersion, err = safe.StateUpdateWithConflicts(ctx, m.state, version.Metadata(), func(dbVer *system.DBVersion) error {
			dbVer.TypedSpec().Value.Version = currentVersion + uint64(i+1)

			return nil
		}); err != nil {
			return Result{}, err
		}
	}

	totalTime := time.Since(total)
	logger.Info("all migrations completed", zap.Duration("total", totalTime))

	return Result{
		StartVersion: currentVersion,
		TotalTime:    totalTime,
		DBVersion:    updatedDBVersion,
	}, nil
}
