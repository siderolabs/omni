// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package helpers_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/qtransform"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
	"github.com/siderolabs/omni/internal/pkg/testsecrets"
)

func TestOutputWritten(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		err     error
		name    string
		written bool
	}{
		{name: "no error", written: true},
		{name: "requeue", err: controller.NewRequeueInterval(time.Second), written: true},
		{name: "requeue with error", err: controller.NewRequeueError(errors.New("boom"), time.Second)},
		{name: "error", err: errors.New("boom")},
		{name: "skipped reconcile", err: xerrors.NewTaggedf[qtransform.SkipReconcileTag]("skip")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.written, helpers.OutputWritten(tt.err))
		})
	}
}

func TestUpdateInputsVersions(t *testing.T) {
	out := omni.NewCluster("test")

	//nolint:prealloc
	in := []resource.Resource{omni.NewMachine("test1"), omni.NewMachine("test2")}

	assert.True(t, helpers.UpdateInputsVersions(out, in...))

	v, _ := out.Metadata().Annotations().Get("inputResourceVersion")
	assert.Equal(t, "a7a451e614fc3b4a7241798235001fea271c7ad5493c392f0a012104379bdb89", v)

	assert.False(t, helpers.UpdateInputsVersions(out, in...))

	in = append(in, omni.NewClusterMachine("cm1"))

	assert.True(t, helpers.UpdateInputsVersions(out, in...))

	v, _ = out.Metadata().Annotations().Get("inputResourceVersion")
	assert.Equal(t, "df4af53c3caf7ae4c0446bcf8b854ed3f5740a47eab0e5151f1962a4a4d52f6f", v)
}

func TestGetTalosClient(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		withCluster bool
		stage       machine.MachineStatusEvent_MachineStage
	}{
		{
			withCluster: true,
			name:        "with cluster",
		},
		{
			name: "insecure",
		},
		{
			withCluster: true,
			stage:       machine.MachineStatusEvent_MAINTENANCE,
			name:        "with cluster and snapshot in maintenance",
		},
		{
			withCluster: true,
			stage:       machine.MachineStatusEvent_INSTALLING,
			name:        "with cluster and snapshot in some different stage",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
			defer cancel()

			st := state.WrapCore(inmem.NewState())

			var clusterMachine *omni.ClusterMachine

			if tt.stage != machine.MachineStatusEvent_UNKNOWN {
				machineStatusSnapshot := omni.NewMachineStatusSnapshot("m1")

				machineStatusSnapshot.TypedSpec().Value.MachineStatus = &machine.MachineStatusEvent{
					Stage: tt.stage,
				}

				require.NoError(t, st.Create(ctx, machineStatusSnapshot))
			}

			if tt.withCluster {
				cluster := omni.NewCluster("test")

				clusterMachine = omni.NewClusterMachine("m1")

				clusterMachine.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

				talosConfig := omni.NewTalosConfig(cluster.Metadata().ID())

				bundle, err := testsecrets.Bundle(config.TalosVersion1_10)
				require.NoError(t, err)

				talosConfig.TypedSpec().Value.Ca = base64.StdEncoding.EncodeToString(bundle.Certs.OS.Crt)

				clientCert, err := secrets.NewAdminCertificateAndKey(time.Now(), bundle.Certs.OS, role.All, time.Hour)
				require.NoError(t, err)

				talosConfig.TypedSpec().Value.Crt = base64.StdEncoding.EncodeToString(clientCert.Crt)
				talosConfig.TypedSpec().Value.Key = base64.StdEncoding.EncodeToString(clientCert.Key)

				require.NoError(t, st.Create(ctx, cluster))
				require.NoError(t, st.Create(ctx, clusterMachine))
				require.NoError(t, st.Create(ctx, talosConfig))
			}

			c, err := helpers.GetTalosClient(ctx, st, "1234", clusterMachine)

			t.Cleanup(func() {
				require.NoError(t, c.Close())
			})

			require.NoError(t, err)
		})
	}
}
