// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package check_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/pkg/check"
)

func TestCanScaleDown(t *testing.T) {
	for _, tt := range []struct {
		count      int
		healthy    int
		index      int
		shouldFail bool
	}{
		{
			count:      1,
			healthy:    1,
			index:      0,
			shouldFail: true,
		},
		{
			count:   2,
			healthy: 2,
			index:   1,
		},
		{
			count:      2,
			healthy:    1,
			index:      1,
			shouldFail: true,
		},
		{
			count:      3,
			healthy:    2,
			index:      1,
			shouldFail: true,
		},
		{
			count:   3,
			healthy: 2,
			index:   2,
		},
		{
			count:   5,
			healthy: 4,
			index:   2,
		},
		{
			count:   3,
			healthy: 1,
			index:   3,
		},
		{
			count:   3,
			healthy: 2,
			index:   3,
		},
	} {
		t.Run(fmt.Sprintf("scale down, joined %d, healthy %d, index: %d", tt.count, tt.healthy, tt.index), func(t *testing.T) {
			members := map[string]check.EtcdMemberStatus{}

			for i := range tt.count {
				members[strconv.FormatInt(int64(i), 10)] = check.EtcdMemberStatus{
					Healthy: i < tt.healthy,
				}
			}

			err := check.CanScaleDown(&check.EtcdStatusResult{
				Members:        members,
				HealthyMembers: tt.healthy,
			}, omni.NewClusterMachine(resources.DefaultNamespace, strconv.FormatInt(int64(tt.index), 10)))
			if !tt.shouldFail {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
		})
	}
}
