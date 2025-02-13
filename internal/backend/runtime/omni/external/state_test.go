// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package external_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xtesting/check"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup/store"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/external"
)

func TestStateList(t *testing.T) {
	uuidStr1 := uuid.New().String()

	happyResult := []pair.Pair[resource.ID, time.Time]{
		pair.MakePair("cluster1-30", time.Unix(30, 0)),
		pair.MakePair("cluster1-15", time.Unix(15, 0)),
	}

	etcdBackupPtr := omni.NewEtcdBackup("", time.Time{}).Metadata()

	tests := map[string]struct {
		kind         resource.Kind
		opts         []state.ListOption
		coreState    state.CoreState
		storeFactory store.Factory

		errCheck check.Check
		result   []pair.Pair[resource.ID, time.Time]
	}{
		"list all backups": {
			kind: etcdBackupPtr,
			opts: []state.ListOption{
				state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, "cluster1")),
			},
			coreState: &coreState{
				m: map[resource.ID]pair.Pair[*omni.ClusterUUID, error]{
					"cluster1": pair.MakePair(makeClusterUUID("cluster1", uuidStr1), error(nil)),
				},
			},
			storeFactory: &backupStore{
				m: map[string]pair.Pair[etcdbackup.InfoIterator, error]{
					uuidStr1: pair.MakePair(makeIter(happyResult[0], happyResult[1]), error(nil)),
				},
			},
			errCheck: check.NoError(),
			result:   happyResult,
		},
		"no query": {
			kind:     etcdBackupPtr,
			errCheck: check.EqualError("failed to validate: cluster ID must be specified in query"),
		},
		"no cluster ID in query": {
			kind: etcdBackupPtr,
			opts: []state.ListOption{
				state.WithIDQuery(resource.IDRegexpMatch(regexp.MustCompile(".*"))),
			},
			errCheck: check.EqualError("failed to validate: ID query is not supported"),
		},
		"unsupported label query": {
			kind: etcdBackupPtr,
			opts: []state.ListOption{
				state.WithLabelQuery(resource.LabelExists("foo")),
			},
			errCheck: check.ErrorContains("failed to validate: unsupported label query term"),
		},
		"incorrect type": {
			kind: omni.NewEtcdBackupStatus("").Metadata(),
			opts: []state.ListOption{
				state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, "cluster1")),
			},
			errCheck: check.EqualError(`unsupported resource type: unsupported resource kind for list "EtcdBackupStatuses.omni.sidero.dev(default/@undefined)"`),
		},
		"empty cluster id": {
			kind: etcdBackupPtr,
			opts: []state.ListOption{
				state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, "")),
			},
			errCheck: check.ErrorContains(`failed to validate: empty value for "omni.sidero.dev/cluster" is not supported`),
		},
		"cluster uuid not found": {
			kind: etcdBackupPtr,
			opts: []state.ListOption{
				state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, "cluster1")),
			},
			coreState: &coreState{
				m: map[resource.ID]pair.Pair[*omni.ClusterUUID, error]{
					"cluster1": pair.MakePair((*omni.ClusterUUID)(nil), errors.New("cluster not found")),
				},
			},
			errCheck: check.EqualError("failed to get cluster UUID by ID: cluster not found"),
		},
		"list backups returns error": {
			kind: etcdBackupPtr,
			opts: []state.ListOption{
				state.WithLabelQuery(resource.LabelEqual(omni.LabelCluster, "cluster1")),
			},
			coreState: &coreState{
				m: map[resource.ID]pair.Pair[*omni.ClusterUUID, error]{
					"cluster1": pair.MakePair(makeClusterUUID("cluster1", uuidStr1), error(nil)),
				},
			},
			storeFactory: &backupStore{
				m: map[string]pair.Pair[etcdbackup.InfoIterator, error]{
					uuidStr1: pair.MakePair(makeIter(), errors.New("list backups error")),
				},
			},
			errCheck: check.EqualError(`failed to list backups for cluster "cluster1": list backups error`),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &external.State{
				CoreState:    tt.coreState,
				StoreFactory: tt.storeFactory,
				Logger:       zaptest.NewLogger(t),
			}

			list, err := s.List(t.Context(), tt.kind, tt.opts...)
			tt.errCheck(t, err)

			if len(list.Items) != len(tt.result) {
				t.Fatalf("unexpected list length %d, expected %d", len(list.Items), len(tt.result))
			}

			l := safe.NewList[*omni.EtcdBackup](list)
			for i := range len(tt.result) {
				backup := l.Get(i)

				assert.Equal(t, tt.result[i].F1, backup.Metadata().ID())
				assert.Equal(t, tt.result[i].F2.UTC(), backup.TypedSpec().Value.CreatedAt.AsTime().UTC())
			}
		})
	}
}

