// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package siderolink

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore/circularlog"
	"github.com/siderolabs/omni/internal/pkg/siderolink/logstore/sqlitelog"
)

func migrateLogStoreToSQLite(ctx context.Context, circularStoreManager *circularlog.StoreManager, sqliteStoreManager *sqlitelog.StoreManager, omniState state.State, logger *zap.Logger) error {
	machineIDs, err := circularStoreManager.MachineIDs()
	if err != nil {
		return fmt.Errorf("failed to get machine IDs from circular log store manager: %w", err)
	}

	machineList, err := omniState.List(ctx, omni.NewMachine(resources.DefaultNamespace, "").Metadata())
	if err != nil {
		return fmt.Errorf("failed to list machines from state: %w", err)
	}

	liveMachineIDs := make(map[string]struct{}, len(machineList.Items))
	for _, machine := range machineList.Items {
		liveMachineIDs[machine.Metadata().ID()] = struct{}{}
	}

	logger.Info("starting log store migration to sqlite", zap.Int("machine_count", len(machineIDs)))

	for _, id := range machineIDs {
		if _, ok := liveMachineIDs[id]; !ok {
			logger.Info("skip migration for machine as it does not exist in state anymore", zap.String("machine_id", id))
		} else {
			logger.Info("migrate log store to sqlite", zap.String("machine_id", id))

			if err = migrateMachineLogs(ctx, circularStoreManager, sqliteStoreManager, id, logger); err != nil {
				return fmt.Errorf("failed to migrate log store for machine %q: %w", id, err)
			}
		}

		if err = circularStoreManager.Remove(ctx, id); err != nil {
			return fmt.Errorf("failed to remove old circular log store for machine %q: %w", id, err)
		}
	}

	logger.Info("completed log store migration to sqlite")

	return nil
}

func migrateMachineLogs(ctx context.Context, circularStoreManager *circularlog.StoreManager, sqliteStoreManager *sqlitelog.StoreManager, id string, logger *zap.Logger) error {
	hasDataInNewStore, err := sqliteStoreManager.Exists(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check if sqlite log store exists for machine %q: %w", id, err)
	}

	if hasDataInNewStore {
		logger.Info("skip migration for machine as sqlite log store already has data (probably already migrated)", zap.String("machine_id", id))

		return nil
	}

	oldStore, err := circularStoreManager.Create(id)
	if err != nil {
		return fmt.Errorf("failed to create circular log store for machine %q: %w", id, err)
	}

	defer oldStore.Close() //nolint:errcheck

	newStore, err := sqliteStoreManager.Create(id)
	if err != nil {
		return fmt.Errorf("failed to create sqlite log store for machine %q: %w", id, err)
	}

	defer newStore.Close() //nolint:errcheck

	reader, err := oldStore.Reader(ctx, 0, false)
	if err != nil {
		return fmt.Errorf("failed to create reader for circular log store for machine %q: %w", id, err)
	}

	defer reader.Close() //nolint:errcheck

	for {
		line, readErr := reader.ReadLine(ctx)
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}

			return fmt.Errorf("failed to read from reader for machine %q: %w", id, readErr)
		}

		if writeErr := newStore.WriteLine(ctx, line); writeErr != nil {
			return fmt.Errorf("failed to write line to sqlite log store for machine %q: %w", id, writeErr)
		}
	}

	return nil
}
