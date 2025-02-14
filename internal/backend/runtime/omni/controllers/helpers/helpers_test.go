// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package helpers_test

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/helpers"
)

func TestUpdateInputsVersions(t *testing.T) {
	out := omni.NewCluster("default", "test")

	in := []resource.Resource{omni.NewMachine("default", "test1"), omni.NewMachine("default", "test2")}

	assert.True(t, helpers.UpdateInputsVersions(out, in...))

	v, _ := out.Metadata().Annotations().Get("inputResourceVersion")
	assert.Equal(t, "a7a451e614fc3b4a7241798235001fea271c7ad5493c392f0a012104379bdb89", v)

	assert.False(t, helpers.UpdateInputsVersions(out, in...))

	in = append(in, omni.NewClusterMachine("default", "cm1"))

	assert.True(t, helpers.UpdateInputsVersions(out, in...))

	v, _ = out.Metadata().Annotations().Get("inputResourceVersion")
	assert.Equal(t, "df4af53c3caf7ae4c0446bcf8b854ed3f5740a47eab0e5151f1962a4a4d52f6f", v)
}

func TestGetTalosClient(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		withCluster bool
	}{
		{
			withCluster: true,
			name:        "with cluster",
		},
		{
			name: "insecure",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
			defer cancel()

			st := state.WrapCore(namespaced.NewState(inmem.Build))

			var clusterMachine *omni.ClusterMachine

			if tt.withCluster {
				cluster := omni.NewCluster(resources.DefaultNamespace, "test")

				clusterMachine = omni.NewClusterMachine(resources.DefaultNamespace, "m1")

				clusterMachine.Metadata().Labels().Set(omni.LabelCluster, cluster.Metadata().ID())

				talosConfig := omni.NewTalosConfig(resources.DefaultNamespace, cluster.Metadata().ID())

				bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersion1_10)
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
