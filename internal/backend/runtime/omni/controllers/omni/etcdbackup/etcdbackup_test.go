// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package etcdbackup_test

import (
	"testing"
	"time"

	"github.com/siderolabs/gen/xtesting/check"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni/etcdbackup"
)

func TestParseSnapshotName(t *testing.T) {
	date := time.Date(2012, 1, 1, 12, 0, 0, 0, time.UTC).In(time.Local)
	name := etcdbackup.CreateSnapshotName(date)
	require.EqualValues(t, "FFFFFFFFB0FFB540.snapshot", name)

	nextDate := date.Add(time.Second)
	nextName := etcdbackup.CreateSnapshotName(nextDate)
	require.EqualValues(t, "FFFFFFFFB0FFB53F.snapshot", nextName)

	require.Less(t, nextName, name)

	//nolint:govet
	tests := map[string]struct {
		snapshotName string
		timestamp    time.Time
		wantErr      check.Check
	}{
		"no error": {
			snapshotName: name,
			timestamp:    date,
			wantErr:      check.NoError(),
		},
		"empty text": {
			snapshotName: "",
			wantErr:      check.EqualError(`failed to parse snapshot "", invalid suffix`),
		},
		"letters and numbers": {
			snapshotName: "FFFFFFFFB0FGB540.snapshot",
			wantErr:      check.ErrorContains(`: invalid syntax`),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			timestamp, err := etcdbackup.ParseSnapshotName(tt.snapshotName)
			tt.wantErr(t, err)
			require.EqualValues(t, tt.timestamp, timestamp)
		})
	}
}