func TestStateGet(t *testing.T) {
	uuidStr1 := uuid.New().String()

	happyResult := []pair.Pair[resource.ID, time.Time]{
		pair.MakePair("cluster1-30", time.Unix(30, 0)),
		pair.MakePair("cluster1-15", time.Unix(15, 0)),
	}

	tests := map[string]struct { //nolint:govet
		pointer      resource.Pointer
		opts         []state.GetOption
		coreState    state.CoreState
		storeFactory store.Factory

		errCheck check.Check
		result   pair.Pair[resource.ID, time.Time]
	}{
		"get backup": {
			pointer: omni.NewEtcdBackup("cluster1", happyResult[0].F2).Metadata(),
			coreState: &coreState{
				m: map[resource.ID]pair.Pair[*omni.ClusterUUID, error]{
					"cluster1": pair.MakePair(makeClusterUUID("cluster1", uuidStr1), error(nil)),
				},
			},
			storeFactory: &backupStore{
				m: map[string]pair.Pair[etcdbackup.InfoIterator, error]{
					uuidStr1: pair.MakePair(makeIter(happyResult[0], happyResult[1]), error(nil)),
				},
			},
			errCheck: check.NoError(),
			result:   happyResult[0],
		},
		"opts": {
			pointer:  omni.NewEtcdBackup("cluster1", happyResult[0].F2).Metadata(),
			opts:     []state.GetOption{state.WithGetUnmarshalOptions(nil)},
			errCheck: check.EqualError("unsupported resource type: no get options are supported"),
		},
		"incorrect type": {
			pointer:  omni.NewEtcdBackupStatus("").Metadata(),
			errCheck: check.EqualError(`unsupported resource type: unsupported resource type for get "EtcdBackupStatuses.omni.sidero.dev(default/@undefined)"`),
		},
		"not found": {
			pointer: omni.NewEtcdBackup("cluster1", happyResult[0].F2).Metadata(),
			coreState: &coreState{
				m: map[resource.ID]pair.Pair[*omni.ClusterUUID, error]{
					"cluster1": pair.MakePair(makeClusterUUID("cluster1", uuidStr1), error(nil)),
				},
			},
			storeFactory: &backupStore{
				m: map[string]pair.Pair[etcdbackup.InfoIterator, error]{
					uuidStr1: pair.MakePair(makeIter(), error(nil)),
				},
			},
			errCheck: check.EqualError(`resource doesn't exist: etcd backup "cluster1-30" for cluster "cluster1" not found`),
		},
		"not found in result": {
			pointer: omni.NewEtcdBackup("cluster1", happyResult[0].F2).Metadata(),
			coreState: &coreState{
				m: map[resource.ID]pair.Pair[*omni.ClusterUUID, error]{
					"cluster1": pair.MakePair(makeClusterUUID("cluster1", uuidStr1), error(nil)),
				},
			},
			storeFactory: &backupStore{
				m: map[string]pair.Pair[etcdbackup.InfoIterator, error]{
					uuidStr1: pair.MakePair(makeIter(happyResult[1]), error(nil)),
				},
			},
			errCheck: check.EqualError(`resource doesn't exist: etcd backup "cluster1-30" for cluster "cluster1" not found`),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &external.State{
				CoreState:    tt.coreState,
				StoreFactory: tt.storeFactory,
				Logger:       zaptest.NewLogger(t),
			}

			backup, err := s.Get(t.Context(), tt.pointer, tt.opts...)
			tt.errCheck(t, err)

			if err == nil {
				res := backup.(*omni.EtcdBackup) //nolint:forcetypeassert,errcheck

				assert.Equal(t, tt.result.F1, backup.Metadata().ID())
				assert.Equal(t, tt.result.F2.UTC(), res.TypedSpec().Value.CreatedAt.AsTime().UTC())
			}
		})
	}
}

//nolint:unparam
func makeClusterUUID(clusterID string, uuidStr string) *omni.ClusterUUID {
	result := omni.NewClusterUUID(clusterID)
	result.TypedSpec().Value.Uuid = uuidStr

	return result
}

type coreState struct {
	state.CoreState

	m map[resource.ID]pair.Pair[*omni.ClusterUUID, error]
}

func (c *coreState) Get(_ context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	if ptr.Namespace() != resources.DefaultNamespace && ptr.Type() != omni.ClusterUUIDType {
		panic(fmt.Errorf("unexpected namespace %q or type %q", ptr.Namespace(), ptr.Type()))
	}

	if len(opts) > 0 {
		panic(fmt.Errorf("unexpected options %v", opts))
	}

	if res, ok := c.m[ptr.ID()]; ok {
		return res.F1, res.F2
	}

	panic(fmt.Errorf("unexpected ID %q", ptr.ID()))
}

type backupStore struct {
	etcdbackup.Store
	store.Factory

	m map[string]pair.Pair[etcdbackup.InfoIterator, error]
}

func (b *backupStore) ListBackups(_ context.Context, clusterUUID string) (etcdbackup.InfoIterator, error) {
	if res, ok := b.m[clusterUUID]; ok {
		return res.F1, res.F2
	}

	panic(fmt.Errorf("unexpected cluster UUID %q", clusterUUID))
}

func (b *backupStore) GetStore() (etcdbackup.Store, error) {
	return b, nil
}

func makeIter(slc ...pair.Pair[resource.ID, time.Time]) etcdbackup.InfoIterator {
	return func() (etcdbackup.Info, bool, error) {
		if len(slc) == 0 {
			return etcdbackup.Info{}, false, nil
		}

		result := slc[0]
		slc = slc[1:]

		return etcdbackup.Info{
			Timestamp: result.F2,
			Reader:    nil,
			Size:      0,
		}, true, nil
	}
}
