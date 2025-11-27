// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni

import (
	"context"
	"fmt"
	"os"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/store"
	"github.com/cosi-project/state-sqlite/pkg/state/impl/sqlite"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnisqlite "github.com/siderolabs/omni/internal/backend/runtime/omni/sqlite"
	"github.com/siderolabs/omni/internal/pkg/config"
)

func newSQLitePersistentState(ctx context.Context, config config.SQLite, logger *zap.Logger) (*PersistentState, error) {
	db, err := omnisqlite.OpenDB(config)
	if err != nil {
		return nil, err
	}

	st, err := sqlite.NewState(ctx, db, store.ProtobufMarshaler{},
		sqlite.WithLogger(logger),
		sqlite.WithTablePrefix("metrics_"),
	)
	if err != nil {
		db.Close() //nolint:errcheck

		return nil, fmt.Errorf("failed to create sqlite state (path %q): %w", config.Path, err)
	}

	return &PersistentState{
		State: st,
		Close: func() error {
			st.Close()

			return db.Close()
		},
	}, nil
}

func migrateBoltDBToSQLite(ctx context.Context, logger *zap.Logger, boltPath string, sqliteState state.CoreState) error {
	if _, err := os.Stat(boltPath); os.IsNotExist(err) {
		logger.Info("no existing BoltDB database found, skipping migration", zap.String("path", boltPath))

		return nil
	}

	// in any case, remove the BoltDB after migration attempt
	defer func() {
		if err := os.Remove(boltPath); err != nil {
			logger.Warn("failed to remove old BoltDB database after migration", zap.String("path", boltPath), zap.Error(err))
		} else {
			logger.Info("removed old BoltDB database after migration", zap.String("path", boltPath))
		}
	}()

	boltDBState, err := newBoltPersistentState(
		boltPath, &bbolt.Options{
			NoSync: true, // we do not need fsync for the secondary storage
		}, false, logger)
	if err != nil {
		// don't fail on migration error, just log it
		logger.Error("failed to open existing BoltDB database for migration", zap.String("path", boltPath), zap.Error(err))

		return nil
	}

	defer boltDBState.Close() //nolint:errcheck

	for _, ns := range []resource.Namespace{resources.MetricsNamespace} {
		for _, typ := range []resource.Type{
			omni.EtcdBackupOverallStatusType,
			omni.EtcdBackupStatusType,
			omni.MachineStatusLinkType,
		} {
			migrated := 0

			items, err := boltDBState.State.List(ctx, resource.NewMetadata(ns, typ, "", resource.VersionUndefined))
			if err != nil {
				logger.Error("failed to list resources for migration",
					zap.String("namespace", ns),
					zap.String("type", typ),
					zap.String("bolt_path", boltPath),
					zap.Error(err),
				)

				continue
			}

			for _, item := range items.Items {
				item.Metadata().SetVersion(resource.VersionUndefined)

				if err = sqliteState.Create(ctx, item, state.WithCreateOwner(item.Metadata().Owner())); err != nil && !state.IsConflictError(err) {
					logger.Error("failed to migrate resource from BoltDB to SQLite",
						zap.String("namespace", ns),
						zap.String("type", typ),
						zap.String("id", item.Metadata().ID()),
						zap.String("bolt_path", boltPath),
						zap.Error(err),
					)

					continue
				}

				migrated++
			}

			logger.Info("migrated resources from BoltDB to SQLite",
				zap.String("namespace", ns),
				zap.String("type", typ),
				zap.Int("count", migrated),
			)
		}
	}

	return nil
}
