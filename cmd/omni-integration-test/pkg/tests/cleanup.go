// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// DestroyAllClusterRelatedResources cleans up the state from the previous runs.
func DestroyAllClusterRelatedResources(testCtx context.Context, st state.State) TestFunc {
	return func(t *testing.T) {
		ctx, cancel := context.WithTimeout(testCtx, 300*time.Second)
		defer cancel()

		rtestutils.Teardown[*omni.MachineSet](ctx, t, st, rtestutils.ResourceIDs[*omni.MachineSet](ctx, t, st))
		rtestutils.DestroyAll[*omni.MachineSetNode](ctx, t, st)
		rtestutils.DestroyAll[*omni.ConfigPatch](ctx, t, st)
		rtestutils.DestroyAll[*omni.MachineSet](ctx, t, st)
		rtestutils.DestroyAll[*omni.Cluster](ctx, t, st)
		rtestutils.Destroy[*omni.EtcdBackupS3Conf](ctx, t, st, []resource.ID{omni.EtcdBackupS3ConfID})
	}
}
