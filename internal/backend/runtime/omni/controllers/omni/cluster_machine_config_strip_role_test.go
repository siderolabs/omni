// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"net/url"
	"testing"

	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	talosrole "github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

func TestStripTalosAPIAccessOSAdminRole(t *testing.T) {
	t.Run("empty config container", func(t *testing.T) {
		ctr, err := container.New()
		require.NoError(t, err)

		cfg, err := omni.StripTalosAPIAccessOSAdminRole(ctr)
		require.NoError(t, err)

		assert.Empty(t, cfg.Documents())
	})

	t.Run("container without v1alpha1 config", func(t *testing.T) {
		ctr, err := container.New(
			siderolinkCfg(t),
			networkDefaultActionCfg(),
		)
		require.NoError(t, err)

		cfg, err := omni.StripTalosAPIAccessOSAdminRole(ctr)
		require.NoError(t, err)

		assert.Len(t, cfg.Documents(), 2)
		assert.Nil(t, cfg.RawV1Alpha1())
	})

	t.Run("os:admin role", func(t *testing.T) {
		ctr, err := container.New(
			siderolinkCfg(t),
			v1alpha1Cfg(),
			networkDefaultActionCfg(),
		)
		require.NoError(t, err)

		cfg, err := omni.StripTalosAPIAccessOSAdminRole(ctr)
		require.NoError(t, err)

		assert.Len(t, cfg.Documents(), 3)
		assert.Equal(t, cfg.Machine().Features().KubernetesTalosAPIAccess().AllowedRoles(), []string{string(talosrole.EtcdBackup), string(talosrole.Reader)})
	})

	t.Run("nil machine features", func(t *testing.T) {
		v1alpha1Config := v1alpha1Cfg()
		v1alpha1Config.MachineConfig.MachineFeatures = nil

		_, err := container.New(
			siderolinkCfg(t),
			v1alpha1Config,
			networkDefaultActionCfg(),
		)
		require.NoError(t, err)
	})

	t.Run("nil machine", func(t *testing.T) {
		v1alpha1Config := v1alpha1Cfg()
		v1alpha1Config.MachineConfig = nil

		_, err := container.New(
			siderolinkCfg(t),
			v1alpha1Config,
			networkDefaultActionCfg(),
		)
		require.NoError(t, err)
	})
}

func siderolinkCfg(t *testing.T) *siderolink.ConfigV1Alpha1 {
	var err error

	cfg := siderolink.NewConfigV1Alpha1()
	cfg.APIUrlConfig.URL, err = url.Parse("https://siderolink.api/join?token=secret")

	require.NoError(t, err)

	return cfg
}

func v1alpha1Cfg() *v1alpha1.Config {
	return &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineFeatures: &v1alpha1.FeaturesConfig{
				KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{
					AccessAllowedRoles: []string{string(talosrole.EtcdBackup), string(talosrole.Admin), string(talosrole.Reader)},
				},
			},
		},
	}
}

func networkDefaultActionCfg() *network.DefaultActionConfigV1Alpha1 {
	cfg := network.NewDefaultActionConfigV1Alpha1()
	cfg.Ingress = nethelpers.DefaultActionBlock

	return cfg
}
