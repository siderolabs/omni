// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package router_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
)

func TestResolveMachineConnection(t *testing.T) {
	t.Parallel()

	machineStatus := func(maintenance bool, cluster string) *omni.MachineStatus {
		s := omni.NewMachineStatus("m1")
		s.TypedSpec().Value.Maintenance = maintenance
		s.TypedSpec().Value.Cluster = cluster

		return s
	}

	for _, tt := range []struct {
		machineStatus    *omni.MachineStatus
		name             string
		requestedCluster string
		expectedCluster  string
		expectedCode     codes.Code // codes.OK means a client is expected, not an error
	}{
		{
			name:             "maintenance, no cluster requested: insecure",
			requestedCluster: "",
			machineStatus:    machineStatus(true, ""),
			expectedCluster:  "",
			expectedCode:     codes.OK,
		},
		{
			name:             "allocated but still in maintenance, no cluster requested: insecure",
			requestedCluster: "",
			machineStatus:    machineStatus(true, "alpha"),
			expectedCluster:  "",
			expectedCode:     codes.OK,
		},
		{
			name:             "allocated but still in maintenance, cluster requested: rejected",
			requestedCluster: "alpha",
			machineStatus:    machineStatus(true, "alpha"),
			expectedCode:     codes.FailedPrecondition,
		},
		{
			name:             "configured, matching cluster requested: secure",
			requestedCluster: "alpha",
			machineStatus:    machineStatus(false, "alpha"),
			expectedCluster:  "alpha",
			expectedCode:     codes.OK,
		},
		{
			name:             "configured, no cluster requested: secure to its own cluster",
			requestedCluster: "",
			machineStatus:    machineStatus(false, "alpha"),
			expectedCluster:  "alpha",
			expectedCode:     codes.OK,
		},
		{
			name:             "configured, wrong cluster requested: rejected",
			requestedCluster: "beta",
			machineStatus:    machineStatus(false, "alpha"),
			expectedCode:     codes.NotFound,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clusterID, err := router.ResolveMachineConnection("m1", tt.requestedCluster, tt.machineStatus)

			if tt.expectedCode == codes.OK {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCluster, clusterID)

				return
			}

			require.Error(t, err)
			assert.Equal(t, tt.expectedCode, status.Code(err))
		})
	}
}
